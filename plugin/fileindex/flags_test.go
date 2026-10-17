package fileindex

import (
	"testing"

	"github.com/safedep/code/core"
	fidx "github.com/safedep/code/fileindex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var flagKinds = []fidx.FlagKind{
	fidx.FlagEval,
	fidx.FlagReflection,
	fidx.FlagComputedImport,
	fidx.FlagComputedAttribute,
	fidx.FlagMonkeyPatch,
	fidx.FlagDecoratorWrap,
	fidx.FlagParseError,
}

func TestDynamicSites(t *testing.T) {
	cases := []struct {
		file  string
		code  core.LanguageCode
		sites []fidx.Flag
	}{
		{"flags.js", core.LanguageCodeJavascript, []fidx.Flag{
			{Kind: fidx.FlagComputedImport, Line: 2},
			{Kind: fidx.FlagComputedImport, Line: 7},
			{Kind: fidx.FlagComputedImport, Line: 8},
			{Kind: fidx.FlagEval, Line: 12},
			{Kind: fidx.FlagEval, Line: 13},
			{Kind: fidx.FlagReflection, Line: 14},
			{Kind: fidx.FlagComputedAttribute, Line: 15},
			{Kind: fidx.FlagMonkeyPatch, Line: 16},
			{Kind: fidx.FlagDecoratorWrap, Line: 21},
			{Kind: fidx.FlagParseError, Line: 25},
		}},
		{"flags.ts", core.LanguageCodeTypescript, []fidx.Flag{
			{Kind: fidx.FlagComputedImport, Line: 2},
			{Kind: fidx.FlagComputedImport, Line: 6},
			{Kind: fidx.FlagEval, Line: 10},
			{Kind: fidx.FlagEval, Line: 11},
			{Kind: fidx.FlagReflection, Line: 12},
			{Kind: fidx.FlagComputedAttribute, Line: 13},
			{Kind: fidx.FlagMonkeyPatch, Line: 14},
			{Kind: fidx.FlagMonkeyPatch, Line: 15},
			{Kind: fidx.FlagDecoratorWrap, Line: 19},
			{Kind: fidx.FlagDecoratorWrap, Line: 21},
			{Kind: fidx.FlagParseError, Line: 25},
		}},
		{"flags.py", core.LanguageCodePython, []fidx.Flag{
			{Kind: fidx.FlagComputedImport, Line: 6},
			{Kind: fidx.FlagComputedImport, Line: 8},
			{Kind: fidx.FlagEval, Line: 13},
			{Kind: fidx.FlagEval, Line: 14},
			{Kind: fidx.FlagReflection, Line: 15},
			{Kind: fidx.FlagComputedAttribute, Line: 17},
			{Kind: fidx.FlagMonkeyPatch, Line: 18},
			{Kind: fidx.FlagDecoratorWrap, Line: 21},
			{Kind: fidx.FlagModuleGetattr, Line: 31},
			{Kind: fidx.FlagParseError, Line: 35},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			ix := indexFiles(t, "testdata", []string{tc.file}, tc.code)[tc.file]
			require.NotNil(t, ix)
			assert.Equal(t, tc.sites, ix.DynamicSites())

			kinds := map[fidx.FlagKind]bool{}
			for _, flag := range ix.DynamicSites() {
				kinds[flag.Kind] = true
			}
			for _, kind := range flagKinds {
				assert.True(t, kinds[kind], "no fixture line sets %s", kind)
			}
		})
	}
}

func TestFlagLandsOnTheEnclosingDefinition(t *testing.T) {
	ix := indexFiles(t, "testdata", []string{"flags.py"}, core.LanguageCodePython)["flags.py"]
	require.NotNil(t, ix)

	flags := map[string][]fidx.Flag{}
	for _, def := range ix.Definitions {
		flags[def.Key] = def.Flags
	}
	assert.Contains(t, flags["flags.py//run"], fidx.Flag{Kind: fidx.FlagEval, Line: 13})
	assert.Equal(t, []fidx.Flag{{Kind: fidx.FlagDecoratorWrap, Line: 21}}, flags["flags.py//index"])
	assert.Empty(t, flags["flags.py//plain"])
	assert.Contains(t, ix.Flags, fidx.Flag{Kind: fidx.FlagEval, Line: 13})

	getattr := fidx.Flag{Kind: fidx.FlagModuleGetattr, Line: 31}
	assert.Contains(t, flags["flags.py"], getattr, "a module __getattr__ flags the module")
	assert.Empty(t, flags["flags.py//__getattr__"])
	assert.Contains(t, ix.Flags, getattr)
}

func TestDynamicImportKinds(t *testing.T) {
	cases := []struct {
		file    string
		code    core.LanguageCode
		imports []fidx.Import
	}{
		{"flags.js", core.LanguageCodeJavascript, []fidx.Import{
			{Line: 1, Module: "fs", Alias: "fs", Kind: fidx.ImportStatic, Namespace: "fs"},
			{Line: 2, Module: "", Kind: fidx.ImportDynamicComputed, Require: true},
			{Line: 3, Module: "./config", Alias: "config", Kind: fidx.ImportStatic, Namespace: ".//config", Require: true},
			{Line: 6, Module: "./lazy", Alias: "lazy", Kind: fidx.ImportDynamicString, Namespace: ".//lazy", Require: true},
			{Line: 7, Module: "./mods/", Kind: fidx.ImportDynamicComputed, Require: true},
			{Line: 8, Module: "./locale/", Kind: fidx.ImportDynamicComputed},
			{Line: 17, Module: "./later", Kind: fidx.ImportDynamicString, Namespace: ".//later"},
		}},
		{"flags.py", core.LanguageCodePython, []fidx.Import{
			{Line: 6, Module: "plugins.", Kind: fidx.ImportDynamicComputed},
			{Line: 7, Module: "yaml", Kind: fidx.ImportDynamicString, Namespace: "yaml"},
			{Line: 8, Module: "", Kind: fidx.ImportDynamicComputed},
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

	assert.Contains(t, ix.Exports, fidx.Export{Name: "default", Target: "exports.js//main", Line: 9})
	assert.Contains(t, ix.Exports, fidx.Export{Name: "shared", Line: 12})
	assert.Contains(t, ix.Exports, fidx.Export{Name: "other", Item: "def", Line: 12})
	assert.Contains(t, ix.Exports, fidx.Export{Name: "load", From: "./parser", Item: "parse", Line: 13})
	assert.Contains(t, ix.Exports, fidx.Export{Name: "*", From: "./all", Item: "*", Line: 14})
	assert.Contains(t, ix.Exports, fidx.Export{Name: "ns", From: "./ns", Line: 15})
	assert.Contains(t, ix.Exports, fidx.Export{Name: "extra", Target: "exports.js//run", Line: 17})
	assert.Contains(t, ix.Exports, fidx.Export{Name: "helper", Item: "shared", Line: 18})

	py := indexFiles(t, "testdata", []string{"exports.py"}, core.LanguageCodePython)["exports.py"]
	require.NotNil(t, py)
	assert.Equal(t, []fidx.Export{
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
