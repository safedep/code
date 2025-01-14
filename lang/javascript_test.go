package lang

import (
	"context"
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
	"github.com/safedep/code/fs"
	"github.com/safedep/code/parser"
	"github.com/stretchr/testify/assert"
)

func TestJavascriptLanguageMeta(t *testing.T) {
	t.Run("Name", func(t *testing.T) {
		l := &javascriptLanguage{}
		assert.Equal(t, javascriptLanguageName, l.Name())
	})

	t.Run("Code", func(t *testing.T) {
		l := &javascriptLanguage{}
		assert.Equal(t, core.LanguageCodeJavascript, l.Meta().Code)
	})

	t.Run("ObjectOriented", func(t *testing.T) {
		l := &javascriptLanguage{}
		assert.True(t, l.Meta().ObjectOriented)
	})
}

type ImportExpectations struct {
	filePath string
	imports  []ast.ImportJsonString
}

var importExpectations = []ImportExpectations{
	{
		filePath: "fixtures/imports.js",
		imports: []ast.ImportJsonString{
			"{ModuleName: express, ModuleItem: , ModuleAlias: express, WildcardImport: false}",
			"{ModuleName: dotenv, ModuleItem: , ModuleAlias: DotEnv, WildcardImport: false}",
			"{ModuleName: buffer, ModuleItem: , ModuleAlias: buffer, WildcardImport: false}",
			"{ModuleName: cluster, ModuleItem: , ModuleAlias: Cluster, WildcardImport: false}",
			"{ModuleName: @gilbarbara/eslint-config, ModuleItem: , ModuleAlias: EslintConfig, WildcardImport: false}",
			"{ModuleName: ./config.js, ModuleItem: , ModuleAlias: config, WildcardImport: false}",
			"{ModuleName: ./utils.js, ModuleItem: , ModuleAlias: utils, WildcardImport: false}",
			"{ModuleName: ../utils/helper.js, ModuleItem: , ModuleAlias: helper, WildcardImport: false}",
			"{ModuleName: ../utils/sideeffects.js, ModuleItem: , ModuleAlias: sideffects, WildcardImport: false}",
			"{ModuleName: ./data1.json, ModuleItem: , ModuleAlias: jsonData, WildcardImport: false}",
			"{ModuleName: ./data2.json, ModuleItem: , ModuleAlias: jsonData2, WildcardImport: false}",
			"{ModuleName: lodash, ModuleItem: , ModuleAlias: lodash, WildcardImport: false}",
			"{ModuleName: ./math-utils, ModuleItem: , ModuleAlias: mathUtils, WildcardImport: false}",
			"{ModuleName: ./dynamic-module.js, ModuleItem: , ModuleAlias: dynamicModule, WildcardImport: false}",
			"{ModuleName: react-dom, ModuleItem: , ModuleAlias: ReactDOM, WildcardImport: false}",
			"{ModuleName: react-dom, ModuleItem: render, ModuleAlias: render, WildcardImport: false}",
			"{ModuleName: react-dom, ModuleItem: flushSync, ModuleAlias: flushIt, WildcardImport: false}",
			"{ModuleName: constants, ModuleItem: EADDRINUSE, ModuleAlias: EADDRINUSE, WildcardImport: false}",
			"{ModuleName: constants, ModuleItem: EACCES, ModuleAlias: EACCES, WildcardImport: false}",
			"{ModuleName: constants, ModuleItem: EAGAIN, ModuleAlias: EAGAIN, WildcardImport: false}",
			"{ModuleName: chalk/ansi-styles, ModuleItem: hex, ModuleAlias: hex, WildcardImport: false}",
			"{ModuleName: virtual-dom, ModuleItem: patch, ModuleAlias: patch, WildcardImport: false}",
			"{ModuleName: react, ModuleItem: useState, ModuleAlias: useMyState, WildcardImport: false}",
			"{ModuleName: @xyz/pqr, ModuleItem: foo, ModuleAlias: fooAlias, WildcardImport: false}",
		},
	},
}

func TestJavascriptLanguageResolvers(t *testing.T) {
	t.Run("ResolversExists", func(t *testing.T) {
		l := &javascriptLanguage{}
		resolvers := l.Resolvers()
		assert.NotNil(t, resolvers)
	})

	t.Run("ResolveImports", func(t *testing.T) {
		l := &javascriptLanguage{}

		importExpectationsMapper := make(map[string][]ast.ImportJsonString)
		importFilePaths := []string{}
		for _, ie := range importExpectations {
			importFilePaths = append(importFilePaths, ie.filePath)
			importExpectationsMapper[ie.filePath] = ie.imports
		}

		fileParser, err := parser.NewParser(l)
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
				assert.Equal(t, expectedImport, imports[i].JsonString())
			}

			return err
		})
		assert.NoError(t, err)
	})
}
