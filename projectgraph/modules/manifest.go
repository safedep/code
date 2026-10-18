package modules

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// PackageJSON holds the fields of a package.json that the link step reads.
// Dir is the directory of the file.
type PackageJSON struct {
	Dir        string
	Name       string          `json:"name"`
	Main       string          `json:"main"`
	Module     string          `json:"module"`
	Bin        json.RawMessage `json:"bin"`
	Exports    json.RawMessage `json:"exports"`
	Workspaces json.RawMessage `json:"workspaces"`

	Dependencies         map[string]json.RawMessage `json:"dependencies"`
	DevDependencies      map[string]json.RawMessage `json:"devDependencies"`
	PeerDependencies     map[string]json.RawMessage `json:"peerDependencies"`
	OptionalDependencies map[string]json.RawMessage `json:"optionalDependencies"`
}

// Path returns the project path of the package.json.
func (p *PackageJSON) Path() string { return path.Join(p.Dir, "package.json") }

// DependencyNames returns the names of the packages that the package.json
// depends on, in any dependency field.
func (p *PackageJSON) DependencyNames() []string {
	set := map[string]bool{}
	for _, deps := range []map[string]json.RawMessage{p.Dependencies, p.DevDependencies, p.PeerDependencies, p.OptionalDependencies} {
		for name := range deps {
			set[name] = true
		}
	}
	return sortedKeys(set)
}

// BinFiles returns the files of the bin field, a string or a map from a
// command to a file, in the order of the commands.
func (p *PackageJSON) BinFiles() []string {
	if len(p.Bin) == 0 {
		return nil
	}
	var one string
	if json.Unmarshal(p.Bin, &one) == nil {
		return []string{one}
	}
	var many map[string]string
	if json.Unmarshal(p.Bin, &many) != nil {
		return nil
	}
	return sortedValues(many)
}

// ExportFiles returns every file that the exports field names, under any
// condition, sorted.
func (p *PackageJSON) ExportFiles() []string {
	var v any
	if len(p.Exports) == 0 || json.Unmarshal(p.Exports, &v) != nil {
		return nil
	}
	set := map[string]bool{}
	var walk func(v any)
	walk = func(v any) {
		switch t := v.(type) {
		case string:
			if !strings.Contains(t, "*") {
				set[t] = true
			}
		case map[string]any:
			for _, x := range t {
				walk(x)
			}
		case []any:
			for _, x := range t {
				walk(x)
			}
		}
	}
	walk(v)
	return sortedKeys(set)
}

// exportSubpaths returns the subpaths of the exports field with no
// pattern, sorted. An exports field with no subpath keys, as a string or a
// map of conditions, exports ".".
func (p *PackageJSON) exportSubpaths() []string {
	v, err := decodeOrdered(p.Exports)
	m, ok := v.(*ordered)
	if err != nil || !ok || !hasSubpathKeys(m) {
		return []string{"."}
	}
	set := map[string]bool{}
	for _, k := range m.keys {
		if strings.HasPrefix(k, ".") && !strings.Contains(k, "*") {
			set[k] = true
		}
	}
	return sortedKeys(set)
}

// workspaceGlobs returns the workspaces field, an array or an object with a
// packages array.
func (p *PackageJSON) workspaceGlobs() []string {
	if len(p.Workspaces) == 0 {
		return nil
	}
	var globs []string
	if json.Unmarshal(p.Workspaces, &globs) == nil {
		return globs
	}
	var obj struct {
		Packages []string `json:"packages"`
	}
	if json.Unmarshal(p.Workspaces, &obj) == nil {
		return obj.Packages
	}
	return nil
}

func parsePackageJSON(dir string, data []byte) (*PackageJSON, error) {
	pj := &PackageJSON{Dir: dir}
	if err := json.Unmarshal(data, pj); err != nil {
		return nil, err
	}
	return pj, nil
}

func parsePNPMWorkspace(data []byte) ([]string, error) {
	var ws struct {
		Packages []string `yaml:"packages"`
	}
	if err := yaml.Unmarshal(data, &ws); err != nil {
		return nil, err
	}
	return ws.Packages, nil
}

// Form is how a file loads a module. The exports map of a package.json
// gives one target per condition, and the form picks the conditions.
type Form string

const (
	// FormImport is an import statement or an import().
	FormImport Form = "import"
	// FormRequire is a CommonJS require.
	FormRequire Form = "require"
)

// active reports whether the form takes an export condition: node, the
// condition of the form, or default. The link step reads no browser build.
func (f Form) active(condition string) bool {
	return condition == "node" || condition == "default" || condition == string(cmp.Or(f, FormImport))
}

// anyCondition marks the targets of an exports map with no condition of
// the form.
const anyCondition = "any"

// otherRuntimes are the export conditions that name no code of Node. A
// map with no condition of the form gives every target but these.
var otherRuntimes = map[string]bool{"browser": true, "types": true}

// ordered is a JSON object with its keys in source order. Node reads the
// conditions of an exports map in that order, and the first condition
// that the form takes wins.
type ordered struct {
	keys   []string
	values map[string]any
}

// decodeOrdered reads a JSON value as decodeValue does, with each object
// as an ordered.
func decodeOrdered(raw json.RawMessage) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	v, err := decodeValue(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("more than one JSON value")
	}
	return v, nil
}

func decodeValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return tok, nil
	}
	switch delim {
	case '{':
		o := &ordered{values: map[string]any{}}
		for dec.More() {
			key, err := dec.Token()
			if err != nil {
				return nil, err
			}
			v, err := decodeValue(dec)
			if err != nil {
				return nil, err
			}
			name, _ := key.(string)
			if _, dup := o.values[name]; !dup {
				o.keys = append(o.keys, name)
			}
			o.values[name] = v
		}
		_, err := dec.Token()
		return o, err
	case '[':
		var list []any
		for dec.More() {
			v, err := decodeValue(dec)
			if err != nil {
				return nil, err
			}
			list = append(list, v)
		}
		_, err := dec.Token()
		return list, err
	}
	return nil, fmt.Errorf("unexpected %v", delim)
}

// exportTargets returns the files that the exports field gives for a
// subpath, as ".", "./utils", and the condition that picked them. A plain
// string has the condition "". A map with no condition of the form gives
// every target of the map but browser and types, with the condition any,
// so an import of a package with other conditions only misses no file.
func exportTargets(raw json.RawMessage, subpath string, form Form) ([]string, string, bool) {
	v, err := decodeOrdered(raw)
	if err != nil {
		return nil, "", false
	}
	m, ok := v.(*ordered)
	if !ok || !hasSubpathKeys(m) {
		if subpath != "." {
			return nil, "", false
		}
		return pickTargets(v, form)
	}
	if value, ok := m.values[subpath]; ok {
		return pickTargets(value, form)
	}
	best, star, ok := matchStar(m.keys, subpath)
	if !ok {
		return nil, "", false
	}
	targets, cond, ok := pickTargets(m.values[best], form)
	for i, target := range targets {
		targets[i] = strings.ReplaceAll(target, "*", star)
	}
	return targets, cond, ok
}

func hasSubpathKeys(m *ordered) bool {
	for _, k := range m.keys {
		if strings.HasPrefix(k, ".") {
			return true
		}
	}
	return false
}

// pickTargets returns the targets of the value of a subpath for the form:
// the target of the first condition that the form takes, as Node reads
// the map, or else every target of the map but browser and types.
func pickTargets(v any, form Form) ([]string, string, bool) {
	if target, cond, ok := pickCondition(v, form); ok {
		return []string{target}, cond, true
	}
	if all := allTargets(v); len(all) > 0 {
		return all, anyCondition, true
	}
	return nil, "", false
}

func pickCondition(v any, form Form) (string, string, bool) {
	switch t := v.(type) {
	case string:
		return t, "", true
	case []any:
		for _, x := range t {
			if target, cond, ok := pickCondition(x, form); ok {
				return target, cond, true
			}
		}
	case *ordered:
		for _, c := range t.keys {
			if !form.active(c) {
				continue
			}
			if target, _, ok := pickCondition(t.values[c], form); ok {
				return target, c, true
			}
		}
	}
	return "", "", false
}

// allTargets returns every target under a value, but the targets of
// browser and types, in the order of the map.
func allTargets(v any) []string {
	var all []string
	switch t := v.(type) {
	case string:
		all = append(all, t)
	case []any:
		for _, x := range t {
			all = append(all, allTargets(x)...)
		}
	case *ordered:
		for _, c := range t.keys {
			if !otherRuntimes[c] {
				all = append(all, allTargets(t.values[c])...)
			}
		}
	}
	return slices.Compact(all)
}

// tsconfig is the effective configuration of one tsconfig.json after its
// extends chain. BaseURL and PathsBase are project paths.
type tsconfig struct {
	hasBaseURL bool
	baseURL    string
	paths      map[string][]string
	pathsBase  string
}

type rawTSConfig struct {
	Extends         json.RawMessage `json:"extends"`
	CompilerOptions struct {
		BaseURL *string             `json:"baseUrl"`
		Paths   map[string][]string `json:"paths"`
	} `json:"compilerOptions"`
}

const maxExtends = 10

// loadTSConfig reads a tsconfig.json and the chain of files it extends. A
// file that extends a package outside the project keeps its own options.
func loadTSConfig(root fs.FS, name string, depth int) (*tsconfig, error) {
	if depth > maxExtends {
		return nil, fmt.Errorf("extends chain longer than %d", maxExtends)
	}
	data, ok, err := readFile(root, name)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%s: %w", name, fs.ErrNotExist)
	}
	var raw rawTSConfig
	if err := json.Unmarshal(StripJSONC(data), &raw); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	cfg := &tsconfig{}
	dir := Dir(name)
	for _, parent := range stringOrList(raw.Extends) {
		file, ok := extendsFile(root, dir, parent)
		if !ok {
			continue
		}
		base, err := loadTSConfig(root, file, depth+1)
		if err != nil {
			return nil, err
		}
		cfg.merge(base)
	}
	if u := raw.CompilerOptions.BaseURL; u != nil {
		if p, ok := Join(dir, *u); ok {
			cfg.hasBaseURL, cfg.baseURL = true, p
		}
	}
	if raw.CompilerOptions.Paths != nil {
		cfg.paths, cfg.pathsBase = raw.CompilerOptions.Paths, dir
	}
	return cfg, nil
}

func (c *tsconfig) merge(base *tsconfig) {
	if base.hasBaseURL {
		c.hasBaseURL, c.baseURL = true, base.baseURL
	}
	if base.paths != nil {
		c.paths, c.pathsBase = base.paths, base.pathsBase
	}
}

// pathsDir returns the directory that the paths values are relative to.
func (c *tsconfig) pathsDir() string {
	if c.hasBaseURL {
		return c.baseURL
	}
	return c.pathsBase
}

// extendsFile returns the project path of a relative extends entry. An
// entry that names a package is outside the project.
func extendsFile(root fs.FS, dir, spec string) (string, bool) {
	if !strings.HasPrefix(spec, ".") {
		return "", false
	}
	p, ok := Join(dir, spec)
	if !ok {
		return "", false
	}
	if path.Ext(p) != ".json" {
		if found, err := exists(root, p+".json"); err == nil && found {
			return p + ".json", true
		}
	}
	return p, true
}

// StripJSONC removes the comments and the trailing commas that
// tsconfig.json allows, so encoding/json reads the rest.
func StripJSONC(data []byte) []byte {
	var out bytes.Buffer
	inString := false
	for i := 0; i < len(data); i++ {
		c := data[i]
		switch {
		case inString:
			out.WriteByte(c)
			if c == '\\' && i+1 < len(data) {
				i++
				out.WriteByte(data[i])
			} else if c == '"' {
				inString = false
			}
		case c == '"':
			inString = true
			out.WriteByte(c)
		case c == '/' && i+1 < len(data) && data[i+1] == '/':
			for i < len(data) && data[i] != '\n' {
				i++
			}
			out.WriteByte('\n')
		case c == '/' && i+1 < len(data) && data[i+1] == '*':
			end := bytes.Index(data[i+2:], []byte("*/"))
			if end < 0 {
				i = len(data)
			} else {
				i += end + 3
			}
			out.WriteByte(' ')
		case c == ',' && closesNext(data[i+1:]):
		default:
			out.WriteByte(c)
		}
	}
	return out.Bytes()
}

// closesNext reports whether the next byte that is not a space or a
// comment closes an object or an array.
func closesNext(rest []byte) bool {
	for i := 0; i < len(rest); i++ {
		switch c := rest[i]; {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
		case c == '/' && i+1 < len(rest) && rest[i+1] == '/':
			for i < len(rest) && rest[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(rest) && rest[i+1] == '*':
			end := bytes.Index(rest[i+2:], []byte("*/"))
			if end < 0 {
				return false
			}
			i += end + 3
		default:
			return c == '}' || c == ']'
		}
	}
	return false
}

// readFile reads a file of the project root. It returns false for a file
// that is not there, and for any file of a nil root.
func readFile(root fs.FS, name string) ([]byte, bool, error) {
	if root == nil {
		return nil, false, nil
	}
	data, err := fs.ReadFile(root, fsName(name))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}
