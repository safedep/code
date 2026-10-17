package callgraph

import (
	"context"
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/pkg/test"
	"github.com/safedep/code/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShapedLanguageCalls(t *testing.T) {
	cases := []struct {
		language core.LanguageCode
		filePath string
		calls    []string
	}{
		{
			language: core.LanguageCodeCSharp,
			filePath: "testShapes.cs",
			calls: []string{
				"OpenAI//Chat//*",
				"ChatClient",
				"ChatClient//CompleteChatAsync",
				"Environment//GetEnvironmentVariable",
				"System//Net//Http//HttpClient",
				"System//Net//Http//HttpClient//GetStringAsync",
				"Console//WriteLine",
				"testShapes.cs//Agent//Ask",
				"testShapes.cs//Host//Run//services//GetRequiredService",
				"testShapes.cs//Host//Run//chatClient//CompleteChat",
				"ChatOptions",
				"AIFunctionFactory//Create",
			},
		},
		{
			// A file-scoped namespace, as in namespace Demo.App;, is a
			// sibling of the declarations in this grammar. vet walks its
			// children too, so a grammar that nests them keeps the calls.
			language: core.LanguageCodeCSharp,
			filePath: "testFileScoped.cs",
			calls: []string{
				"OpenAI//Chat//*",
				"testFileScoped.cs//Agent//Ask",
				"ChatClient",
				"ChatClient//CompleteChat",
			},
		},
		{
			language: core.LanguageCodeRust,
			filePath: "testShapes.rs",
			calls: []string{
				"async_openai//Client//new",
				"async_openai//Client//chat//create",
				"async_openai//types//CreateChatCompletionRequestArgs//default",
				"async_openai//types//CreateChatCompletionRequestArgs//model//build",
				"reqwest//Client//new",
				"reqwest//Client//get//send",
				"serde_json//to_string",
				"candle_core//Tensor//arange//unsqueeze",
				"async_openai//Client//models//list//data//iter//map//collect",
				"println",
				"testShapes.rs//main",
			},
		},
		{
			language: core.LanguageCodePHP,
			filePath: "testShapes.php",
			calls: []string{
				"OpenAI//client",
				"OpenAI//client//chat//create",
				"GuzzleHttp//Client",
				"GuzzleHttp//Client//get",
				"Yethee//Tiktoken//EncoderProvider//getForModel",
				"GuzzleHttp//Psr7//Request",
				"getenv",
				"strtoupper",
				"testShapes.php//Assistant//ask",
			},
		},
		{
			language: core.LanguageCodeRuby,
			filePath: "testShapes.rb",
			calls: []string{
				"openai//*",
				"net/http//*",
				"OpenAI//Client//new",
				"OpenAI//Client//chat",
				"Net//HTTP//get",
				"URI",
				"ENV//fetch",
				"testShapes.rb//Assistant//initialize",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.filePath, func(t *testing.T) {
			treeWalker, fileSystem, err := test.SetupBasicPluginContext("fixtures", []string{tc.filePath}, []core.LanguageCode{tc.language})
			require.NoError(t, err)

			var calls []string
			callback := func(_ context.Context, cg *CallGraph) error {
				for _, item := range cg.DFS() {
					calls = append(calls, item.Namespace)
				}
				return nil
			}

			executor, err := plugin.NewTreeWalkPluginExecutor(treeWalker, []core.Plugin{NewCallGraphPlugin(callback)})
			require.NoError(t, err)
			report, err := executor.Execute(context.Background(), fileSystem)
			require.NoError(t, err)
			assert.Empty(t, report.Failed)

			assert.Subset(t, calls, tc.calls)
			for _, c := range calls {
				assert.NotRegexp(t, `[\s()|{}]`, c, "a namespace holds no source text")
			}
		})
	}
}

func TestWithoutTypeArguments(t *testing.T) {
	for in, want := range map[string]string{
		"List<string>":                    "List",
		"serde_json::from_str::<Value>":   "serde_json::from_str::",
		"Hmac::<Sha256>::new_from_slice":  "Hmac::::new_from_slice",
		"Dictionary<string, List<int>>.X": "Dictionary.X",
		"GetRequiredService<IChat>":       "GetRequiredService",
		"plain::path":                     "plain::path",
	} {
		assert.Equal(t, want, withoutTypeArguments(in), in)
	}
}
