package projectgraph

import (
	"context"
	iofs "io/fs"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/fileindex/store/memstore"
	codefs "github.com/safedep/code/fs"
	"github.com/safedep/code/plugin/indexer"
	"github.com/safedep/code/projectgraph"
	"github.com/safedep/code/projectgraph/reach"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// receiverRow is one form of the receiver of a method call of a type of a
// dependency. want is the answer for the target, as found or
// uncertain/<cause>. A row that the link step does not answer yet names the
// issue that fixes it in gap, and its answer before that fix in today.
type receiverRow struct {
	name   string
	source string
	target string
	want   string
	gap    string
	today  string
	// noDeps leaves the dependency code out of the graph.
	noDeps bool
}

// receiverUnits are the units of dependency code of the receiver rows, by
// language: a directory under testdata/receivers and its unit.
var receiverUnits = map[core.LanguageCode][2]string{
	core.LanguageCodeGo:     {"go/dep", "go:example.com/dep@v1.0.0"},
	core.LanguageCodeRust:   {"rust/dep", "cargo:dep@1.0.0"},
	core.LanguageCodePython: {"python/dep", "pypi:dep@1.0.0"},
}

const (
	goReceiverManifest     = "module app\n\ngo 1.22\n\nrequire example.com/dep v1.0.0\n"
	rustReceiverManifest   = "[package]\nname = \"app\"\nversion = \"0.1.0\"\nedition = \"2021\"\n\n[dependencies]\ndep = \"1.0.0\"\n"
	pythonReceiverManifest = "[project]\nname = \"app\"\nversion = \"0.1.0\"\ndependencies = [\"dep\"]\n"
)

var goReceiverRows = []receiverRow{
	{
		name:   "declared variable",
		source: "func run() {\n\tvar t *dep.Thing = dep.NewThing()\n\tt.Do()\n}",
		target: "example.com//dep//Thing//Do",
		want:   "found",
	},
	{
		name:   "parameter",
		source: "func run(t *dep.Thing) {\n\tt.Do()\n}",
		target: "example.com//dep//Thing//Do",
		want:   "found",
	},
	{
		name:   "struct field",
		source: "type app struct {\n\tt *dep.Thing\n}\n\nfunc run(a *app) {\n\ta.t.Do()\n}",
		target: "example.com//dep//Thing//Do",
		want:   "found",
	},
	{
		name:   "first-party function return",
		source: "func make() *dep.Thing { return nil }\n\nfunc run() {\n\tt := make()\n\tt.Do()\n}",
		target: "example.com//dep//Thing//Do",
		want:   "found",
	},
	{
		name:   "external function return",
		source: "func run() {\n\tt := dep.NewThing()\n\tt.Do()\n}",
		target: "example.com//dep//Thing//Do",
		want:   "found",
	},
	{
		name:   "multi-return",
		source: "func run() {\n\tt, err := dep.NewPair()\n\tif err != nil {\n\t\treturn\n\t}\n\tt.Do()\n}",
		target: "example.com//dep//Thing//Do",
		want:   "found",
	},
	{
		name:   "chained call",
		source: "func run() {\n\tdep.NewThing().Do()\n}",
		target: "example.com//dep//Thing//Do",
		want:   "found",
	},
	{
		name:   "chained method call",
		source: "func run() {\n\tdep.NewThing().Next().Do()\n}",
		target: "example.com//dep//Thing//Do",
		want:   "found",
	},
	{
		name:   "interface return",
		source: "func run() {\n\td := dep.NewDoer()\n\td.Do()\n}",
		target: "example.com//dep//Thing//Do",
		want:   "found",
	},
	{
		name:   "interface return with no implementation",
		source: "func run() {\n\tr := dep.NewRunner()\n\tr.Run()\n}",
		target: "example.com//other//Job//Run",
		want:   "uncertain/interface-receiver",
	},
	{
		name:   "external function return with no dependency code",
		source: "func run() {\n\tt := dep.NewThing()\n\tt.Do()\n}",
		target: "example.com//dep//Thing//Do",
		want:   "uncertain/no-dependency-source",
		noDeps: true,
	},
}

var rustReceiverRows = []receiverRow{
	{
		name:   "declared variable",
		source: "fn run() {\n    let mut t: Thing<u8> = Thing::new();\n    t.insert(1);\n}",
		target: "dep//Thing//insert",
		want:   "found",
	},
	{
		name:   "parameter",
		source: "fn run(mut t: Thing<u8>) {\n    t.insert(1);\n}",
		target: "dep//Thing//insert",
		want:   "found",
	},
	{
		name:   "reference and generic parameter",
		source: "fn run(t: &mut Thing<[u8; 4]>) {\n    t.insert([1; 4]);\n}",
		target: "dep//Thing//insert",
		want:   "found",
	},
	{
		name:   "struct field",
		source: "struct App {\n    t: Thing<u8>,\n}\n\nfn run(a: &mut App) {\n    a.t.insert(1);\n}",
		target: "dep//Thing//insert",
		want:   "found",
	},
	{
		name:   "first-party function return",
		source: "fn make() -> Thing<u8> {\n    Thing::new()\n}\n\nfn run() {\n    let mut t = make();\n    t.insert(1);\n}",
		target: "dep//Thing//insert",
		want:   "found",
	},
	{
		name:   "external function return",
		source: "fn run() {\n    let mut t = dep::new_thing();\n    t.insert(1);\n}",
		target: "dep//Thing//insert",
		want:   "found",
	},
	{
		name:   "multi-return with ?",
		source: "fn run() -> Result<(), String> {\n    let mut t = dep::load_thing()?;\n    t.insert(1);\n    Ok(())\n}",
		target: "dep//Thing//insert",
		want:   "found",
	},
	{
		name:   "option unwrap",
		source: "fn run() {\n    let mut t = dep::try_thing().unwrap();\n    t.insert(1);\n}",
		target: "dep//Thing//insert",
		want:   "found",
	},
	{
		name:   "chained call",
		source: "fn run() {\n    dep::new_thing().insert(1);\n}",
		target: "dep//Thing//insert",
		want:   "found",
	},
	{
		name:   "associated function",
		source: "fn run() {\n    let mut t = Thing::from_vec(vec![1u8]);\n    t.insert(1);\n}",
		target: "dep//Thing//insert",
		want:   "found",
	},
	{
		name:   "associated function with type arguments",
		source: "fn run() {\n    let mut t = Thing::<u8>::new();\n    t.insert(1);\n}",
		target: "dep//Thing//insert",
		want:   "found",
	},
	{
		name:   "clone of a typed value",
		source: "fn run() {\n    let a: Thing<u8> = Thing::new();\n    let mut b = a.clone();\n    b.insert(1);\n}",
		target: "dep//Thing//insert",
		want:   "found",
	},
	{
		name:   "trait object return",
		source: "fn run() {\n    let d = dep::doer();\n    d.run();\n}",
		target: "dep//Thing//run",
		want:   "found",
	},
	{
		name:   "generic bound",
		source: "fn run<T: Doer>(d: &T) {\n    d.run();\n}",
		target: "dep//Thing//run",
		want:   "found",
	},
	{
		name:   "trait object with no implementation",
		source: "fn run(r: &dyn Runner) {\n    r.start();\n}",
		target: "dep//Job//start",
		want:   "uncertain/trait-receiver",
	},
}

// pythonTypeHints is the step of the Python plan that reads the type hints
// of a parameter, a local, a field and a return into the index, and gives
// a definition of a pypi unit its external key. Most Python rows wait on
// it. Two forms of today show the gap: a call on a variable that holds
// the result of an external call gets the false key dep//new_thing//do and
// no record, so the method is no-such-target, and a call on an unknown
// receiver is a record and a name edge, so the method is uncertain.
const pythonTypeHints = "the type-hint step of the Python plan"

// pythonReceiverRows are the forms of a Python receiver. A type hint is
// the only declared type that Python has.
var pythonReceiverRows = []receiverRow{
	{
		name:   "declared variable",
		source: "def run():\n    t: Thing = dep.new_thing()\n    t.do()",
		target: "dep//Thing//do",
		want:   "found",
		gap:    pythonTypeHints,
		today:  "no-such-target",
	},
	{
		name:   "parameter",
		source: "def run(t: Thing):\n    t.do()",
		target: "dep//Thing//do",
		want:   "found",
		gap:    pythonTypeHints,
		today:  "uncertain/unknown-receiver",
	},
	{
		name:   "optional parameter",
		source: "def run(t: Optional[Thing]):\n    t.do()",
		target: "dep//Thing//do",
		want:   "found",
		gap:    pythonTypeHints,
		today:  "uncertain/unknown-receiver",
	},
	{
		name:   "union with none parameter",
		source: "def run(t: Thing | None):\n    t.do()",
		target: "dep//Thing//do",
		want:   "found",
		gap:    pythonTypeHints,
		today:  "uncertain/unknown-receiver",
	},
	{
		name:   "untyped parameter",
		source: "def run(t):\n    t.do()",
		target: "dep//Thing//do",
		want:   "uncertain/unknown-receiver",
	},
	{
		name:   "typed instance attribute",
		source: "class App:\n    def __init__(self):\n        self.t: Thing = dep.new_thing()\n\n    def go(self):\n        self.t.do()\n\n\ndef run():\n    App().go()",
		target: "dep//Thing//do",
		want:   "found",
		gap:    pythonTypeHints,
		today:  "uncertain/no-definition",
	},
	{
		name:   "instance attribute from the constructor",
		source: "class App:\n    def __init__(self):\n        self.t = Thing()\n\n    def go(self):\n        self.t.do()\n\n\ndef run():\n    App().go()",
		target: "dep//Thing//do",
		want:   "found",
		gap:    pythonTypeHints,
		today:  "uncertain/no-definition",
	},
	{
		name:   "dataclass field",
		source: "@dataclass\nclass App:\n    t: Thing\n\n\ndef run(a: App):\n    a.t.do()",
		target: "dep//Thing//do",
		want:   "found",
		gap:    pythonTypeHints,
		today:  "uncertain/unknown-receiver",
	},
	{
		name:   "first-party function return",
		source: "def make() -> Thing:\n    return dep.new_thing()\n\n\ndef run():\n    t = make()\n    t.do()",
		target: "dep//Thing//do",
		want:   "found",
		gap:    pythonTypeHints,
		today:  "uncertain/unknown-receiver",
	},
	{
		name:   "external function return",
		source: "def run():\n    t = dep.new_thing()\n    t.do()",
		target: "dep//Thing//do",
		want:   "found",
		gap:    pythonTypeHints,
		today:  "no-such-target",
	},
	{
		name:   "tuple return",
		source: "def run():\n    t, n = dep.new_pair()\n    t.do()",
		target: "dep//Thing//do",
		want:   "found",
		gap:    pythonTypeHints,
		today:  "uncertain/unknown-receiver",
	},
	{
		name:   "optional return",
		source: "def run():\n    t = dep.try_thing()\n    if t is not None:\n        t.do()",
		target: "dep//Thing//do",
		want:   "found",
		gap:    pythonTypeHints,
		today:  "no-such-target",
	},
	{
		name:   "chained call",
		source: "def run():\n    dep.new_thing().do()",
		target: "dep//Thing//do",
		want:   "found",
		gap:    pythonTypeHints,
		today:  "uncertain/unknown-receiver",
	},
	{
		name:   "chained method call",
		source: "def run():\n    dep.new_thing().next().do()",
		target: "dep//Thing//do",
		want:   "found",
		gap:    pythonTypeHints,
		today:  "uncertain/unknown-receiver",
	},
	{
		name:   "instance from the constructor",
		source: "def run():\n    Thing().do()",
		target: "dep//Thing//do",
		want:   "found",
	},
	{
		name:   "abstract base return",
		source: "def run():\n    d = dep.new_doer()\n    d.do()",
		target: "dep//Thing//do",
		want:   "found",
		gap:    pythonTypeHints,
		today:  "no-such-target",
	},
	{
		name:   "abstract base return with no implementation",
		source: "def run():\n    r = dep.new_runner()\n    r.run()",
		target: "other//Job//run",
		want:   "uncertain/interface-receiver",
		gap:    pythonTypeHints,
		today:  "no-such-target",
	},
	{
		name:   "external function return with no dependency code",
		source: "def run():\n    t = dep.new_thing()\n    t.do()",
		target: "dep//Thing//do",
		want:   "uncertain/no-dependency-source",
		noDeps: true,
		gap:    pythonTypeHints,
		today:  "no-such-target",
	},
}

func TestReceiverForms(t *testing.T) {
	languages := caseLanguages(t)
	for _, tc := range []struct {
		language core.LanguageCode
		rows     []receiverRow
		files    func(source string) map[string]string
	}{
		{core.LanguageCodeGo, goReceiverRows, goReceiverProject},
		{core.LanguageCodeRust, rustReceiverRows, rustReceiverProject},
		{core.LanguageCodePython, pythonReceiverRows, pythonReceiverProject},
	} {
		for _, row := range tc.rows {
			t.Run(string(tc.language)+"/"+row.name, func(t *testing.T) {
				got := receiverAnswer(t, languages, tc.language, tc.files(row.source), row)
				if row.gap != "" {
					t.Logf("known gap, %s fixes it: the answer is %s, want %s", row.gap, got, row.want)
					assert.Equal(t, row.today, got, "the answer before %s changed: update the row", row.gap)
					return
				}
				assert.Equal(t, row.want, got)
			})
		}
	}
}

// goReceiverProject is a Go project whose main.go holds the source.
func goReceiverProject(source string) map[string]string {
	return map[string]string{
		"go.mod":  goReceiverManifest,
		"main.go": "package main\n\nimport \"example.com/dep\"\n\n" + source + "\n\nfunc main() {}\n",
	}
}

// rustReceiverProject is a Rust crate whose src/main.rs holds the source
// and the uses of the dependency.
func rustReceiverProject(source string) map[string]string {
	return map[string]string{
		"Cargo.toml":  rustReceiverManifest,
		"src/main.rs": "use dep::{Doer, Runner, Thing};\n\n" + source + "\n\nfn main() {}\n",
	}
}

// pythonReceiverProject is a Python project whose main.py holds the source
// and the imports of the dependency.
func pythonReceiverProject(source string) map[string]string {
	return map[string]string{
		"pyproject.toml": pythonReceiverManifest,
		"main.py":        "from dataclasses import dataclass\nfrom typing import Optional\n\nimport dep\nfrom dep import Doer, Thing\n\n\n" + source + "\n\n\nif __name__ == \"__main__\":\n    run()\n",
	}
}

// receiverAnswer links a project of the files with the dependency code of
// the language and answers the target of the row.
func receiverAnswer(t *testing.T, languages []core.Language, language core.LanguageCode, files map[string]string, row receiverRow) string {
	dir := t.TempDir()
	for p, text := range files {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, p)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, p), []byte(text), 0o644))
	}
	cfg := codefs.LocalFileSystemConfig{AppDirectories: []string{dir}, ImportUnits: map[string]string{}}
	units := map[string]iofs.FS{}
	if !row.noDeps {
		u := receiverUnits[language]
		depDir, err := filepath.Abs(filepath.Join("testdata", "receivers", filepath.FromSlash(u[0])))
		require.NoError(t, err)
		cfg.ImportDirectories = []string{depDir}
		cfg.ImportUnits[depDir] = u[1]
		units[u[1]] = os.DirFS(depDir)
	}
	fsys, err := codefs.NewLocalFileSystem(cfg)
	require.NoError(t, err)
	indexed := indexProject(t, fsys, indexer.Options{Languages: languages, Store: memstore.New(), Imports: len(units) > 0})
	g := linkProject(t, dir, indexed, projectgraph.Options{Units: units})
	source := receiverSource(t, language)
	results, err := reach.Reach(context.Background(), g, reach.Query{Targets: targetsOf(t, row.target), Sources: []reach.Source{source}, SourceMode: reach.SourcesOnly})
	require.NoError(t, err)
	return outcome(results[0], receiverReasons(t, g, []string{source.Key()}))
}

// receiverSource is the run function of the main file of the language,
// which the search starts from.
func receiverSource(t *testing.T, language core.LanguageCode) reach.Source {
	p := "main.go"
	switch language {
	case core.LanguageCodeRust:
		p = "src/main.rs"
	case core.LanguageCodePython:
		p = "main.py"
	}
	s, err := reach.SourceByKey(fileindex.Join(p, "run"))
	require.NoError(t, err)
	return s
}

// outcome is the answer for one target: found, not-found, no-such-target,
// or uncertain with its cause.
func outcome(r reach.Result, methods map[string]string) string {
	if r.Status != reach.Uncertain {
		return string(r.Status)
	}
	return "uncertain/" + uncertainCause(r, methods)
}

var reasonText = regexp.MustCompile(`\(([a-z-]+)\)$`)

// reasonOf names an uncertainty by its kind, and an unresolved call by its
// reason.
func reasonOf(u reach.Uncertainty) string {
	if m := reasonText.FindStringSubmatch(u.Text); u.Kind == reach.UnresolvedCall && m != nil {
		return m[1]
	}
	return string(u.Kind)
}

// receiverReasons returns the reason of each call site that a search from
// the keys reaches, by the method name of the site. An unknown receiver
// wins over another reason of the name. A result lists at most three
// uncertainties of a kind, so the reasons come from the whole list of an
// exploration.
func receiverReasons(t testing.TB, g *projectgraph.Graph, from []string) map[string]string {
	e, err := reach.Explore(context.Background(), g, from, 0)
	require.NoError(t, err)
	out := map[string]string{}
	for _, u := range e.Uncertainties {
		if u.Kind != reach.UnresolvedCall || u.Method == "" {
			continue
		}
		if reason := reasonOf(u); out[u.Method] == "" || reason == string(fileindex.UnresolvedUnknownReceiver) {
			out[u.Method] = reason
		}
	}
	return out
}

// uncertainCause names the cause of an uncertain answer. A reached call of
// the method of the target whose receiver the link step does not know
// comes first. Else the cause is the first uncertainty of the result that
// the root fallback does not explain.
func uncertainCause(r reach.Result, methods map[string]string) string {
	if reason, ok := methods[fileindex.Last(r.Target.Key())]; ok {
		return reason
	}
	for _, u := range r.Uncertainties {
		if u.Kind != reach.RootFallback {
			return reasonOf(u)
		}
	}
	return string(reach.RootFallback)
}
