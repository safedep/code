package depsusage

import (
	"context"
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/lang"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/stretchr/testify/assert"
)

// var commonIgnoredTypesList = []string{"comment", "import_statement", "import_from_statement", "import_declaration"}

func TestIsIgnoredNode(t *testing.T) {
	commonIgnoredTestcases := []struct {
		snippet      string
		ignored      bool
		languageCode core.LanguageCode
	}{
		{
			snippet:      `# commented`,
			ignored:      true,
			languageCode: core.LanguageCodePython,
		},
		{
			snippet: `import os
import sys as s`,
			ignored:      true,
			languageCode: core.LanguageCodePython,
		},
		{
			snippet: `from os import path
from sys import path as p
`,
			ignored:      true,
			languageCode: core.LanguageCodePython,
		},
		{
			snippet: `x = someFunction()
print(x)`,
			ignored:      false,
			languageCode: core.LanguageCodePython,
		},
		{
			snippet: `// comment
/* comment2 */
`,
			ignored:      true,
			languageCode: core.LanguageCodeJavascript,
		},
		{
			snippet: `import express from 'express'
import { somevar } from 'somemodule'
import * as somevar from 'somemodule'
`,
			ignored:      true,
			languageCode: core.LanguageCodeJavascript,
		},
		{
			snippet:      `// commented`,
			ignored:      true,
			languageCode: core.LanguageCodeGo,
		},
		{
			snippet:      `import "fmt"`,
			ignored:      true,
			languageCode: core.LanguageCodeGo,
		},
		{
			snippet: `somevar := fncall("arg1")
somevar2 = uint32(1)`,
			ignored:      false,
			languageCode: core.LanguageCodeGo,
		},
	}

	for _, tc := range commonIgnoredTestcases {
		parser := sitter.NewParser()
		language, err := lang.GetLanguage(string(tc.languageCode))
		assert.NoError(t, err)
		parser.SetLanguage(language.Language())

		t.Run(tc.snippet, func(t *testing.T) {
			data := []byte(tc.snippet)

			tree, err := parser.ParseCtx(context.Background(), nil, data)
			assert.NoError(t, err)
			defer tree.Close()

			rootNode := tree.RootNode()

			for i := range int(rootNode.ChildCount()) {
				node := rootNode.Child(i)
				isIgnored := isIgnoredNode(node, &language, &data)
				assert.Equal(t, tc.ignored, isIgnored)
			}
		})
	}

	t.Run("IgnoreJavascriptRequireImports", func(t *testing.T) {
		parser := sitter.NewParser()
		language, err := lang.GetLanguage(string(core.LanguageCodeJavascript))
		assert.NoError(t, err)
		parser.SetLanguage(language.Language())

		testcases := []struct {
			snippet string
			ignored bool
		}{
			{snippet: `const express = require('express')`, ignored: true},
			{snippet: `const { somevar } = require('somemodule')`, ignored: true},
			{snippet: `const somevar = require('somemodule')`, ignored: true},
			{snippet: `const abc = somefunc(25)`, ignored: false},
			{snippet: `const abc = require('somefunc')(25)`, ignored: false},
			{snippet: `let abc = "xyz"`, ignored: false},
		}
		for _, testcase := range testcases {
			t.Run(testcase.snippet, func(t *testing.T) {
				data := []byte(testcase.snippet)
				tree, err := parser.ParseCtx(context.Background(), nil, data)
				assert.NoError(t, err)
				defer tree.Close()

				rootNode := tree.RootNode()
				assert.Equal(t, uint32(1), rootNode.ChildCount())
				lexicalDeclarationNode := rootNode.Child(0)
				assert.Equal(t, "lexical_declaration", lexicalDeclarationNode.Type())

				assert.Equal(t, uint32(2), lexicalDeclarationNode.ChildCount())
				variableDeclaratorNode := lexicalDeclarationNode.Child(1)
				assert.Equal(t, "variable_declarator", variableDeclaratorNode.Type())

				isIgnored := isIgnoredNode(variableDeclaratorNode, &language, &data)
				assert.Equal(t, testcase.ignored, isIgnored)
			})
		}
	})
}
