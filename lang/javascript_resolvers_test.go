package lang_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fs"
	"github.com/safedep/code/lang"
	"github.com/safedep/code/parser"
	"github.com/stretchr/testify/assert"
)

var javascriptImportExpectations = []ImportExpectations{
	{
		filePath: "fixtures/imports.js",
		imports: []string{
			"ImportNode{ModuleName: express, ModuleItem: , ModuleAlias: express, WildcardImport: false}",
			"ImportNode{ModuleName: dotenv, ModuleItem: , ModuleAlias: DotEnv, WildcardImport: false}",
			"ImportNode{ModuleName: ./config.js, ModuleItem: , ModuleAlias: config, WildcardImport: false}",
			"ImportNode{ModuleName: ../utils/helper.js, ModuleItem: , ModuleAlias: helper, WildcardImport: false}",
			"ImportNode{ModuleName: ./data1.json, ModuleItem: , ModuleAlias: jsonData, WildcardImport: false}",
			"ImportNode{ModuleName: lodash, ModuleItem: , ModuleAlias: lodash, WildcardImport: false}",
			"ImportNode{ModuleName: ./math-utils, ModuleItem: , ModuleAlias: mathUtils, WildcardImport: false}",
			"ImportNode{ModuleName: ./dynamic-module.js, ModuleItem: , ModuleAlias: dynamicModule, WildcardImport: false}",
			"ImportNode{ModuleName: react-dom, ModuleItem: , ModuleAlias: ReactDOM, WildcardImport: false}",
			"ImportNode{ModuleName: react-dom, ModuleItem: render, ModuleAlias: render, WildcardImport: false}",
			"ImportNode{ModuleName: react-dom, ModuleItem: flushSync, ModuleAlias: flushIt, WildcardImport: false}",
			"ImportNode{ModuleName: constants, ModuleItem: EADDRINUSE, ModuleAlias: EADDRINUSE, WildcardImport: false}",
			"ImportNode{ModuleName: constants, ModuleItem: EACCES, ModuleAlias: EACCES, WildcardImport: false}",
			"ImportNode{ModuleName: constants, ModuleItem: EAGAIN, ModuleAlias: EAGAIN, WildcardImport: false}",
			"ImportNode{ModuleName: chalk/ansi-styles, ModuleItem: hex, ModuleAlias: hex, WildcardImport: false}",
			"ImportNode{ModuleName: react, ModuleItem: useEffect, ModuleAlias: useEffect, WildcardImport: false}",
			"ImportNode{ModuleName: react, ModuleItem: useState, ModuleAlias: useMyState, WildcardImport: false}",
			"ImportNode{ModuleName: buffer, ModuleItem: , ModuleAlias: buffer, WildcardImport: false}",
			"ImportNode{ModuleName: cluster, ModuleItem: , ModuleAlias: Cluster, WildcardImport: false}",
			"ImportNode{ModuleName: @gilbarbara/eslint-config, ModuleItem: , ModuleAlias: EslintConfig, WildcardImport: false}",
			"ImportNode{ModuleName: ./utils.js, ModuleItem: , ModuleAlias: utils, WildcardImport: false}",
			"ImportNode{ModuleName: ../utils/sideeffects.js, ModuleItem: , ModuleAlias: sideffects, WildcardImport: false}",
			"ImportNode{ModuleName: ./data2.json, ModuleItem: , ModuleAlias: jsonData2, WildcardImport: false}",
			"ImportNode{ModuleName: virtual-dom, ModuleItem: patch, ModuleAlias: patch, WildcardImport: false}",
			"ImportNode{ModuleName: @xyz/pqr, ModuleItem: foo, ModuleAlias: fooAlias, WildcardImport: false}",
			"ImportNode{ModuleName: @xyz/pqr, ModuleItem: bar, ModuleAlias: bar, WildcardImport: false}",
			"ImportNode{ModuleName: @xyz/mno, ModuleItem: baz2, ModuleAlias: baz2Alias, WildcardImport: false}",
			"ImportNode{ModuleName: @xyz/mno, ModuleItem: baz, ModuleAlias: baz, WildcardImport: false}",
			"ImportNode{ModuleName: @xyz/abc, ModuleItem: , ModuleAlias: a, WildcardImport: false}",
			"ImportNode{ModuleName: @xyz/xyz, ModuleItem: , ModuleAlias: b, WildcardImport: false}",
			"ImportNode{ModuleName: @xyz/pqr, ModuleItem: , ModuleAlias: c, WildcardImport: false}",
			"ImportNode{ModuleName: @xyz/mno, ModuleItem: baz, ModuleAlias: bazAlias, WildcardImport: false}",
			"ImportNode{ModuleName: @xyz/mno, ModuleItem: d, ModuleAlias: d, WildcardImport: false}",
		},
	},
}

var javascriptFunctionExpectations = map[string][]string{
	"fixtures/functions.js": {
		"FunctionDeclarationNode{Name: declaredFunction, Type: function, Access: public, ParentClass: }",
		"FunctionDeclarationNode{Name: arrowFunction, Type: arrow, Access: public, ParentClass: }",
		"FunctionDeclarationNode{Name: asyncFunction, Type: async, Access: public, ParentClass: }",
		"FunctionDeclarationNode{Name: constructor, Type: constructor, Access: public, ParentClass: MyClass}",
		"FunctionDeclarationNode{Name: myMethod, Type: method, Access: public, ParentClass: MyClass}",
		"FunctionDeclarationNode{Name: staticMethod, Type: method, Access: public, ParentClass: MyClass}",
		"FunctionDeclarationNode{Name: myProperty, Type: method, Access: public, ParentClass: MyClass}",
		"FunctionDeclarationNode{Name: myDecorator, Type: function, Access: public, ParentClass: }",
		"FunctionDeclarationNode{Name: decoratedMethod, Type: method, Access: public, ParentClass: ClassWithDecorator}",
	},
}

func TestJavascriptLanguageResolvers(t *testing.T) {
	t.Run("ResolversExists", func(t *testing.T) {
		l, err := lang.NewJavascriptLanguage()
		assert.NoError(t, err)
		resolvers := l.Resolvers()
		assert.NotNil(t, resolvers)
	})

	t.Run("ResolveImports", func(t *testing.T) {
		l, err := lang.NewJavascriptLanguage()
		assert.NoError(t, err)

		importExpectationsMapper := make(map[string][]string)
		importFilePaths := []string{}
		for _, ie := range javascriptImportExpectations {
			importFilePaths = append(importFilePaths, ie.filePath)
			importExpectationsMapper[ie.filePath] = ie.imports
		}

		javascriptLanguage, err := lang.NewJavascriptLanguage()
		assert.NoError(t, err)

		fileParser, err := parser.NewParser([]core.Language{javascriptLanguage})
		assert.NoError(t, err)

		fileSystem, err := fs.NewLocalFileSystem(fs.LocalFileSystemConfig{
			AppDirectories: importFilePaths,
		})
		assert.NoError(t, err)

		err = fileSystem.EnumerateApp(context.Background(), func(f core.File) error {
			parseTree, err := fileParser.Parse(context.Background(), f)
			assert.NoError(t, err)

			imports, err := l.Resolvers().ResolveImports(parseTree)
			assert.NoError(t, err)

			expectedImports, ok := importExpectationsMapper[f.Name()]
			assert.True(t, ok)

			assert.Equal(t, len(expectedImports), len(imports))
			for i, expectedImport := range expectedImports {
				assert.Equal(t, expectedImport, imports[i].String())
			}

			return err
		})
		assert.NoError(t, err)
	})

	t.Run("ResolveFunctions", func(t *testing.T) {
		var filePaths []string
		for path := range javascriptFunctionExpectations {
			filePaths = append(filePaths, path)
		}

		javascriptLanguage, err := lang.NewJavascriptLanguage()
		assert.NoError(t, err)

		fileParser, err := parser.NewParser([]core.Language{javascriptLanguage})
		assert.NoError(t, err)

		fileSystem, err := fs.NewLocalFileSystem(fs.LocalFileSystemConfig{
			AppDirectories: filePaths,
		})
		assert.NoError(t, err)

		err = fileSystem.EnumerateApp(context.Background(), func(f core.File) error {
			parseTree, err := fileParser.Parse(context.Background(), f)
			assert.NoError(t, err)

			functions, err := javascriptLanguage.Resolvers().ResolveFunctions(parseTree)
			assert.NoError(t, err)

			expectedFunctions, ok := javascriptFunctionExpectations[f.Name()]
			assert.True(t, ok)

			var foundFunctions []string
			for _, fun := range functions {
				foundFunctions = append(foundFunctions,
					fmt.Sprintf("FunctionDeclarationNode{Name: %s, Type: %s, Access: %s, ParentClass: %s}",
						fun.FunctionName(), fun.GetFunctionType(), fun.GetAccessModifier(), fun.GetParentClassName()))
			}

			assert.ElementsMatch(t, expectedFunctions, foundFunctions)

			return nil
		})
		assert.NoError(t, err)
	})
}
