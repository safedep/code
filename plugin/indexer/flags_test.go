package indexer

import (
	"slices"
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fileindex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var flagKinds = []fileindex.FlagKind{
	fileindex.FlagEval,
	fileindex.FlagReflection,
	fileindex.FlagComputedImport,
	fileindex.FlagComputedAttribute,
	fileindex.FlagMonkeyPatch,
	fileindex.FlagDecoratorWrap,
	fileindex.FlagParseError,
}

func TestDynamicSites(t *testing.T) {
	cases := []struct {
		file  string
		code  core.LanguageCode
		sites []fileindex.Flag
		// missing names the kinds that no line of the fixture can set. The
		// JavaScript and the TypeScript grammars read a decorator as a
		// name, a dotted name or a call of one, so no decorator of theirs
		// wraps a function in code that the graph cannot name.
		missing []fileindex.FlagKind
	}{
		{"flags.js", core.LanguageCodeJavascript, []fileindex.Flag{
			{Kind: fileindex.FlagComputedImport, Line: 2},
			{Kind: fileindex.FlagComputedImport, Line: 7},
			{Kind: fileindex.FlagComputedImport, Line: 8},
			{Kind: fileindex.FlagEval, Line: 12},
			{Kind: fileindex.FlagEval, Line: 13},
			{Kind: fileindex.FlagReflection, Line: 14},
			{Kind: fileindex.FlagComputedAttribute, Line: 15},
			{Kind: fileindex.FlagMonkeyPatch, Line: 16},
			{Kind: fileindex.FlagParseError, Line: 25},
		}, []fileindex.FlagKind{fileindex.FlagDecoratorWrap}},
		{"flags.ts", core.LanguageCodeTypescript, []fileindex.Flag{
			{Kind: fileindex.FlagComputedImport, Line: 2},
			{Kind: fileindex.FlagComputedImport, Line: 6},
			{Kind: fileindex.FlagEval, Line: 10},
			{Kind: fileindex.FlagEval, Line: 11},
			{Kind: fileindex.FlagReflection, Line: 12},
			{Kind: fileindex.FlagComputedAttribute, Line: 13},
			{Kind: fileindex.FlagMonkeyPatch, Line: 14},
			{Kind: fileindex.FlagMonkeyPatch, Line: 15},
			{Kind: fileindex.FlagParseError, Line: 25},
		}, []fileindex.FlagKind{fileindex.FlagDecoratorWrap}},
		{"flags.py", core.LanguageCodePython, []fileindex.Flag{
			{Kind: fileindex.FlagComputedImport, Line: 6},
			{Kind: fileindex.FlagComputedImport, Line: 8},
			{Kind: fileindex.FlagEval, Line: 13},
			{Kind: fileindex.FlagEval, Line: 14},
			{Kind: fileindex.FlagReflection, Line: 15},
			{Kind: fileindex.FlagComputedAttribute, Line: 17},
			{Kind: fileindex.FlagMonkeyPatch, Line: 18},
			{Kind: fileindex.FlagDecoratorWrap, Line: 31},
			{Kind: fileindex.FlagModuleGetattr, Line: 37},
			{Kind: fileindex.FlagParseError, Line: 41},
		}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			ix := indexFiles(t, "testdata", []string{tc.file}, tc.code)[tc.file]
			require.NotNil(t, ix)
			assert.Equal(t, tc.sites, ix.DynamicSites())

			kinds := map[fileindex.FlagKind]bool{}
			for _, flag := range ix.DynamicSites() {
				kinds[flag.Kind] = true
			}
			for _, kind := range flagKinds {
				if slices.Contains(tc.missing, kind) {
					continue
				}
				assert.True(t, kinds[kind], "no fixture line sets %s", kind)
			}
		})
	}
}

func TestFlagLandsOnTheEnclosingDefinition(t *testing.T) {
	ix := indexFiles(t, "testdata", []string{"flags.py"}, core.LanguageCodePython)["flags.py"]
	require.NotNil(t, ix)

	flags := map[string][]fileindex.Flag{}
	for _, def := range ix.Definitions {
		flags[def.Key] = def.Flags
	}
	assert.Contains(t, flags["flags.py//run"], fileindex.Flag{Kind: fileindex.FlagEval, Line: 13})
	assert.Equal(t, []fileindex.Flag{{Kind: fileindex.FlagDecoratorWrap, Line: 31}}, flags["flags.py//picked"])
	assert.Empty(t, flags["flags.py//index"], "a decorator that names its code, as @app.route, is no dynamic feature")
	assert.Empty(t, flags["flags.py//plain"])
	assert.Contains(t, ix.Flags, fileindex.Flag{Kind: fileindex.FlagEval, Line: 13})

	getattr := fileindex.Flag{Kind: fileindex.FlagModuleGetattr, Line: 31}
	assert.Contains(t, flags["flags.py"], getattr, "a module __getattr__ flags the module")
	assert.Empty(t, flags["flags.py//__getattr__"])
	assert.Contains(t, ix.Flags, getattr)
}

func TestDynamicImportKinds(t *testing.T) {
	cases := []struct {
		file    string
		code    core.LanguageCode
		imports []fileindex.Import
	}{
		{"flags.js", core.LanguageCodeJavascript, []fileindex.Import{
			{Line: 1, Module: "fs", Alias: "fs", Kind: fileindex.ImportStatic, Namespace: "fs"},
			{Line: 2, Module: "", Kind: fileindex.ImportDynamicComputed, Require: true},
			{Line: 3, Module: "./config", Alias: "config", Kind: fileindex.ImportStatic, Namespace: ".//config", Require: true},
			{Line: 6, Module: "./lazy", Alias: "lazy", Kind: fileindex.ImportDynamicString, Namespace: ".//lazy", Require: true},
			{Line: 7, Module: "./mods/", Kind: fileindex.ImportDynamicComputed, Require: true},
			{Line: 8, Module: "./locale/", Kind: fileindex.ImportDynamicComputed},
			{Line: 17, Module: "./later", Kind: fileindex.ImportDynamicString, Namespace: ".//later"},
		}},
		{"flags.py", core.LanguageCodePython, []fileindex.Import{
			{Line: 6, Module: "plugins.", Kind: fileindex.ImportDynamicComputed},
			{Line: 7, Module: "yaml", Kind: fileindex.ImportDynamicString, Namespace: "yaml"},
			{Line: 8, Module: "", Kind: fileindex.ImportDynamicComputed},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			ix := indexFiles(t, "testdata", []string{tc.file}, tc.code)[tc.file]
			require.NotNil(t, ix)
			for _, imp := range tc.imports {
				assert.Contains(t, ix.Imports, imp)
			}
		})
	}
}

func TestExports(t *testing.T) {
	ix := indexFiles(t, "testdata", []string{"exports.js"}, core.LanguageCodeJavascript)["exports.js"]
	require.NotNil(t, ix)

	exported := map[string]bool{}
	for _, def := range ix.Definitions {
		exported[def.Key] = def.Exported
	}
	assert.True(t, exported["exports.js//hidden"], "export { hidden } exports the function")
	assert.True(t, exported["exports.js//Service//start"], "a method is exported with its class")

	assert.Contains(t, ix.Exports, fileindex.Export{Name: "default", Target: "exports.js//main", Line: 9})
	assert.Contains(t, ix.Exports, fileindex.Export{Name: "shared", Line: 12})
	assert.Contains(t, ix.Exports, fileindex.Export{Name: "other", Item: "def", Line: 12})
	assert.Contains(t, ix.Exports, fileindex.Export{Name: "load", From: "./parser", Item: "parse", Line: 13})
	assert.Contains(t, ix.Exports, fileindex.Export{Name: "*", From: "./all", Item: "*", Line: 14})
	assert.Contains(t, ix.Exports, fileindex.Export{Name: "ns", From: "./ns", Line: 15})
	assert.Contains(t, ix.Exports, fileindex.Export{Name: "extra", Target: "exports.js//run", Line: 17})
	assert.Contains(t, ix.Exports, fileindex.Export{Name: "helper", Item: "shared", Line: 18})

	py := indexFiles(t, "testdata", []string{"exports.py"}, core.LanguageCodePython)["exports.py"]
	require.NotNil(t, py)
	assert.Equal(t, []fileindex.Export{
		{Name: "CONSTANT", Target: "exports.py//CONSTANT", Line: 21},
		{Name: "Service", Target: "exports.py//Service", Line: 21},
		{Name: "run", Target: "exports.py//run", Line: 21},
		{Name: "shared", Line: 21},
	}, py.Exports)
}

func TestMainGuard(t *testing.T) {
	tests := []struct {
		name   string
		code   core.LanguageCode
		file   string
		source string
		want   bool
	}{
		{name: "python double quotes", code: core.LanguageCodePython, file: "a.py", source: "if __name__ == \"__main__\":\n    run()\n", want: true},
		{name: "python single quotes", code: core.LanguageCodePython, file: "a.py", source: "if __name__ == '__main__':\n    run()\n", want: true},
		{name: "python reversed", code: core.LanguageCodePython, file: "a.py", source: "if '__main__' == __name__:\n    run()\n", want: true},
		{name: "python other condition", code: core.LanguageCodePython, file: "a.py", source: "if __name__ == 'app':\n    run()\n", want: false},
		{name: "ruby $0", code: core.LanguageCodeRuby, file: "a.rb", source: "if __FILE__ == $0\n  run\nend\n", want: true},
		{name: "ruby $PROGRAM_NAME", code: core.LanguageCodeRuby, file: "a.rb", source: "if $PROGRAM_NAME == __FILE__\n  run\nend\n", want: true},
		{name: "ruby other condition", code: core.LanguageCodeRuby, file: "a.rb", source: "if ARGV.empty?\n  run\nend\n", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := indexSource(t, tt.code, tt.file, tt.source)
			assert.Equal(t, tt.want, ix.MainGuard)
		})
	}
}
