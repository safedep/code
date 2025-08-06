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

var goImportExpectations = []ImportExpectations{
	{
		filePath: "fixtures/imports.go",
		imports: []string{
			"ImportNode{ModuleName: \"fmt\", ModuleItem: , ModuleAlias: \"fmt\", WildcardImport: false}",
			"ImportNode{ModuleName: \"github.com/safedep/code/parser\", ModuleItem: , ModuleAlias: \"github.com/safedep/code/parser\", WildcardImport: false}",
			"ImportNode{ModuleName: \"os\", ModuleItem: , ModuleAlias: osalias, WildcardImport: false}",
			"ImportNode{ModuleName: \"github.com/safedep/code/core\", ModuleItem: , ModuleAlias: codeccorealias, WildcardImport: false}",
			"ImportNode{ModuleName: \"embed\", ModuleItem: , ModuleAlias: , WildcardImport: true}",
			"ImportNode{ModuleName: \"math\", ModuleItem: , ModuleAlias: , WildcardImport: true}",
			"ImportNode{ModuleName: \"crypto\", ModuleItem: , ModuleAlias: cryptoalias, WildcardImport: false}",
			"ImportNode{ModuleName: \"github.com/gin-contrib/pprof\", ModuleItem: , ModuleAlias: , WildcardImport: true}",
			"ImportNode{ModuleName: \"github.com/smacker/go-tree-sitter\", ModuleItem: , ModuleAlias: gotreesitteralias, WildcardImport: false}",
			"ImportNode{ModuleName: \"net/http\", ModuleItem: , ModuleAlias: , WildcardImport: true}",
			"ImportNode{ModuleName: \"strings\", ModuleItem: , ModuleAlias: \"strings\", WildcardImport: false}",
		},
	},
}

var goFunctionExpectations = map[string][]string{
	"fixtures/functions.go": {
		"FunctionDeclarationNode{Name: simpleFunction, Type: function, Access: package, ParentClass: }",
		"FunctionDeclarationNode{Name: functionWithArgs, Type: function, Access: package, ParentClass: }",
		"FunctionDeclarationNode{Name: MyMethod, Type: method, Access: public, ParentClass: MyStruct}",
		"FunctionDeclarationNode{Name: privateFunction, Type: function, Access: package, ParentClass: }",
		"FunctionDeclarationNode{Name: ExportedFunction, Type: function, Access: public, ParentClass: }",
		"FunctionDeclarationNode{Name: unexportedFunction, Type: function, Access: package, ParentClass: }",
		"FunctionDeclarationNode{Name: _underscoreFunction, Type: function, Access: package, ParentClass: }",
		"FunctionDeclarationNode{Name: ExportedMethod, Type: method, Access: public, ParentClass: myPrivateStruct}",
		"FunctionDeclarationNode{Name: unexportedMethod, Type: method, Access: package, ParentClass: myPrivateStruct}",
		"FunctionDeclarationNode{Name: PublicMethod, Type: method, Access: public, ParentClass: PublicStruct}",
		"FunctionDeclarationNode{Name: privateMethod, Type: method, Access: package, ParentClass: PublicStruct}",
	},
}

func TestGoLanguageResolvers(t *testing.T) {
	t.Run("ResolversExists", func(t *testing.T) {
		l, err := lang.NewGoLanguage()
		assert.NoError(t, err)
		resolvers := l.Resolvers()
		assert.NotNil(t, resolvers)
	})

	t.Run("ResolveImports", func(t *testing.T) {
		l, err := lang.NewGoLanguage()
		assert.NoError(t, err)

		importExpectationsMapper := make(map[string][]string)
		importFilePaths := []string{}
		for _, ie := range goImportExpectations {
			importFilePaths = append(importFilePaths, ie.filePath)
			importExpectationsMapper[ie.filePath] = ie.imports
		}

		goLanguage, err := lang.NewGoLanguage()
		assert.NoError(t, err)

		fileParser, err := parser.NewParser([]core.Language{goLanguage})
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
		l, err := lang.NewGoLanguage()
		assert.NoError(t, err)

		var filePaths []string
		for path := range goFunctionExpectations {
			filePaths = append(filePaths, path)
		}

		goLanguage, err := lang.NewGoLanguage()
		assert.NoError(t, err)

		fileParser, err := parser.NewParser([]core.Language{goLanguage})
		assert.NoError(t, err)

		fileSystem, err := fs.NewLocalFileSystem(fs.LocalFileSystemConfig{
			AppDirectories: filePaths,
		})
		assert.NoError(t, err)

		err = fileSystem.EnumerateApp(context.Background(), func(f core.File) error {
			parseTree, err := fileParser.Parse(context.Background(), f)
			assert.NoError(t, err)

			functions, err := l.Resolvers().ResolveFunctions(parseTree)
			assert.NoError(t, err)

			expectedFunctions, ok := goFunctionExpectations[f.Name()]
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
