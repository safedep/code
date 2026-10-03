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

var typescriptImportExpectations = []ImportExpectations{
	{
		filePath: "fixtures/imports.ts",
		imports: []string{
			// ES module imports
			"ImportNode{ModuleName: express, ModuleItem: , ModuleAlias: express, WildcardImport: false}",
			"ImportNode{ModuleName: dotenv, ModuleItem: , ModuleAlias: DotEnv, WildcardImport: false}",
			"ImportNode{ModuleName: lodash, ModuleItem: , ModuleAlias: lodash, WildcardImport: false}",
			"ImportNode{ModuleName: ./math-utils, ModuleItem: , ModuleAlias: mathUtils, WildcardImport: false}",
			"ImportNode{ModuleName: react-dom, ModuleItem: , ModuleAlias: ReactDOM, WildcardImport: false}",
			"ImportNode{ModuleName: ./dynamic-module.js, ModuleItem: , ModuleAlias: dynamicModule, WildcardImport: false}",
			"ImportNode{ModuleName: ./config, ModuleItem: , ModuleAlias: config, WildcardImport: false}",
			"ImportNode{ModuleName: ../utils/helper, ModuleItem: , ModuleAlias: helper, WildcardImport: false}",
			// Type-only imports (matched by same ES queries — TS grammar treats them as import_statement)
			"ImportNode{ModuleName: express, ModuleItem: , ModuleAlias: Express, WildcardImport: false}",
			// Named imports
			"ImportNode{ModuleName: constants, ModuleItem: EADDRINUSE, ModuleAlias: EADDRINUSE, WildcardImport: false}",
			"ImportNode{ModuleName: constants, ModuleItem: EACCES, ModuleAlias: EACCES, WildcardImport: false}",
			"ImportNode{ModuleName: constants, ModuleItem: EAGAIN, ModuleAlias: EAGAIN, WildcardImport: false}",
			"ImportNode{ModuleName: chalk/ansi-styles, ModuleItem: hex, ModuleAlias: hex, WildcardImport: false}",
			"ImportNode{ModuleName: react, ModuleItem: useEffect, ModuleAlias: useEffect, WildcardImport: false}",
			"ImportNode{ModuleName: react, ModuleItem: useState, ModuleAlias: useMyState, WildcardImport: false}",
			"ImportNode{ModuleName: react-dom, ModuleItem: render, ModuleAlias: render, WildcardImport: false}",
			"ImportNode{ModuleName: react-dom, ModuleItem: flushSync, ModuleAlias: flushIt, WildcardImport: false}",
			"ImportNode{ModuleName: @angular/core, ModuleItem: Component, ModuleAlias: Component, WildcardImport: false}",
			// Type-only named imports
			"ImportNode{ModuleName: express, ModuleItem: Request, ModuleAlias: Request, WildcardImport: false}",
			"ImportNode{ModuleName: express, ModuleItem: Response, ModuleAlias: Response, WildcardImport: false}",
			"ImportNode{ModuleName: ./config, ModuleItem: Config, ModuleAlias: AppConfig, WildcardImport: false}",
			// CommonJS require
			"ImportNode{ModuleName: buffer, ModuleItem: , ModuleAlias: buffer, WildcardImport: false}",
			"ImportNode{ModuleName: virtual-dom, ModuleItem: patch, ModuleAlias: patch, WildcardImport: false}",
			"ImportNode{ModuleName: @xyz/pqr, ModuleItem: foo, ModuleAlias: fooAlias, WildcardImport: false}",
			"ImportNode{ModuleName: @xyz/pqr, ModuleItem: bar, ModuleAlias: bar, WildcardImport: false}",
		},
	},
	{
		filePath: "fixtures/imports_require.ts",
		imports: []string{
			"ImportNode{ModuleName: path, ModuleItem: , ModuleAlias: path, WildcardImport: false}",
			"ImportNode{ModuleName: ./types, ModuleItem: Foo, ModuleAlias: Foo, WildcardImport: false}",
			"ImportNode{ModuleName: fs, ModuleItem: , ModuleAlias: fs, WildcardImport: false}",
		},
	},
}

var typescriptFunctionExpectations = map[string][]string{
	"fixtures/functions.ts": {
		"FunctionDeclarationNode{Name: declaredFunction, Type: function, Access: public, ParentClass: }",
		"FunctionDeclarationNode{Name: arrowFunction, Type: arrow, Access: public, ParentClass: }",
		"FunctionDeclarationNode{Name: asyncFunction, Type: async, Access: public, ParentClass: }",
		"FunctionDeclarationNode{Name: identity, Type: function, Access: public, ParentClass: }",
		"FunctionDeclarationNode{Name: constructor, Type: constructor, Access: public, ParentClass: MyClass}",
		"FunctionDeclarationNode{Name: myMethod, Type: method, Access: public, ParentClass: MyClass}",
		"FunctionDeclarationNode{Name: staticMethod, Type: method, Access: public, ParentClass: MyClass}",
		"FunctionDeclarationNode{Name: myProperty, Type: method, Access: public, ParentClass: MyClass}",
		"FunctionDeclarationNode{Name: process, Type: method, Access: public, ParentClass: AbstractService}",
		"FunctionDeclarationNode{Name: helper, Type: method, Access: protected, ParentClass: AbstractService}",
		"FunctionDeclarationNode{Name: myDecorator, Type: function, Access: public, ParentClass: }",
		"FunctionDeclarationNode{Name: decoratedMethod, Type: method, Access: public, ParentClass: ClassWithDecorator}",
	},
}

func TestTypescriptLanguageResolvers(t *testing.T) {
	t.Run("ResolversExists", func(t *testing.T) {
		l, err := lang.NewTypescriptLanguage()
		assert.NoError(t, err)
		resolvers := l.Resolvers()
		assert.NotNil(t, resolvers)
	})

	t.Run("ResolveImports", func(t *testing.T) {
		importExpectationsMapper := make(map[string][]string)
		importFilePaths := []string{}
		for _, ie := range typescriptImportExpectations {
			importFilePaths = append(importFilePaths, ie.filePath)
			importExpectationsMapper[ie.filePath] = ie.imports
		}

		typescriptLanguage, err := lang.NewTypescriptLanguage()
		assert.NoError(t, err)

		fileParser, err := parser.NewParser([]core.Language{typescriptLanguage})
		assert.NoError(t, err)

		fileSystem, err := fs.NewLocalFileSystem(fs.LocalFileSystemConfig{
			AppDirectories: importFilePaths,
		})
		assert.NoError(t, err)

		err = fileSystem.EnumerateApp(context.Background(), func(f core.File) error {
			parseTree, err := fileParser.Parse(context.Background(), f)
			assert.NoError(t, err)

			imports, err := typescriptLanguage.Resolvers().ResolveImports(parseTree)
			assert.NoError(t, err)

			expectedImports, ok := importExpectationsMapper[f.Name()]
			assert.True(t, ok)

			var foundImports []string
			for _, imp := range imports {
				foundImports = append(foundImports, imp.String())
			}

			assert.ElementsMatch(t, expectedImports, foundImports)

			return err
		})
		assert.NoError(t, err)
	})

	t.Run("ResolveFunctions", func(t *testing.T) {
		var filePaths []string
		for path := range typescriptFunctionExpectations {
			filePaths = append(filePaths, path)
		}

		typescriptLanguage, err := lang.NewTypescriptLanguage()
		assert.NoError(t, err)

		fileParser, err := parser.NewParser([]core.Language{typescriptLanguage})
		assert.NoError(t, err)

		fileSystem, err := fs.NewLocalFileSystem(fs.LocalFileSystemConfig{
			AppDirectories: filePaths,
		})
		assert.NoError(t, err)

		err = fileSystem.EnumerateApp(context.Background(), func(f core.File) error {
			parseTree, err := fileParser.Parse(context.Background(), f)
			assert.NoError(t, err)

			functions, err := typescriptLanguage.Resolvers().ResolveFunctions(parseTree)
			assert.NoError(t, err)

			expectedFunctions, ok := typescriptFunctionExpectations[f.Name()]
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
