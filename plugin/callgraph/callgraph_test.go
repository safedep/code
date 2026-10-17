package callgraph

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
	"github.com/safedep/code/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const configSource = `import yaml
from os import getenv as env

def read(path):
    return yaml.safe_load(open(path, "r"))

read(env("CONFIG"))
`

// buildAndClose builds the call graph of source and closes the tree before
// it returns the graph.
func buildAndClose(t *testing.T, language core.Language, name, source string) *CallGraph {
	t.Helper()

	tree, err := parser.ParseBytes(t.Context(), language, []byte(source), parser.WithFileName(name))
	require.NoError(t, err)
	cg, err := buildCallGraph(t.Context(), tree, language, name)
	tree.Close()
	require.NoError(t, err)
	return cg
}

func TestGraphOutlivesTheTree(t *testing.T) {
	cg := buildAndClose(t, pythonLanguage(t), "config.py", configSource)

	assert.Equal(t, core.LanguageCodePython, cg.Language)
	require.Same(t, cg.Nodes["config.py"], cg.RootNode)
	assert.Equal(t, "module", cg.RootNode.Kind)

	read := cg.Nodes["config.py//read"]
	require.NotNil(t, read)
	assert.Equal(t, "function_definition", read.Kind)
	assert.Equal(t, uint32(4), read.Position.StartLine)
	assert.Equal(t, uint32(5), read.Position.EndLine)

	assert.Contains(t, read.CallsTo, CallReference{
		CalleeNamespace: "yaml//safe_load",
		Targets:         []string{"yaml//safe_load"},
		Site:            ast.NodePosition{StartByte: 69, EndByte: 83, StartLine: 5, EndLine: 5, StartColumn: 12, EndColumn: 26},
		Text:            "yaml.safe_load",
		Arguments:       []CallArgument{{Values: []ArgumentValue{}}},
	})
	assert.Contains(t, read.CallsTo, CallReference{
		CalleeNamespace: "open",
		Targets:         []string{"open"},
		Site:            ast.NodePosition{StartByte: 84, EndByte: 88, StartLine: 5, EndLine: 5, StartColumn: 27, EndColumn: 31},
		Text:            "open",
		Arguments: []CallArgument{
			{Values: []ArgumentValue{{Namespace: "config.py//read//path"}}},
			{Values: []ArgumentValue{{Namespace: `"r"`, Literal: true}}},
		},
	})

	edges := 0
	for namespace, node := range cg.Nodes {
		assert.Equal(t, namespace, node.Namespace)
		if node.Kind != "" {
			assert.Positive(t, node.Position.StartLine, namespace)
		}
		for _, ref := range node.CallsTo {
			edges++
			assert.Contains(t, cg.Nodes, ref.CalleeNamespace)
			assert.LessOrEqual(t, len(ref.Text), maxSiteText)
			if ref.Text != "" {
				assert.Positive(t, ref.Site.StartLine, ref.Text)
			}
		}
	}
	assert.Positive(t, edges)

	assert.NotEmpty(t, cg.DFS())
	var dump strings.Builder
	require.NoError(t, cg.Dump(&dump))
	assert.Contains(t, dump.String(), "config.py//read (calls)=>")
}

func TestGraphJSONRoundTrip(t *testing.T) {
	cg := buildAndClose(t, pythonLanguage(t), "config.py", configSource+"later()\nlater()\n")
	require.Len(t, cg.RootNode.CallsTo, 4)
	require.Nil(t, cg.RootNode.CallsTo[2].Targets, "an unbound callee has no target")

	data, err := json.Marshal(cg)
	require.NoError(t, err)

	var decoded CallGraph
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, cg.FileName, decoded.FileName)
	assert.Equal(t, cg.Language, decoded.Language)
	assert.Equal(t, cg.Nodes, decoded.Nodes, "a nil and an empty list of targets keep their form")
	assert.Equal(t, cg.RootNode, decoded.RootNode)

	reached := func(items []DfsResultItem) []string {
		var namespaces []string
		for _, item := range items {
			namespaces = append(namespaces, item.Namespace)
		}
		return namespaces
	}
	decodedReach := reached(decoded.DFS())
	assert.Subset(t, reached(cg.DFS()), decodedReach)
	assert.Contains(t, decodedReach, "config.py//read")
	assert.Contains(t, decodedReach, "yaml//safe_load")
}

func TestCallTargetsResolveTheLocalName(t *testing.T) {
	cases := []struct {
		name     string
		language core.Language
		file     string
		source   string
		caller   string
		callee   string
		targets  []string
	}{
		{
			name:     "javascript named import",
			language: javascriptLanguage(t),
			file:     "a.js",
			source:   "import { load } from 'js-yaml';\nload(x);\n",
			caller:   "a.js",
			callee:   "load",
			targets:  []string{"js-yaml//load"},
		},
		{
			name:     "javascript aliased import",
			language: javascriptLanguage(t),
			file:     "a.js",
			source:   "import { parse as p } from 'js-yaml';\np(x);\n",
			caller:   "a.js",
			callee:   "p",
			targets:  []string{"js-yaml//parse"},
		},
		{
			name:     "python from import",
			language: pythonLanguage(t),
			file:     "a.py",
			source:   "from yaml import load\nload(x)\n",
			caller:   "a.py",
			callee:   "load",
			targets:  []string{"yaml//load"},
		},
		{
			name:     "python local function is its own target",
			language: pythonLanguage(t),
			file:     "a.py",
			source:   "def f():\n    pass\nf()\n",
			caller:   "a.py",
			callee:   "a.py//f",
			targets:  []string{"a.py//f"},
		},
		{
			name:     "python alias of two values",
			language: pythonLanguage(t),
			file:     "a.py",
			source:   "import json\nimport yaml\nload = yaml.load\nload = json.load\nload(x)\n",
			caller:   "a.py",
			callee:   "a.py//load",
			targets:  []string{"json//load", "yaml//load"},
		},
		{
			name:     "python assignment after a read of the name",
			language: pythonLanguage(t),
			file:     "a.py",
			source:   "import os\na = b\nprint(a)\nb = os\na.getenv('x')\n",
			caller:   "a.py",
			callee:   "os//getenv",
			targets:  []string{"os//getenv"},
		},
		{
			name:     "python cycle of two names",
			language: pythonLanguage(t),
			file:     "a.py",
			source:   "a = b\nb = a\na()\n",
			caller:   "a.py",
			callee:   "a.py//a",
			targets:  []string{"a.py//a"},
		},
		{
			name:     "python unbound callee has no target",
			language: pythonLanguage(t),
			file:     "a.py",
			source:   "g()\ndef g():\n    pass\n",
			caller:   "a.py",
			callee:   "g",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cg := buildAndClose(t, tc.language, tc.file, tc.source)

			require.Contains(t, cg.Nodes, tc.caller)
			var found *CallReference
			for i, ref := range cg.Nodes[tc.caller].CallsTo {
				if ref.CalleeNamespace == tc.callee {
					found = &cg.Nodes[tc.caller].CallsTo[i]
				}
			}
			require.NotNil(t, found, "no call to %s in %v", tc.callee, calleesOf(cg, tc.caller))
			assert.Equal(t, tc.targets, found.Targets)
		})
	}
}

func TestDumpSortsTheKeys(t *testing.T) {
	cg := buildAndClose(t, pythonLanguage(t), "z.py", "import os\ndef b():\n    os.getcwd()\ndef a():\n    b()\n")

	var dump strings.Builder
	require.NoError(t, cg.Dump(&dump))

	sections := strings.Split(dump.String(), "\n\n")
	require.Len(t, sections, 3)
	for _, section := range sections[:2] {
		lines := strings.Split(strings.TrimSpace(section), "\n")[1:]
		require.NotEmpty(t, lines)
		assert.IsIncreasing(t, lines)
	}
	assert.Contains(t, sections[1], "z.py//a (calls)=> [z.py//b]")
	assert.NotContains(t, dump.String(), "  print ", "a builtin with no edge stays out")
}

func TestDefinitionAfterTheCallKeepsItsKind(t *testing.T) {
	cg := buildAndClose(t, javascriptLanguage(t), "a.js", "g();\nfunction g() {}\n")

	node := cg.Nodes["a.js//g"]
	require.NotNil(t, node)
	assert.Equal(t, "function_declaration", node.Kind)
	assert.Equal(t, uint32(2), node.Position.StartLine)
	assert.Equal(t, uint32(2), node.Position.EndLine)

	require.Len(t, cg.RootNode.CallsTo, 1)
	call := cg.RootNode.CallsTo[0]
	assert.Equal(t, "a.js//g", call.CalleeNamespace)
	assert.True(t, call.Unbound)
	assert.Equal(t, []string{"a.js//g"}, call.Targets)
}

func TestDottedPythonImportBindsTheFirstSegment(t *testing.T) {
	cg := buildAndClose(t, pythonLanguage(t), "a.py", "import os.path\nos.path.join(x)\n")

	assert.Contains(t, cg.Nodes, "os//path")
	require.Len(t, cg.RootNode.CallsTo, 1)
	call := cg.RootNode.CallsTo[0]
	assert.Equal(t, "os//path//join", call.CalleeNamespace)
	assert.Equal(t, []string{"os//path//join"}, call.Targets)
	assert.Empty(t, cg.Unresolved())
}
