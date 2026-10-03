package lang_test

import (
	"context"
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fs"
	"github.com/safedep/code/lang"
	"github.com/safedep/code/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShapedLanguageResolvers(t *testing.T) {
	cases := []struct {
		language  core.LanguageCode
		filePath  string
		imports   []string
		functions []string
	}{
		{
			language: core.LanguageCodeCSharp,
			filePath: "fixtures/imports.cs",
			imports: []string{
				"ImportNode{ModuleName: System, ModuleItem: , ModuleAlias: , WildcardImport: true}",
				"ImportNode{ModuleName: System.Text.Json, ModuleItem: , ModuleAlias: , WildcardImport: true}",
				"ImportNode{ModuleName: System.Math, ModuleItem: , ModuleAlias: , WildcardImport: true}",
				"ImportNode{ModuleName: Microsoft.Extensions.Logging, ModuleItem: , ModuleAlias: , WildcardImport: true}",
				"ImportNode{ModuleName: System.Net.Http.HttpClient, ModuleItem: , ModuleAlias: Http, WildcardImport: false}",
				"ImportNode{ModuleName: OpenAI.Chat, ModuleItem: , ModuleAlias: , WildcardImport: true}",
			},
			functions: []string{"Service", "Run", "Local"},
		},
		{
			language: core.LanguageCodeRust,
			filePath: "fixtures/imports.rs",
			imports: []string{
				"ImportNode{ModuleName: std::collections::HashMap, ModuleItem: , ModuleAlias: HashMap, WildcardImport: false}",
				"ImportNode{ModuleName: serde, ModuleItem: Deserialize, ModuleAlias: Deserialize, WildcardImport: false}",
				"ImportNode{ModuleName: serde, ModuleItem: Serialize, ModuleAlias: Ser, WildcardImport: false}",
				"ImportNode{ModuleName: reqwest::Client, ModuleItem: , ModuleAlias: Http, WildcardImport: false}",
				"ImportNode{ModuleName: tokio, ModuleItem: , ModuleAlias: , WildcardImport: true}",
				"ImportNode{ModuleName: async_openai, ModuleItem: , ModuleAlias: async_openai, WildcardImport: false}",
				"ImportNode{ModuleName: async_openai, ModuleItem: CreateChatCompletionRequestArgs, ModuleAlias: CreateChatCompletionRequestArgs, WildcardImport: false}",
				"ImportNode{ModuleName: log, ModuleItem: , ModuleAlias: , WildcardImport: false}",
				"ImportNode{ModuleName: rand, ModuleItem: , ModuleAlias: random, WildcardImport: false}",
				"ImportNode{ModuleName: std, ModuleItem: io, ModuleAlias: io, WildcardImport: false}",
				"ImportNode{ModuleName: std, ModuleItem: Read, ModuleAlias: Read, WildcardImport: false}",
				"ImportNode{ModuleName: std, ModuleItem: fmt, ModuleAlias: fmt, WildcardImport: false}",
				"ImportNode{ModuleName: serde_json, ModuleItem: , ModuleAlias: , WildcardImport: false}",
			},
			functions: []string{"main", "helper", "read"},
		},
		{
			language: core.LanguageCodePHP,
			filePath: "fixtures/imports.php",
			imports: []string{
				"ImportNode{ModuleName: OpenAI\\Client, ModuleItem: , ModuleAlias: Client, WildcardImport: false}",
				"ImportNode{ModuleName: GuzzleHttp\\Client, ModuleItem: , ModuleAlias: HttpClient, WildcardImport: false}",
				"ImportNode{ModuleName: Monolog\\Logger, ModuleItem: , ModuleAlias: Logger, WildcardImport: false}",
				"ImportNode{ModuleName: Illuminate\\Support, ModuleItem: Str, ModuleAlias: Str, WildcardImport: false}",
				"ImportNode{ModuleName: Illuminate\\Support, ModuleItem: Facades\\Log, ModuleAlias: Logs, WildcardImport: false}",
				"ImportNode{ModuleName: Laravel\\Prompts\\text, ModuleItem: , ModuleAlias: text, WildcardImport: false}",
			},
			functions: []string{"handle", "index"},
		},
		{
			language: core.LanguageCodeRuby,
			filePath: "fixtures/imports.rb",
			imports: []string{
				"ImportNode{ModuleName: openai, ModuleItem: , ModuleAlias: , WildcardImport: true}",
				"ImportNode{ModuleName: net/http, ModuleItem: , ModuleAlias: , WildcardImport: true}",
				"ImportNode{ModuleName: google/cloud/storage, ModuleItem: , ModuleAlias: , WildcardImport: true}",
			},
			functions: []string{"run", "build"},
		},
		{
			language: core.LanguageCodeKotlin,
			filePath: "fixtures/imports.kt",
			imports: []string{
				"ImportNode{ModuleName: okhttp3.OkHttpClient, ModuleItem: , ModuleAlias: OkHttpClient, WildcardImport: false}",
				"ImportNode{ModuleName: com.openai.client.okhttp.OpenAIOkHttpClient, ModuleItem: , ModuleAlias: OpenAIClient, WildcardImport: false}",
				"ImportNode{ModuleName: kotlinx.coroutines, ModuleItem: , ModuleAlias: , WildcardImport: true}",
			},
			functions: []string{"main", "run"},
		},
	}

	for _, tc := range cases {
		t.Run(string(tc.language), func(t *testing.T) {
			language, err := lang.GetLanguage(string(tc.language))
			require.NoError(t, err)

			fileParser, err := parser.NewParser([]core.Language{language})
			require.NoError(t, err)

			fileSystem, err := fs.NewLocalFileSystem(fs.LocalFileSystemConfig{AppDirectories: []string{tc.filePath}})
			require.NoError(t, err)

			err = fileSystem.EnumerateApp(context.Background(), func(f core.File) error {
				tree, err := fileParser.Parse(context.Background(), f)
				require.NoError(t, err)

				imports, err := language.Resolvers().ResolveImports(tree)
				require.NoError(t, err)

				var found []string
				for _, imp := range imports {
					found = append(found, imp.String())
				}
				assert.Equal(t, tc.imports, found)

				functions, err := language.Resolvers().ResolveFunctions(tree)
				require.NoError(t, err)

				var names []string
				for _, function := range functions {
					names = append(names, function.FunctionName())
				}
				assert.Equal(t, tc.functions, names)
				return nil
			})
			require.NoError(t, err)
		})
	}
}
