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

var pythonImportExpectations = []ImportExpectations{
	{
		filePath: "fixtures/imports.py",
		imports: []string{
			"ImportNode{ModuleName: prototurk, ModuleItem: , ModuleAlias: prototurk, WildcardImport: false}",
			"ImportNode{ModuleName: sys, ModuleItem: , ModuleAlias: sys, WildcardImport: false}",
			"ImportNode{ModuleName: pandas, ModuleItem: , ModuleAlias: pd, WildcardImport: false}",
			"ImportNode{ModuleName: langchain.chat_models, ModuleItem: , ModuleAlias: customchat, WildcardImport: false}",
			"ImportNode{ModuleName: matplotlib.pyplot, ModuleItem: , ModuleAlias: plt, WildcardImport: false}",
			"ImportNode{ModuleName: ujson, ModuleItem: , ModuleAlias: ujson, WildcardImport: false}",
			"ImportNode{ModuleName: plistlib, ModuleItem: , ModuleAlias: plb, WildcardImport: false}",
			"ImportNode{ModuleName: simplejson, ModuleItem: , ModuleAlias: smpjson, WildcardImport: false}",
			"ImportNode{ModuleName: seaborn, ModuleItem: , ModuleAlias: , WildcardImport: true}",
			"ImportNode{ModuleName: flask.helpers, ModuleItem: , ModuleAlias: , WildcardImport: true}",
			"ImportNode{ModuleName: xyz.pqr.mno, ModuleItem: , ModuleAlias: , WildcardImport: true}",
			"ImportNode{ModuleName: math, ModuleItem: sqrt, ModuleAlias: sqrt, WildcardImport: false}",
			"ImportNode{ModuleName: langchain_community, ModuleItem: llms, ModuleAlias: llms, WildcardImport: false}",
			"ImportNode{ModuleName: odbc, ModuleItem: connect, ModuleAlias: connect, WildcardImport: false}",
			"ImportNode{ModuleName: odbc, ModuleItem: fetch, ModuleAlias: fetch, WildcardImport: false}",
			"ImportNode{ModuleName: sklearn, ModuleItem: datasets, ModuleAlias: ds, WildcardImport: false}",
			"ImportNode{ModuleName: sklearn, ModuleItem: metric, ModuleAlias: metric, WildcardImport: false}",
			"ImportNode{ModuleName: sklearn, ModuleItem: preprocessing, ModuleAlias: pre, WildcardImport: false}",
			"ImportNode{ModuleName: oauthlib.oauth2, ModuleItem: WebApplicationClient, ModuleAlias: WAC, WildcardImport: false}",
			"ImportNode{ModuleName: oauthlib.oauth2, ModuleItem: WebApplicationServer, ModuleAlias: WebApplicationServer, WildcardImport: false}",
		},
	},
}

func TestPythonLanguageResolvers(t *testing.T) {
	t.Run("ResolversExists", func(t *testing.T) {
		l, err := lang.NewPythonLanguage()
		assert.NoError(t, err)
		resolvers := l.Resolvers()
		assert.NotNil(t, resolvers)
	})

	t.Run("ResolveImports", func(t *testing.T) {
		l, err := lang.NewPythonLanguage()
		assert.NoError(t, err)

		importExpectationsMapper := make(map[string][]string)
		importFilePaths := []string{}
		for _, ie := range pythonImportExpectations {
			importFilePaths = append(importFilePaths, ie.filePath)
			importExpectationsMapper[ie.filePath] = ie.imports
		}

		pythonLanugage, err := lang.NewPythonLanguage()
		assert.NoError(t, err)

		fileParser, err := parser.NewParser([]core.Language{pythonLanugage})
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
