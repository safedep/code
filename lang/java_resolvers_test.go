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

var javaImportExpectations = []ImportExpectations{
	{
		filePath: "fixtures/Imports.java",
		imports: []string{
			"ImportNode{ModuleName: java.util.List, ModuleItem: , ModuleAlias: List, WildcardImport: false}",
			"ImportNode{ModuleName: java.util.Map.Entry, ModuleItem: , ModuleAlias: Entry, WildcardImport: false}",
			"ImportNode{ModuleName: mypackage.Helper, ModuleItem: , ModuleAlias: Helper, WildcardImport: false}",
			"ImportNode{ModuleName: java.util, ModuleItem: , ModuleAlias: , WildcardImport: true}",
			"ImportNode{ModuleName: java.lang.Math.PI, ModuleItem: , ModuleAlias: PI, WildcardImport: false}",
			"ImportNode{ModuleName: java.lang.Math, ModuleItem: , ModuleAlias: , WildcardImport: true}",
		},
	},
}

var javaFunctionExpectations = map[string][]string{
	"fixtures/Functions.java": {
		"FunctionDeclarationNode{Name: MyClassWithFunctions, Type: constructor, Access: public, ParentClass: MyClassWithFunctions}",
		"FunctionDeclarationNode{Name: publicMethod, Type: method, Access: public, ParentClass: MyClassWithFunctions}",
		"FunctionDeclarationNode{Name: protectedMethod, Type: method, Access: protected, ParentClass: MyClassWithFunctions}",
		"FunctionDeclarationNode{Name: privateMethod, Type: method, Access: private, ParentClass: MyClassWithFunctions}",
		"FunctionDeclarationNode{Name: staticMethod, Type: static_method, Access: public, ParentClass: MyClassWithFunctions}",
		"FunctionDeclarationNode{Name: toString, Type: method, Access: public, ParentClass: MyClassWithFunctions}",
		"FunctionDeclarationNode{Name: add, Type: static_method, Access: public, ParentClass: TestFunctions}",
	},
}

func TestJavaLanguageResolvers(t *testing.T) {
	t.Run("ResolversExists", func(t *testing.T) {
		l, err := lang.NewJavaLanguage()
		assert.NoError(t, err)
		resolvers := l.Resolvers()
		assert.NotNil(t, resolvers)
	})

	t.Run("ResolveImports", func(t *testing.T) {
		l, err := lang.NewJavaLanguage()
		assert.NoError(t, err)

		importExpectationsMapper := make(map[string][]string)
		importFilePaths := []string{}
		for _, ie := range javaImportExpectations {
			importFilePaths = append(importFilePaths, ie.filePath)
			importExpectationsMapper[ie.filePath] = ie.imports
		}

		javaLanguage, err := lang.NewJavaLanguage()
		assert.NoError(t, err)

		fileParser, err := parser.NewParser([]core.Language{javaLanguage})
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
		for path := range javaFunctionExpectations {
			filePaths = append(filePaths, path)
		}

		javaLanguage, err := lang.NewJavaLanguage()
		assert.NoError(t, err)

		fileParser, err := parser.NewParser([]core.Language{javaLanguage})
		assert.NoError(t, err)

		fileSystem, err := fs.NewLocalFileSystem(fs.LocalFileSystemConfig{
			AppDirectories: filePaths,
		})
		assert.NoError(t, err)

		err = fileSystem.EnumerateApp(context.Background(), func(f core.File) error {
			parseTree, err := fileParser.Parse(context.Background(), f)
			assert.NoError(t, err)

			functions, err := javaLanguage.Resolvers().ResolveFunctions(parseTree)
			assert.NoError(t, err)

			expectedFunctions, ok := javaFunctionExpectations[f.Name()]
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
