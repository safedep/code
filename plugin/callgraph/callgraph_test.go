package callgraph

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
	"github.com/safedep/code/lang"
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
// buildAndClose builds the call graph of a source and closes its tree, so
// each test reads a graph that outlives the tree.
func buildAndClose(t *testing.T, language core.Language, name, source string) *CallGraph {
	t.Helper()

	tree, err := parser.ParseBytes(t.Context(), language, []byte(source), parser.WithFileName(name))
	require.NoError(t, err)
	cg, err := Build(t.Context(), tree)
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
		Kind:            CallCall,
		Targets:         []string{"yaml//safe_load"},
		Site:            ast.NodePosition{StartByte: 69, EndByte: 83, StartLine: 5, EndLine: 5, StartColumn: 12, EndColumn: 26},
		Text:            "yaml.safe_load",
		Receiver:        "yaml",
		Method:          "safe_load",
		Arguments:       []CallArgument{{Values: []ArgumentValue{}}},
	})
	assert.Contains(t, read.CallsTo, CallReference{
		CalleeNamespace: "open",
		Kind:            CallCall,
		Targets:         []string{"open"},
		Site:            ast.NodePosition{StartByte: 84, EndByte: 88, StartLine: 5, EndLine: 5, StartColumn: 27, EndColumn: 31},
		Text:            "open",
		Method:          "open",
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
	assert.Equal(t, cg.Imports, decoded.Imports)

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
			name:     "javascript default import",
			language: javascriptLanguage(t),
			file:     "a.js",
			source:   "import y from 'js-yaml';\ny.load(x);\n",
			caller:   "a.js",
			callee:   "js-yaml//load",
			targets:  []string{"js-yaml//load"},
		},
		{
			name:     "javascript default import by name",
			language: javascriptLanguage(t),
			file:     "a.js",
			source:   "import { default as y } from 'js-yaml';\ny.load(x);\n",
			caller:   "a.js",
			callee:   "js-yaml//load",
			targets:  []string{"js-yaml//load"},
		},
		{
			name:     "javascript namespace import",
			language: javascriptLanguage(t),
			file:     "a.js",
			source:   "import * as y from 'js-yaml';\ny.load(x);\n",
			caller:   "a.js",
			callee:   "js-yaml//load",
			targets:  []string{"js-yaml//load"},
		},
		{
			name:     "javascript call of an import holds the import",
			language: javascriptLanguage(t),
			file:     "a.js",
			source:   "const express = require('express');\nconst app = express();\napp.get('/', h);\n",
			caller:   "a.js",
			callee:   "express//get",
			targets:  []string{"express//get"},
		},
		{
			name:     "javascript call of a local function holds no import",
			language: javascriptLanguage(t),
			file:     "a.js",
			source:   "function make() {}\nconst app = make();\napp.get('/', h);\n",
			caller:   "a.js",
			callee:   "app//get",
			targets:  []string{"app//get"},
		},
		{
			name:     "javascript require destructure with a new name",
			language: javascriptLanguage(t),
			file:     "a.js",
			source:   "const {load: l} = require('js-yaml');\nl(x);\n",
			caller:   "a.js",
			callee:   "l",
			targets:  []string{"js-yaml//load"},
		},
		{
			name:     "javascript require destructure",
			language: javascriptLanguage(t),
			file:     "a.js",
			source:   "const {load} = require('js-yaml');\nload(x);\n",
			caller:   "a.js",
			callee:   "load",
			targets:  []string{"js-yaml//load"},
		},
		{
			name:     "go aliased import",
			language: goLanguage(t),
			file:     "a.go",
			source:   "package a\nimport j \"encoding/json\"\nfunc f() { j.Marshal(x) }\n",
			caller:   "a.go//f",
			callee:   "encoding//json//Marshal",
			targets:  []string{"encoding//json//Marshal"},
		},
		{
			name:     "go import with a major version",
			language: goLanguage(t),
			file:     "a.go",
			source:   "package a\nimport \"github.com/golang-jwt/jwt/v5\"\nfunc f() { jwt.Parse(x) }\n",
			caller:   "a.go//f",
			callee:   "github.com//golang-jwt//jwt//v5//Parse",
			targets:  []string{"github.com//golang-jwt//jwt//v5//Parse"},
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

func TestCallKindAndReceiver(t *testing.T) {
	type call struct {
		caller, callee   string
		kind             CallKind
		receiver, method string
		text             string
	}
	cases := []struct {
		name     string
		language core.Language
		file     string
		source   string
		calls    []call
	}{
		{
			name:     "javascript",
			language: javascriptLanguage(t),
			file:     "a.js",
			source:   "import x from 'x';\nclass A { constructor() {} run() {} }\nx.a.b(1);\nnew A();\nimport('y');\n",
			calls: []call{
				{"a.js//A", "a.js//A//constructor", CallSynthetic, "", "", ""},
				{"a.js//A//this//constructor", "a.js//A//constructor", CallSynthetic, "", "", ""},
				{"a.js//A//this//run", "a.js//A//run", CallSynthetic, "", "", ""},
				{"a.js", "x//a//b", CallCall, "x.a", "b", "x.a.b"},
				{"a.js", "a.js//A", CallNew, "", "A", "A"},
				{"a.js", "y//*", CallImport, "", "import", "import"},
			},
		},
		{
			name:     "python",
			language: pythonLanguage(t),
			file:     "a.py",
			source:   "from os import *\nimport yaml\nyaml.load(s)\n",
			calls: []call{
				{"a.py", "os//*", CallWildcardImport, "", "", "from os import *"},
				{"a.py", "yaml//load", CallCall, "yaml", "load", "yaml.load"},
			},
		},
		{
			name:     "go",
			language: goLanguage(t),
			file:     "a.go",
			source:   "package a\nimport \"os\"\nfunc f() { os.Exit(1) }\n",
			calls: []call{
				{"a.go", "a.go//f", CallSynthetic, "", "", ""},
				{"a.go//f", "os//Exit", CallCall, "os", "Exit", "os.Exit"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cg := buildAndClose(t, tc.language, tc.file, tc.source)

			var calls []call
			for _, node := range cg.order {
				for _, ref := range node.CallsTo {
					calls = append(calls, call{node.Namespace, ref.CalleeNamespace, ref.Kind, ref.Receiver, ref.Method, ref.Text})
					if ref.Kind == CallSynthetic {
						assert.Zero(t, ref.Site, "a synthetic edge has no site")
					}
				}
			}
			assert.ElementsMatch(t, tc.calls, calls)
		})
	}
}

func TestImportsInSourceOrder(t *testing.T) {
	cases := []struct {
		name     string
		language core.Language
		file     string
		source   string
		imports  []Import
	}{
		{
			name:     "python",
			language: pythonLanguage(t),
			file:     "a.py",
			source:   "from yaml import load as l\nimport os.path\nimport json as j\nfrom m import *\n",
			imports: []Import{
				{Module: "yaml", Item: "load", Alias: "l", Namespace: "yaml//load"},
				{Module: "os.path", Alias: "os", Namespace: "os"},
				{Module: "json", Alias: "j", Namespace: "json"},
				{Module: "m", Namespace: "m", Wildcard: true},
			},
		},
		{
			name:     "javascript",
			language: javascriptLanguage(t),
			file:     "a.js",
			source:   "import { default as y } from 'js-yaml';\nimport 'polyfill';\nconst { load: l } = require('./util');\n",
			imports: []Import{
				{Module: "js-yaml", Item: "default", Alias: "y", Namespace: "js-yaml"},
				{Module: "polyfill", Namespace: "polyfill", Wildcard: true},
				{Module: "./util", Item: "load", Alias: "l", Namespace: ".//util//load"},
			},
		},
		{
			name:     "go",
			language: goLanguage(t),
			file:     "a.go",
			source:   "package a\nimport (\n\tj \"encoding/json\"\n\t\"github.com/golang-jwt/jwt/v5\"\n)\n",
			imports: []Import{
				{Module: "encoding/json", Alias: "j", Namespace: "encoding//json"},
				{Module: "github.com/golang-jwt/jwt/v5", Alias: "jwt", Namespace: "github.com//golang-jwt//jwt//v5"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cg := buildAndClose(t, tc.language, tc.file, tc.source)

			require.Len(t, cg.Imports, len(tc.imports))
			for i, imp := range cg.Imports {
				assert.Positive(t, imp.Position.StartLine)
				if i > 0 {
					assert.Less(t, cg.Imports[i-1].Position.StartByte, imp.Position.StartByte, "source order")
				}
				imp.Position = ast.NodePosition{}
				assert.Equal(t, tc.imports[i], imp)
			}
		})
	}
}

func TestSiteIsTheCalleeExpression(t *testing.T) {
	cases := []struct {
		name     string
		language core.LanguageCode
		file     string
		source   string
		callee   string
		text     string
		column   uint32
		receiver string
		method   string
	}{
		{"python call", core.LanguageCodePython, "a.py", "import os\nos.getenv(x)\n", "os//getenv", "os.getenv", 1, "os", "getenv"},
		{"javascript object creation", core.LanguageCodeJavascript, "a.js", "import sqlite3 from 'sqlite3';\nx = new sqlite3.Database(':memory:');\n", "sqlite3//Database", "sqlite3.Database", 9, "sqlite3", "Database"},
		{"go call", core.LanguageCodeGo, "a.go", "package a\nimport \"os\"\nfunc f() { os.Exit(1) }\n", "os//Exit", "os.Exit", 12, "os", "Exit"},
		{"java method invocation", core.LanguageCodeJava, "A.java", "class A { void f() { Math.max(1, 2); } }\n", "Math//max", "Math.max", 22, "Math", "max"},
		{"java object creation", core.LanguageCodeJava, "A.java", "import java.util.Random;\nclass A { void f() { new Random(1); } }\n", "Random", "Random", 26, "", "Random"},
		{"ruby call with a receiver", core.LanguageCodeRuby, "a.rb", "Foo.bar(1)\n", "Foo//bar", "Foo.bar", 1, "Foo", "bar"},
		{"php member call", core.LanguageCodePHP, "a.php", "<?php\n$c->send(1);\n", "$c//send", "$c->send", 1, "$c", "send"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			language, err := lang.GetLanguage(string(tc.language))
			require.NoError(t, err)
			cg := buildAndClose(t, language, tc.file, tc.source)

			var found *CallReference
			for _, node := range cg.order {
				for i, ref := range node.CallsTo {
					if ref.CalleeNamespace == tc.callee {
						found = &node.CallsTo[i]
					}
				}
			}
			require.NotNil(t, found, "no call to %s", tc.callee)
			assert.Equal(t, tc.text, found.Text)
			assert.Equal(t, tc.column, found.Site.StartColumn)
			assert.Equal(t, tc.receiver, found.Receiver)
			assert.Equal(t, tc.method, found.Method)
			assert.Equal(t, uint32(len(tc.text)), found.Site.EndByte-found.Site.StartByte)
		})
	}
}

func TestGoMethodKeyNamesTheReceiverType(t *testing.T) {
	cases := []struct {
		name, receiver, key string
	}{
		{"value", "s S", "a.go//S//M"},
		{"pointer", "s *S", "a.go//S//M"},
		{"generic", "s S[T]", "a.go//S//M"},
		{"generic pointer", "s *S[K, V]", "a.go//S//M"},
		{"no name", "*S", "a.go//S//M"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cg := buildAndClose(t, goLanguage(t), "a.go", "package a\nfunc ("+tc.receiver+") M() {}\n")
			require.Contains(t, cg.Nodes, tc.key)
			assert.Equal(t, "method_declaration", cg.Nodes[tc.key].Kind)
		})
	}
}

func TestGoCallsOnTypedValues(t *testing.T) {
	cases := []struct {
		name, source, callee string
	}{
		{"composite literal", "T{}.M()", "T//M"},
		{"pointer to a composite literal", "(&T{}).M()", "T//M"},
		{"composite literal of an import", "store.T{}.M()", "example.com//store//T//M"},
		{"type assertion", "x.(*T).M()", "T//M"},
		{"generic function", "work[string](x)", "work"},
		{"generic function of an import", "store.Do[int, string](x)", "example.com//store//Do"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := "package a\nimport \"example.com/store\"\nfunc f() {\n" + tc.source + "\n}\n"
			cg := buildAndClose(t, goLanguage(t), "a.go", source)
			var callees []string
			for _, ref := range cg.Nodes["a.go//f"].CallsTo {
				callees = append(callees, ref.CalleeNamespace)
			}
			assert.Contains(t, callees, tc.callee)
		})
	}
}

func TestJavaCallsTheLinkStepResolves(t *testing.T) {
	source := strings.Join([]string{
		"class A extends B {",
		"  A(int x) { super(x); init(); }",
		"  void run() {",
		"    start();",
		"    new Box<String>();",
		"    Runtime.getRuntime().exec(\"ls\");",
		"    new Runnable() { public void run() { anon(); } };",
		"  }",
		"  abstract void hook();",
		"  void anon() {}",
		"}",
		"interface I { void m(); default void d() { m(); } }",
		"enum E { ONE; void e() { work(); } }",
	}, "\n") + "\n"
	cg := buildAndClose(t, javaLanguage(t), "A.java", source)

	callees := map[string][]string{}
	for _, node := range cg.order {
		for _, ref := range node.CallsTo {
			if ref.Kind != CallSynthetic {
				callees[node.Namespace] = append(callees[node.Namespace], ref.CalleeNamespace)
			}
		}
	}
	assert.Equal(t, []string{"super", "init"}, callees["A.java//A//<init>"], "a constructor is <init> of its class")
	assert.Equal(t, []string{"start", "Box", "Runtime//getRuntime", "anon", "Runnable"}, callees["A.java//A//run"], "an anonymous class runs in its scope")
	assert.Equal(t, []string{"m"}, callees["A.java//I//d"], "a default method is a method of the interface")
	assert.Equal(t, []string{"work"}, callees["A.java//E//e"], "an enum is a class")
	assert.NotContains(t, cg.Nodes, "A.java//A//hook", "an abstract method is no definition")

	var methods []string
	for _, u := range cg.Unresolved() {
		methods = append(methods, u.Method)
	}
	assert.Equal(t, []string{"exec"}, methods, "a call on the value of a call is a record")
}

func TestClassExpressionIsAClass(t *testing.T) {
	source := strings.Join([]string{
		"module.exports = class { a() {} };",
		"exports.E = class { b() {} };",
		"const Named = class { c() {} };",
		"class Holder { static Inner = class { d() {} }; }",
		"const mix = (B) => class extends B { e() {} };",
		"new Named().c();",
	}, "\n") + "\n"
	cg := buildAndClose(t, javascriptLanguage(t), "a.js", source)
	for _, key := range []string{"a.js//default", "a.js//E", "a.js//Named", "a.js//Holder//Inner", "a.js//mix//<anonymous:5>"} {
		require.Contains(t, cg.Nodes, key)
		assert.Equal(t, "class", cg.Nodes[key].Kind, key)
	}
	for _, key := range []string{"a.js//default//a", "a.js//E//b", "a.js//Named//c", "a.js//Holder//Inner//d", "a.js//mix//<anonymous:5>//e"} {
		assert.Contains(t, cg.Nodes, key)
	}
	var targets []string
	for _, call := range cg.RootNode.CallsTo {
		targets = append(targets, call.Targets...)
	}
	assert.Contains(t, targets, "a.js//Named//c")
}

func TestArrowFunctionTakesOnlyTheNameThatHoldsIt(t *testing.T) {
	source := strings.Join([]string{
		"const a = wrap(() => one());",
		"const b = ready ? () => two() : null;",
		"const o = { m: () => three() };",
		"let c;",
		"c = (() => four());",
	}, "\n") + "\n"
	cg := buildAndClose(t, javascriptLanguage(t), "a.js", source)
	for key, kind := range map[string]string{"a.js//b": "arrow_function", "a.js//c": "arrow_function"} {
		require.Contains(t, cg.Nodes, key)
		assert.Equal(t, kind, cg.Nodes[key].Kind, key)
	}
	for _, key := range []string{"a.js//a", "a.js//o"} {
		if node, ok := cg.Nodes[key]; ok {
			assert.NotEqual(t, "arrow_function", node.Kind, "an arrow function in an argument or an object is no value of %s", key)
		}
	}
}
