package modules

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strings"
)

// Distribution is one installed Python distribution, as the .dist-info
// directory of its wheel describes it. Dir is that directory, relative to
// the root. TopLevel holds the import names that the distribution installs,
// from top_level.txt, or else from the files of RECORD. Files holds the
// files of RECORD under the root, and Compiled the module paths with no
// Python source, as lxml/etree for a .so or a .pyd file.
type Distribution struct {
	Name     string
	Version  string
	Dir      string
	TopLevel []string
	Files    []string
	Compiled []string
	// Requires holds the names of the distributions that the Requires-Dist
	// lines of METADATA name with no extra marker, in order, each once.
	Requires []string
}

// Unit returns the package unit of the distribution, as pypi:PyYAML@6.0.1.
func (d Distribution) Unit() string { return "pypi:" + d.Name + "@" + d.Version }

// Roots returns the paths under the root that hold the Python code of the
// distribution, as the keys of an import unit take them: a package
// directory with an __init__.py, a module file, or, for a namespace
// package that other distributions share, the packages and the modules
// under it.
func (d Distribution) Roots() []string {
	files := map[string]bool{}
	for _, f := range d.Files {
		if path.Ext(f) == ".py" {
			files[f] = true
		}
	}
	var roots []string
	var walk func(dir string)
	walk = func(dir string) {
		if files[path.Join(dir, "__init__.py")] {
			roots = append(roots, dir)
			return
		}
		seen := map[string]bool{}
		for f := range files {
			rel, ok := strings.CutPrefix(f, dir+"/")
			if dir == "" {
				rel, ok = f, true
			}
			if !ok {
				continue
			}
			first, _, nested := strings.Cut(rel, "/")
			if seen[first] {
				continue
			}
			seen[first] = true
			if nested {
				walk(path.Join(dir, first))
			} else {
				roots = append(roots, path.Join(dir, first))
			}
		}
	}
	for _, top := range d.TopLevel {
		if files[top+".py"] {
			roots = append(roots, top+".py")
		}
		if slices.ContainsFunc(d.Files, func(f string) bool { return strings.HasPrefix(f, top+"/") }) {
			walk(top)
		}
	}
	slices.Sort(roots)
	return slices.Compact(roots)
}

// requiredName returns the distribution name of a Requires-Dist value, as
// requests of "requests (>=2.0); python_version >= '3.8'" or of
// "requests[socks]>=2.0". A requirement with an extra marker, as
// "; extra == 'socks'", is no requirement of a plain install.
func requiredName(req string) (string, bool) {
	spec, marker, _ := strings.Cut(req, ";")
	if strings.Contains(marker, "extra") {
		return "", false
	}
	end := strings.IndexAny(spec, " [(<>=!~;@")
	if end < 0 {
		end = len(spec)
	}
	name := strings.TrimSpace(spec[:end])
	return name, name != ""
}

// PythonTestPath reports whether no program imports a Python file of a
// package, by its path under the root of the package: a file under a tests
// directory, a test file by its name, a conftest.py, a file under a
// __pycache__, and a file under the .dist-info, .egg-info or .data
// directory of a wheel. A test or a testing directory stays, because
// django.test and numpy.testing are modules that programs import.
func PythonTestPath(p string) bool {
	base := path.Base(p)
	if base == "conftest.py" || strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test.py") {
		return true
	}
	return slices.ContainsFunc(strings.Split(path.Dir(p), "/"), func(d string) bool {
		return d == "tests" || d == "__pycache__" || strings.HasSuffix(d, ".dist-info") || strings.HasSuffix(d, ".egg-info") || strings.HasSuffix(d, ".data")
	})
}

var pythonNameRun = regexp.MustCompile(`[-_.]+`)

// NormalizePythonName returns the name of a PyPI package as PEP 503
// compares it: in lower case, with one - for each run of -, _ and ".".
func NormalizePythonName(name string) string {
	return strings.ToLower(pythonNameRun.ReplaceAllString(name, "-"))
}

// compiledExts are the extensions of a Python extension module.
var compiledExts = map[string]bool{".so": true, ".pyd": true, ".dylib": true}

// PythonDistributions reads the .dist-info directories under a root, as a
// site-packages directory or an unpacked wheel holds them, and returns the
// distributions that they describe, in the order of the directories. A
// directory with no METADATA describes no distribution and is left out. It
// returns the files that it could not read.
func PythonDistributions(root fs.FS) ([]Distribution, []Problem) {
	if root == nil {
		return nil, nil
	}
	entries, err := fs.ReadDir(root, ".")
	if err != nil {
		return nil, []Problem{{Path: ".", Text: err.Error()}}
	}
	var out []Distribution
	var ps problems
	for _, e := range entries {
		if !e.IsDir() || !strings.HasSuffix(e.Name(), ".dist-info") {
			continue
		}
		d, err := readDistribution(root, e.Name())
		switch {
		case errors.Is(err, fs.ErrNotExist):
			continue
		case err != nil:
			ps.add(e.Name(), err)
			continue
		}
		out = append(out, d)
	}
	return out, ps
}

var errNoDistributionName = errors.New("METADATA has no Name")

// readDistribution reads the METADATA, the RECORD and the top_level.txt of
// one .dist-info directory.
func readDistribution(root fs.FS, dir string) (Distribution, error) {
	d := Distribution{Dir: dir}
	meta, err := fs.ReadFile(root, path.Join(dir, "METADATA"))
	if err != nil {
		return d, err
	}
	for _, line := range strings.Split(string(meta), "\n") {
		if line == "" {
			break
		}
		if name, ok := strings.CutPrefix(line, "Name: "); ok && d.Name == "" {
			d.Name = strings.TrimSpace(name)
		}
		if version, ok := strings.CutPrefix(line, "Version: "); ok && d.Version == "" {
			d.Version = strings.TrimSpace(version)
		}
		if req, ok := strings.CutPrefix(line, "Requires-Dist: "); ok {
			if name, ok := requiredName(req); ok && !slices.Contains(d.Requires, name) {
				d.Requires = append(d.Requires, name)
			}
		}
	}
	if d.Name == "" {
		return d, errNoDistributionName
	}
	record, err := fs.ReadFile(root, path.Join(dir, "RECORD"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return d, err
	}
	if err == nil {
		if d.Files, d.Compiled, err = readRecord(record); err != nil {
			return d, err
		}
	}
	top, err := fs.ReadFile(root, path.Join(dir, "top_level.txt"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return d, err
	}
	if err == nil {
		scanner := bufio.NewScanner(bytes.NewReader(top))
		for scanner.Scan() {
			if name := strings.TrimSpace(scanner.Text()); name != "" {
				d.TopLevel = append(d.TopLevel, name)
			}
		}
	}
	if len(d.TopLevel) == 0 {
		d.TopLevel = topLevelOf(d.Files, d.Compiled)
	}
	return d, nil
}

// readRecord returns the files of a RECORD under the root, and the module
// paths of its extension modules. A path that leaves the root, as a script
// in ../../bin, is no file of the package.
func readRecord(data []byte) ([]string, []string, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1
	var files, compiled []string
	for {
		row, err := r.Read()
		if errors.Is(err, io.EOF) {
			return files, compiled, nil
		}
		if err != nil {
			return nil, nil, err
		}
		if len(row) == 0 {
			continue
		}
		p, ok := Join(row[0])
		if !ok || p == "" {
			continue
		}
		files = append(files, p)
		if compiledExts[path.Ext(p)] {
			compiled = append(compiled, compiledModule(p))
		}
	}
}

// compiledModule returns the module path of an extension module, with the
// ABI tag and the extension of its file name cut, as lxml/etree for
// lxml/etree.cpython-311-x86_64-linux-gnu.so.
func compiledModule(p string) string {
	dir, base := path.Split(p)
	name, _, _ := strings.Cut(base, ".")
	return dir + name
}

// topLevelOf returns the first path element of each Python module and
// extension module, in order, each once.
func topLevelOf(files, compiled []string) []string {
	var names []string
	for _, f := range files {
		if path.Ext(f) != ".py" {
			continue
		}
		first, _, _ := strings.Cut(f, "/")
		names = append(names, strings.TrimSuffix(first, ".py"))
	}
	for _, m := range compiled {
		first, _, _ := strings.Cut(m, "/")
		names = append(names, first)
	}
	return compactStable(names)
}
