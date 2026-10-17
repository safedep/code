package fileindex

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/safedep/code/lang/grammar"
	"github.com/safedep/code/langcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fullIndex() *Index {
	return &Index{
		Version:     Version,
		Path:        "src/app.py",
		Language:    langcode.Python,
		ContentHash: HashContent([]byte("app")),
		Test:        true,
		Imports: []Import{
			{Line: 1, Module: "yaml", Kind: ImportStatic, Namespace: "yaml"},
			{Line: 2, Module: ".utils", Item: "load", Alias: "ld", Kind: ImportStatic, Namespace: "utils//load"},
			{Line: 3, Module: "os.path", Wildcard: true, Kind: ImportStatic, Namespace: "os//path"},
			{Line: 9, Module: "plugins", Kind: ImportDynamicString, Namespace: "plugins"},
			{Line: 10, Kind: ImportDynamicComputed},
		},
		Exports: []Export{
			{Name: "Parser", Target: "src/app.py//Parser", Line: 4},
			{Name: "*", From: ".utils", Item: "*", Line: 12},
		},
		Definitions: []Definition{
			{Key: "src/app.py", Kind: DefModule, Name: "app", StartLine: 1, EndLine: 20, Exported: true},
			{Key: "src/app.py//Parser", Kind: DefClass, Name: "Parser", StartLine: 4, EndLine: 8, Exported: true},
			{Key: "src/app.py//Parser//__init__", Kind: DefConstructor, Name: "__init__", StartLine: 5, EndLine: 5, Exported: true, Parent: "src/app.py//Parser"},
			{
				Key: "src/app.py//Parser//parse", Kind: DefMethod, Name: "parse", StartLine: 6, EndLine: 8, Exported: true,
				Parent: "src/app.py//Parser", Decorators: []string{"@cache", "@app.route"},
				Flags: []Flag{{Kind: FlagEval, Line: 7}},
			},
			{Key: "src/app.py//helper", Kind: DefFunction, Name: "helper", StartLine: 14, EndLine: 16},
		},
		Classes: []Class{
			{Key: "src/app.py//Parser", Bases: []string{"Base", "Mixin"}, Methods: []string{"src/app.py//Parser//__init__", "src/app.py//Parser//parse"}},
		},
		Calls: []Call{
			{Caller: "src/app.py//Parser//parse", Line: 7, Column: 9, Callee: "yaml.load", Targets: []string{"yaml//load"}, Receiver: "yaml", Method: "load", Kind: CallCall, Args: []ArgShape{ArgName, ArgLiteral}},
			{Caller: "src/app.py", Line: 11, Column: 1, Callee: "Parser", Targets: []string{"src/app.py//Parser"}, Kind: CallNew},
			{Caller: "src/app.py", Line: 13, Column: 1, Callee: "helper", Targets: []string{"src/app.py//helper"}, Kind: CallCall, Args: []ArgShape{ArgString, ArgTemplate, ArgMember, ArgComputed, ArgFunction}},
			{Caller: "src/app.py//Parser//parse", Line: 6, Column: 5, Callee: "cache", Targets: []string{"cache"}, Kind: CallDecorator},
			{Caller: "src/app.py", Line: 9, Column: 1, Callee: "import_module", Targets: []string{"plugins"}, Kind: CallImport, Args: []ArgShape{ArgString}},
		},
		Unresolved: []Unresolved{
			{Caller: "src/app.py//helper", Line: 15, Column: 5, Text: "getattr(obj, name)", Reason: UnresolvedComputedName},
			{Caller: "src/app.py", Line: 10, Column: 1, Text: "import_module(name)", Reason: UnresolvedDynamicImport},
			{Caller: "src/app.py//helper", Line: 16, Column: 5, Text: "client.send", Receiver: "client", Method: "send", Reason: UnresolvedUnknownReceiver},
			{Caller: "src/app.py", Line: 17, Column: 1, Text: "missing", Method: "missing", Reason: UnresolvedNoDefinition},
			{Caller: "src/app.py", Line: 18, Column: 1, Text: "a.b", Receiver: "a", Method: "b", Reason: UnresolvedAmbiguous},
			{Caller: "src/app.py", Line: 19, Column: 1, Text: "x.y", Receiver: "x", Method: "y", Reason: UnresolvedLimit},
		},
		Flags: []Flag{
			{Kind: FlagReflection, Line: 15},
			{Kind: FlagComputedImport, Line: 10},
			{Kind: FlagComputedAttribute, Line: 15},
			{Kind: FlagMonkeyPatch, Line: 18},
			{Kind: FlagDecoratorWrap, Line: 6},
			{Kind: FlagParseError, Line: 20},
		},
		Truncated: []Truncation{
			{Kind: TruncationNodes, Namespace: "src/app.py//helper", Line: 15},
			{Kind: TruncationAssignmentNodes, Line: 15},
			{Kind: TruncationDepth, Line: 16},
			{Kind: TruncationMemberDepth, Line: 16},
			{Kind: TruncationAttributeDepth, Line: 16},
			{Kind: TruncationPolymorphic, Line: 17},
			{Kind: TruncationDFSResults},
			{Kind: TruncationDeadline, Line: 19},
		},
	}
}

func TestIndexJSONRoundTripIsByteForByte(t *testing.T) {
	ix := fullIndex()
	ix.Sort()

	golden, err := os.ReadFile("testdata/index.json")
	require.NoError(t, err)

	got, err := json.MarshalIndent(ix, "", "  ")
	require.NoError(t, err)
	assert.Equal(t, string(golden), string(got)+"\n")

	var decoded Index
	require.NoError(t, json.Unmarshal(golden, &decoded))
	assert.Equal(t, ix, &decoded)

	again, err := json.MarshalIndent(&decoded, "", "  ")
	require.NoError(t, err)
	assert.Equal(t, got, again)
}

func TestSortGivesOneOrderForAnyInputOrder(t *testing.T) {
	sorted := fullIndex()
	sorted.Sort()
	want, err := json.Marshal(sorted)
	require.NoError(t, err)

	reversed := fullIndex()
	reverseAll(reversed)
	reversed.Sort()
	got, err := json.Marshal(reversed)
	require.NoError(t, err)

	assert.Equal(t, string(want), string(got))
	assert.Equal(t, []string{"@cache", "@app.route"}, reversed.Definitions[3].Decorators)
	assert.Equal(t, []string{"Base", "Mixin"}, reversed.Classes[0].Bases)
}

func reverseAll(ix *Index) {
	reverse(ix.Imports)
	reverse(ix.Exports)
	reverse(ix.Definitions)
	reverse(ix.Calls)
	reverse(ix.Unresolved)
	reverse(ix.Flags)
	reverse(ix.Truncated)
	for i := range ix.Classes {
		reverse(ix.Classes[i].Methods)
	}
}

func reverse[T any](s []T) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

func TestExternalCalls(t *testing.T) {
	ix := &Index{
		Path: "src/app.py",
		Imports: []Import{
			{Module: "yaml", Kind: ImportStatic, Namespace: "yaml"},
			{Module: "requests", Item: "get", Kind: ImportStatic, Namespace: "requests//get"},
		},
		Definitions: []Definition{
			{Key: "src/app.py//helper", Kind: DefFunction, Name: "helper"},
		},
	}

	cases := []struct {
		name     string
		call     Call
		external bool
	}{
		{"local call", Call{Callee: "helper", Targets: []string{"src/app.py//helper"}}, false},
		{"bare name of a local definition", Call{Callee: "helper"}, false},
		{"import member call", Call{Callee: "yaml.load", Targets: []string{"yaml//load"}}, true},
		{"import item call", Call{Callee: "get", Targets: []string{"requests//get"}}, true},
		{"import namespace prefix of another module", Call{Callee: "yamlx.load", Targets: []string{"yamlx//load"}}, false},
		{"unbound call", Call{Callee: "eval"}, true},
		{"unbound target", Call{Callee: "f", Targets: []string{"exec"}}, true},
		{"one external target of two", Call{Callee: "f", Targets: []string{"src/app.py//helper", "yaml//dump"}}, true},
		{"member of a local object", Call{Callee: "p.parse", Targets: []string{"src/app.py//p//parse"}}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ix.Calls = []Call{tc.call}
			if tc.external {
				assert.Equal(t, []Call{tc.call}, ix.ExternalCalls())
			} else {
				assert.Empty(t, ix.ExternalCalls())
			}
		})
	}
}

func TestDynamicSitesJoinsFileAndDefinitionFlags(t *testing.T) {
	ix := &Index{
		Flags: []Flag{{Kind: FlagReflection, Line: 9}, {Kind: FlagEval, Line: 3}},
		Definitions: []Definition{
			{Key: "a", Flags: []Flag{{Kind: FlagMonkeyPatch, Line: 5}, {Kind: FlagEval, Line: 3}}},
			{Key: "b"},
		},
	}

	assert.Equal(t, []Flag{
		{Kind: FlagEval, Line: 3},
		{Kind: FlagMonkeyPatch, Line: 5},
		{Kind: FlagReflection, Line: 9},
	}, ix.DynamicSites())
	assert.Len(t, ix.Flags, 2)
}

func TestKey(t *testing.T) {
	ix := fullIndex()

	assert.Equal(t, Key{
		Path:           "src/app.py",
		ContentHash:    HashContent([]byte("app")),
		IndexVersion:   Version,
		GrammarVersion: grammar.Version(langcode.Python),
		Language:       langcode.Python,
	}, ix.Key())
	assert.Equal(t, NewKey(ix.Path, ix.ContentHash, ix.Language), ix.Key())

	ix.Version = "0"
	assert.NotEqual(t, NewKey(ix.Path, ix.ContentHash, ix.Language), ix.Key())
}

func TestHashContent(t *testing.T) {
	assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", HashContent(nil))
	assert.NotEqual(t, HashContent([]byte("a")), HashContent([]byte("b")))
}
