package lang_test

import (
	"context"
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

		javaLanugage, err := lang.NewJavaLanguage()
		assert.NoError(t, err)

		fileParser, err := parser.NewParser([]core.Language{javaLanugage})
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
}
