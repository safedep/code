// Package roots holds the root rules of the project graph: the rule types,
// the loader and the embedded rules file. A rule is data, so a new entry
// point convention is a change to a YAML file and not to Go code.
package roots

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"regexp"
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
	Composer    = "composer.json"
)

var manifestFields = map[string][]string{
	PackageJSON: {"main", "bin", "module", "exports"},
	PyProject:   {"project.scripts", "project.entry-points", "tool.poetry.scripts"},
	SetupPy:     {"console_scripts"},
	Procfile:    nil,
	Dockerfile:  nil,
	Composer:    {"bin"},
}

// Rule marks the roots of one kind. Language is a langcode or AnyLanguage.
// A javascript rule also matches typescript files, and a java rule kotlin
// files. Requires names the packages of the framework of the rule, as
// next. A javascript rule then applies only when a package.json of the
// project depends on one of them, and a python rule when a pyproject.toml,
// a setup.py, a Pipfile or a requirements file does. A project whose
// manifests declare no dependency gives no evidence, so the rule applies.
type Rule struct {
	ID       string   `yaml:"id"`
	Language string   `yaml:"language"`
	Kind     Kind     `yaml:"kind"`
	Requires []string `yaml:"requires,omitempty"`
	Match    Match    `yaml:"match"`
}

// Match holds one match key. Decorator lists the names of the decorators as
// written, as @click.command, and a decorator matches when it is one of
// them, with or without arguments. Annotation matches as Decorator does,
// on the annotations of Java that the index holds as decorators.
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
// class. InPackage names the package clause of the file, as main for Go.
// Static matches a static method of Java.
type DefinitionMatch struct {
	Name      string `yaml:"name"`
	Parent    string `yaml:"parent,omitempty"`
	Static    bool   `yaml:"static,omitempty"`
	InPackage string `yaml:"in_package,omitempty"`
}

// CallMatch matches a call by the symbol key of its callee, as
// django//urls//path, or by a suffix of the key, as //get. ReceiverFrom
// names the keys that the receiver of the call comes from, as express for
// app in app = express(), and a key under one of them matches too. Arg
// picks the argument whose definition is the root. It counts from 0, and -1
// is the last argument. Shape lists the shapes that the argument may have.
// Root receiver makes the first-party class or object of the receiver the
// root, as MyView in MyView.as_view(). A call with no Arg and no Root is a
// decorator, and the root is the definition that it decorates.
type CallMatch struct {
	Callee       []string `yaml:"callee,omitempty"`
	CalleeSuffix []string `yaml:"callee_suffix,omitempty"`
	ReceiverFrom []string `yaml:"receiver_from,omitempty"`
	Arg          *int     `yaml:"arg,omitempty"`
	Shape        Shapes   `yaml:"shape,omitempty"`
	Root         string   `yaml:"root,omitempty"`
}

// RootReceiver is the Root of a call rule that marks the receiver.
const RootReceiver = "receiver"

// Shapes is one argument shape or a list of them.
type Shapes []fileindex.ArgShape

// UnmarshalYAML reads a scalar as a list of one shape.
func (s *Shapes) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		*s = Shapes{fileindex.ArgShape(node.Value)}
		return nil
	}
	var list []fileindex.ArgShape
	if err := node.Decode(&list); err != nil {
		return err
	}
	*s = list
	return nil
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

// unknownField matches the error of the yaml decoder for a key that the
// type does not have.
var unknownField = regexp.MustCompile(`field (\S+) not found in type`)

// decodeRule decodes one rule with no unknown key. A yaml.Node decodes with
// no check of the keys, so the rule goes through its own decoder. An
// unknown key is an error that names the key, so a typo in a user rule
// file does not silently match nothing.
func decodeRule(node *yaml.Node) (Rule, error) {
	data, err := yaml.Marshal(node)
	if err != nil {
		return Rule{}, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var rule Rule
	err = dec.Decode(&rule)
	var typeErr *yaml.TypeError
	if errors.As(err, &typeErr) {
		for _, msg := range typeErr.Errors {
			if m := unknownField.FindStringSubmatch(msg); m != nil {
				return Rule{}, fmt.Errorf("unknown key %q", m[1])
			}
		}
	}
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
	if len(r.Requires) > 0 && r.Language != string(langcode.Javascript) && r.Language != string(langcode.Python) {
		return fmt.Errorf("requires: the language %q has no manifest of dependencies", r.Language)
	}
	if slices.Contains(r.Requires, "") {
		return errors.New("requires: an empty package name")
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
	if c.Arg == nil && len(c.Shape) > 0 {
		return errors.New("call: a shape needs an arg")
	}
	switch {
	case c.Root != "" && c.Root != RootReceiver:
		return fmt.Errorf("call: unknown root %q", c.Root)
	case c.Root != "" && c.Arg != nil:
		return errors.New("call: a root receiver takes no arg")
	}
	for _, shape := range c.Shape {
		switch shape {
		case fileindex.ArgString, fileindex.ArgTemplate, fileindex.ArgName, fileindex.ArgMember,
			fileindex.ArgLiteral, fileindex.ArgComputed, fileindex.ArgFunction:
		default:
			return fmt.Errorf("call: unknown shape %q", shape)
		}
	}
	return nil
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
	case langcode.Java:
		return lang == langcode.Kotlin
	}
	return false
}
