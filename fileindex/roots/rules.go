// Package roots holds the root rules of the project graph: the rule types,
// the loader and the embedded rules file. A rule is data, so a new entry
// point convention is a change to a YAML file and not to Go code.
package roots

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"slices"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/internal/pathglob"
	"github.com/safedep/code/langcode"
	"gopkg.in/yaml.v3"
)

// Kind is the kind of a root.
type Kind string

const (
	Main           Kind = "main"
	Manifest       Kind = "manifest"
	Framework      Kind = "framework"
	Test           Kind = "test"
	LibrarySurface Kind = "library-surface"
)

// AnyLanguage makes a rule match the files of every language.
const AnyLanguage = "any"

// Manifest files that a manifest match can name.
const (
	PackageJSON = "package.json"
	PyProject   = "pyproject.toml"
	SetupPy     = "setup.py"
	Procfile    = "Procfile"
	Dockerfile  = "Dockerfile"
)

var manifestFields = map[string][]string{
	PackageJSON: {"main", "bin", "module", "exports"},
	PyProject:   {"project.scripts", "project.entry-points", "tool.poetry.scripts"},
	SetupPy:     {"console_scripts"},
	Procfile:    nil,
	Dockerfile:  nil,
}

// Rule marks the roots of one kind. Language is a langcode or AnyLanguage.
// A javascript rule also matches typescript files.
type Rule struct {
	ID       string `yaml:"id"`
	Language string `yaml:"language"`
	Kind     Kind   `yaml:"kind"`
	Match    Match  `yaml:"match"`
}

// Match holds one match key. Decorator, Annotation, Call and Export parse,
// and they match nothing until the framework root rules of plan step 28
// ship.
type Match struct {
	Definition   *DefinitionMatch `yaml:"definition,omitempty"`
	Decorator    []string         `yaml:"decorator,omitempty"`
	Annotation   []string         `yaml:"annotation,omitempty"`
	Call         *CallMatch       `yaml:"call,omitempty"`
	Export       *ExportMatch     `yaml:"export,omitempty"`
	Path         []string         `yaml:"path,omitempty"`
	Manifest     *ManifestMatch   `yaml:"manifest,omitempty"`
	FileContains string           `yaml:"file_contains,omitempty"`
	TestFile     bool             `yaml:"test_file,omitempty"`
}

// DefinitionMatch matches a definition by name and by the name of its
// class. Static and InPackage match nothing until the index records them in
// plan step 28.
type DefinitionMatch struct {
	Name      string `yaml:"name"`
	Parent    string `yaml:"parent,omitempty"`
	Static    bool   `yaml:"static,omitempty"`
	InPackage string `yaml:"in_package,omitempty"`
}

// CallMatch matches a call by its callee and the shape of one argument. Arg
// counts from 0, and -1 is the last argument.
type CallMatch struct {
	Callee       []string           `yaml:"callee,omitempty"`
	CalleeSuffix []string           `yaml:"callee_suffix,omitempty"`
	ReceiverFrom []string           `yaml:"receiver_from,omitempty"`
	Arg          int                `yaml:"arg,omitempty"`
	Shape        fileindex.ArgShape `yaml:"shape,omitempty"`
}

// ExportMatch matches an exported name in a file of the paths.
type ExportMatch struct {
	Names []string `yaml:"names"`
	Path  []string `yaml:"path,omitempty"`
}

// ManifestMatch matches the files that the fields of a manifest name.
type ManifestMatch struct {
	File   string   `yaml:"file"`
	Fields []string `yaml:"fields,omitempty"`
}

//go:embed rules.yaml
var embedded []byte

var defaults = mustParse(embedded)

// Default returns the embedded rules.
func Default() []Rule {
	return slices.Clone(defaults)
}

func mustParse(data []byte) []Rule {
	rules, err := Parse(data)
	if err != nil {
		panic(fmt.Sprintf("embedded root rules: %v", err))
	}
	return rules
}

// Parse reads a rules file. An error names the rule that it is about.
func Parse(data []byte) ([]Rule, error) {
	var file struct {
		Version int         `yaml:"version"`
		Rules   []yaml.Node `yaml:"rules"`
	}
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("root rules: %w", err)
	}
	if file.Version != 1 {
		return nil, fmt.Errorf("root rules: version %d, want 1", file.Version)
	}

	rules := make([]Rule, 0, len(file.Rules))
	seen := map[string]bool{}
	for i := range file.Rules {
		rule, err := decodeRule(&file.Rules[i])
		if err == nil {
			err = rule.Validate()
		}
		if err == nil && seen[rule.ID] {
			err = errors.New("the id is not unique")
		}
		if err != nil {
			return nil, fmt.Errorf("root rule %s: %w", ruleName(&file.Rules[i], i), err)
		}
		seen[rule.ID] = true
		rules = append(rules, rule)
	}
	return rules, nil
}

// decodeRule decodes one rule with no unknown field. A yaml.Node decodes
// with no check of the fields, so the rule goes through its own decoder.
func decodeRule(node *yaml.Node) (Rule, error) {
	data, err := yaml.Marshal(node)
	if err != nil {
		return Rule{}, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var rule Rule
	err = dec.Decode(&rule)
	return rule, err
}

func ruleName(node *yaml.Node, i int) string {
	for j := 0; j+1 < len(node.Content); j += 2 {
		if node.Content[j].Value == "id" && node.Content[j+1].Value != "" {
			return fmt.Sprintf("%q", node.Content[j+1].Value)
		}
	}
	return fmt.Sprintf("#%d", i+1)
}

// Validate checks the id, the language, the kind and the one match key of
// the rule.
func (r Rule) Validate() error {
	if r.ID == "" {
		return errors.New("no id")
	}
	if r.Language != AnyLanguage && !slices.Contains(langcode.All(), langcode.Code(r.Language)) {
		return fmt.Errorf("unknown language %q", r.Language)
	}
	switch r.Kind {
	case Main, Manifest, Framework, Test:
	default:
		return fmt.Errorf("unknown kind %q", r.Kind)
	}
	return r.Match.validate()
}

func (m Match) validate() error {
	keys := 0
	count := func(set bool) {
		if set {
			keys++
		}
	}
	count(m.Definition != nil)
	count(len(m.Decorator) > 0)
	count(len(m.Annotation) > 0)
	count(m.Call != nil)
	count(m.Export != nil)
	count(len(m.Path) > 0)
	count(m.Manifest != nil)
	count(m.FileContains != "")
	count(m.TestFile)
	if keys != 1 {
		return fmt.Errorf("the match has %d keys, want 1", keys)
	}

	switch {
	case m.Definition != nil && m.Definition.Name == "":
		return errors.New("definition: no name")
	case m.Call != nil:
		return m.Call.validate()
	case m.Export != nil:
		if len(m.Export.Names) == 0 {
			return errors.New("export: no names")
		}
		return validateGlobs(m.Export.Path)
	case m.Manifest != nil:
		return m.Manifest.validate()
	}
	return validateGlobs(m.Path)
}

func (c *CallMatch) validate() error {
	if len(c.Callee) == 0 && len(c.CalleeSuffix) == 0 {
		return errors.New("call: no callee and no callee_suffix")
	}
	switch c.Shape {
	case "", fileindex.ArgString, fileindex.ArgTemplate, fileindex.ArgName, fileindex.ArgMember,
		fileindex.ArgLiteral, fileindex.ArgComputed, fileindex.ArgFunction:
		return nil
	}
	return fmt.Errorf("call: unknown shape %q", c.Shape)
}

func (m *ManifestMatch) validate() error {
	fields, ok := manifestFields[m.File]
	if !ok {
		return fmt.Errorf("manifest: unknown file %q", m.File)
	}
	for _, f := range m.Fields {
		if !slices.Contains(fields, f) {
			return fmt.Errorf("manifest: %s has no field %q", m.File, f)
		}
	}
	return nil
}

func validateGlobs(globs []string) error {
	for _, g := range globs {
		if err := pathglob.Validate(g); err != nil {
			return err
		}
	}
	return nil
}

// Merge returns the base rules with the extra rules over them. An extra rule
// replaces the base rule of the same id at its place, and a new id goes at
// the end.
func Merge(base, extra []Rule) []Rule {
	out := slices.Clone(base)
	at := make(map[string]int, len(out))
	for i, r := range out {
		at[r.ID] = i
	}
	for _, r := range extra {
		if i, ok := at[r.ID]; ok {
			out[i] = r
			continue
		}
		at[r.ID] = len(out)
		out = append(out, r)
	}
	return out
}

// Applies reports whether the rule matches files of the language.
func (r Rule) Applies(lang langcode.Code) bool {
	switch langcode.Code(r.Language) {
	case AnyLanguage, lang:
		return true
	case langcode.Javascript:
		return lang == langcode.Typescript
	}
	return false
}
