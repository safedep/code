package callgraph

import (
	"context"
	"testing"

	callgraphv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/code/callgraph/v1"
	"github.com/safedep/code/core"
	"github.com/safedep/code/pkg/test"
	"github.com/safedep/code/plugin"
	"github.com/stretchr/testify/assert"
)

func TestValidateSignatures(t *testing.T) {
	signatureValidationTestCases := []struct {
		signature     *callgraphv1.Signature
		expectedError bool
	}{
		{
			signature: &callgraphv1.Signature{
				Id: "valid.signature",
				Languages: map[string]*callgraphv1.Signature_LanguageMatcher{
					"python": {
						Match:      "any",
						Conditions: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition{},
					},
				},
			},
			expectedError: false,
		},
		{
			signature: &callgraphv1.Signature{
				Id: "invalid.match",
				Languages: map[string]*callgraphv1.Signature_LanguageMatcher{
					"python": {
						Match:      "invalid_match_type",
						Conditions: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition{},
					},
				},
			},
			expectedError: true,
		},
		{
			signature: &callgraphv1.Signature{
				Id: "invalid.language",
				Languages: map[string]*callgraphv1.Signature_LanguageMatcher{
					"invalid_language": {
						Match:      "any",
						Conditions: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition{},
					},
				},
			},
			expectedError: true,
		},
		{
			signature:     callSignature("ruby.language", core.LanguageCodeRuby, MatchAny, "OpenAI::Client.new"),
			expectedError: false,
		},
		{
			signature:     callSignature("kotlin.language", core.LanguageCodeKotlin, MatchAny, "okhttp3.OkHttpClient"),
			expectedError: true,
		},
		{
			signature:     callSignature("csharp.invalid.match", core.LanguageCodeCSharp, "invalid_match_type", "OpenAI.*"),
			expectedError: true,
		},
	}

	for _, tc := range signatureValidationTestCases {
		err := ValidateSignatures([]*callgraphv1.Signature{tc.signature})
		if tc.expectedError {
			assert.Error(t, err, "Expected error during validation of invalid signature")
		} else {
			assert.NoError(t, err, "Expected no error during validation of valid signature")
		}
	}
}

// signatureMatchExpectation defines an expected signature match result
type signatureMatchExpectation struct {
	SignatureID      string
	ShouldMatch      bool
	ExpectedLanguage core.LanguageCode
	MinEvidenceCount int
	CalleeContains   string // Optional: substring to verify in callee namespace
}

// signatureMatcherTestCase defines a test case for signature matching
type signatureMatcherTestCase struct {
	Name            string
	Language        core.LanguageCode
	FilePaths       []string
	Signatures      []*callgraphv1.Signature
	ExpectedMatches []signatureMatchExpectation
}

func TestSignatureMatcher(t *testing.T) {
	testCases := []signatureMatcherTestCase{
		{
			Name:      "JavaScript signatures",
			Language:  core.LanguageCodeJavascript,
			FilePaths: []string{"fixtures/testJavascript.js"},
			Signatures: []*callgraphv1.Signature{
				{
					Id: "js.console.log.usage",
					Languages: map[string]*callgraphv1.Signature_LanguageMatcher{
						"javascript": {
							Match: "any",
							Conditions: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition{
								{
									Type:  "call",
									Value: "console/log",
								},
							},
						},
					},
				},
				{
					Id: "js.filesystem.access",
					Languages: map[string]*callgraphv1.Signature_LanguageMatcher{
						"javascript": {
							Match: "any",
							Conditions: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition{
								{
									Type:  "call",
									Value: "fs/readFileSync",
								},
							},
						},
					},
				},
				{
					Id: "js.http.request",
					Languages: map[string]*callgraphv1.Signature_LanguageMatcher{
						"javascript": {
							Match: "any",
							Conditions: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition{
								{
									Type:  "call",
									Value: "axios/get",
								},
							},
						},
					},
				},
				{
					Id: "js.database.constructor",
					Languages: map[string]*callgraphv1.Signature_LanguageMatcher{
						"javascript": {
							Match: "any",
							Conditions: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition{
								{
									Type:  "call",
									Value: "sqlite3/Database",
								},
							},
						},
					},
				},
			},
			ExpectedMatches: []signatureMatchExpectation{
				{
					SignatureID:      "js.console.log.usage",
					ShouldMatch:      true,
					ExpectedLanguage: core.LanguageCodeJavascript,
					MinEvidenceCount: 1,
					CalleeContains:   "log",
				},
				{
					SignatureID:      "js.filesystem.access",
					ShouldMatch:      true,
					ExpectedLanguage: core.LanguageCodeJavascript,
					MinEvidenceCount: 1,
					CalleeContains:   "readFileSync",
				},
				{
					SignatureID:      "js.http.request",
					ShouldMatch:      true,
					ExpectedLanguage: core.LanguageCodeJavascript,
					MinEvidenceCount: 1,
					CalleeContains:   "get",
				},
				{
					SignatureID:      "js.database.constructor",
					ShouldMatch:      true,
					ExpectedLanguage: core.LanguageCodeJavascript,
					MinEvidenceCount: 1,
					CalleeContains:   "Database",
				},
			},
		},
		{
			Name:      "JavaScript CommonJS signatures",
			Language:  core.LanguageCodeJavascript,
			FilePaths: []string{"fixtures/testCommonJS.js"},
			Signatures: []*callgraphv1.Signature{
				callSignature("anthropic.client", core.LanguageCodeJavascript, MatchAny, "@anthropic-ai/sdk"),
				callSignature("node.events.emitter", core.LanguageCodeJavascript, MatchAny, "events/EventEmitter"),
				callSignature("debug.logger", core.LanguageCodeJavascript, MatchAny, "debug"),
			},
			ExpectedMatches: []signatureMatchExpectation{
				{SignatureID: "anthropic.client", ShouldMatch: true, ExpectedLanguage: core.LanguageCodeJavascript, MinEvidenceCount: 1},
				{SignatureID: "node.events.emitter", ShouldMatch: true, ExpectedLanguage: core.LanguageCodeJavascript, MinEvidenceCount: 1},
				{SignatureID: "debug.logger", ShouldMatch: true, ExpectedLanguage: core.LanguageCodeJavascript, MinEvidenceCount: 1},
			},
		},
		{
			Name:      "C# signatures",
			Language:  core.LanguageCodeCSharp,
			FilePaths: []string{"fixtures/testShapes.cs"},
			Signatures: []*callgraphv1.Signature{
				callSignature("openai.chat", core.LanguageCodeCSharp, MatchAll, "OpenAI.Chat.*", "ChatClient.CompleteChatAsync"),
				callSignature("http.client", core.LanguageCodeCSharp, MatchAny, "System.Net.Http.HttpClient.*"),
				callSignature("anthropic.client", core.LanguageCodeCSharp, MatchAll, "Anthropic.*", "AnthropicClient"),
			},
			ExpectedMatches: []signatureMatchExpectation{
				{SignatureID: "openai.chat", ShouldMatch: true, ExpectedLanguage: core.LanguageCodeCSharp, MinEvidenceCount: 1},
				{SignatureID: "http.client", ShouldMatch: true, ExpectedLanguage: core.LanguageCodeCSharp, MinEvidenceCount: 1},
				{SignatureID: "anthropic.client", ShouldMatch: false},
			},
		},
		{
			Name:      "Rust signatures",
			Language:  core.LanguageCodeRust,
			FilePaths: []string{"fixtures/testShapes.rs"},
			Signatures: []*callgraphv1.Signature{
				callSignature("openai.client", core.LanguageCodeRust, MatchAny, "async_openai::Client::new"),
				callSignature("http.client", core.LanguageCodeRust, MatchAny, "reqwest::*"),
				callSignature("anthropic.client", core.LanguageCodeRust, MatchAny, "anthropic::Client::new"),
			},
			ExpectedMatches: []signatureMatchExpectation{
				{SignatureID: "openai.client", ShouldMatch: true, ExpectedLanguage: core.LanguageCodeRust, MinEvidenceCount: 1},
				{SignatureID: "http.client", ShouldMatch: true, ExpectedLanguage: core.LanguageCodeRust, MinEvidenceCount: 1},
				{SignatureID: "anthropic.client", ShouldMatch: false},
			},
		},
		{
			Name:      "PHP signatures",
			Language:  core.LanguageCodePHP,
			FilePaths: []string{"fixtures/testShapes.php"},
			Signatures: []*callgraphv1.Signature{
				callSignature("openai.client", core.LanguageCodePHP, MatchAny, "OpenAI::client"),
				callSignature("http.client", core.LanguageCodePHP, MatchAny, "GuzzleHttp\\Client"),
				callSignature("anthropic.client", core.LanguageCodePHP, MatchAny, "Anthropic::client"),
			},
			ExpectedMatches: []signatureMatchExpectation{
				{SignatureID: "openai.client", ShouldMatch: true, ExpectedLanguage: core.LanguageCodePHP, MinEvidenceCount: 1},
				{SignatureID: "http.client", ShouldMatch: true, ExpectedLanguage: core.LanguageCodePHP, MinEvidenceCount: 1},
				{SignatureID: "anthropic.client", ShouldMatch: false},
			},
		},
		{
			Name:      "Ruby signatures",
			Language:  core.LanguageCodeRuby,
			FilePaths: []string{"fixtures/testShapes.rb"},
			Signatures: []*callgraphv1.Signature{
				callSignature("openai.client", core.LanguageCodeRuby, MatchAny, "OpenAI::Client.new"),
				callSignature("http.client", core.LanguageCodeRuby, MatchAny, "Net::HTTP.*"),
				callSignature("anthropic.client", core.LanguageCodeRuby, MatchAny, "Anthropic::Client.new"),
			},
			ExpectedMatches: []signatureMatchExpectation{
				{SignatureID: "openai.client", ShouldMatch: true, ExpectedLanguage: core.LanguageCodeRuby, MinEvidenceCount: 2},
				{SignatureID: "http.client", ShouldMatch: true, ExpectedLanguage: core.LanguageCodeRuby, MinEvidenceCount: 1},
				{SignatureID: "anthropic.client", ShouldMatch: false},
			},
		},
		{
			Name:      "Python signatures",
			Language:  core.LanguageCodePython,
			FilePaths: []string{"fixtures/testFunctions.py"},
			Signatures: []*callgraphv1.Signature{
				{
					Id: "python.print.usage",
					Languages: map[string]*callgraphv1.Signature_LanguageMatcher{
						"python": {
							Match: "any",
							Conditions: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition{
								{
									Type:  "call",
									Value: "print",
								},
							},
						},
					},
				},
				{
					Id: "python.pprint.pprint.usage",
					Languages: map[string]*callgraphv1.Signature_LanguageMatcher{
						"python": {
							Match: "any",
							Conditions: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition{
								{
									Type:  "call",
									Value: "pprint.pprint",
								},
							},
						},
					},
				},
				{
					Id: "python.os.getenv.usage",
					Languages: map[string]*callgraphv1.Signature_LanguageMatcher{
						"python": {
							Match: "any",
							Conditions: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition{
								{
									Type:  "call",
									Value: "os.getenv",
								},
							},
						},
					},
				},
				{
					Id: "python.pstats.getsomestat.usage",
					Languages: map[string]*callgraphv1.Signature_LanguageMatcher{
						"python": {
							Match: "any",
							Conditions: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition{
								{
									Type:  "call",
									Value: "pstats.getsomestat",
								},
							},
						},
					},
				},
			},
			ExpectedMatches: []signatureMatchExpectation{
				{
					SignatureID:      "python.print.usage",
					ShouldMatch:      true,
					ExpectedLanguage: core.LanguageCodePython,
					MinEvidenceCount: 1,
					CalleeContains:   "print",
				},
				{
					SignatureID:      "python.pprint.pprint.usage",
					ShouldMatch:      true,
					ExpectedLanguage: core.LanguageCodePython,
					MinEvidenceCount: 1,
					CalleeContains:   "pprint",
				},
				{
					SignatureID:      "python.os.getenv.usage",
					ShouldMatch:      true,
					ExpectedLanguage: core.LanguageCodePython,
					MinEvidenceCount: 1,
					CalleeContains:   "getenv",
				},
				{
					SignatureID:      "python.pstats.getsomestat.usage",
					ShouldMatch:      true,
					ExpectedLanguage: core.LanguageCodePython,
					MinEvidenceCount: 1,
					CalleeContains:   "getsomestat",
				},
			},
		},
		{
			Name:      "Go signatures",
			Language:  core.LanguageCodeGo,
			FilePaths: []string{"fixtures/testCallGraph.go"},
			Signatures: []*callgraphv1.Signature{
				{
					Id: "go.fmt.println.usage",
					Languages: map[string]*callgraphv1.Signature_LanguageMatcher{
						"go": {
							Match: "any",
							Conditions: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition{
								{
									Type:  "call",
									Value: "fmt/Println",
								},
							},
						},
					},
				},
				{
					Id: "go.os.writefile.usage",
					Languages: map[string]*callgraphv1.Signature_LanguageMatcher{
						"go": {
							Match: "any",
							Conditions: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition{
								{
									Type:  "call",
									Value: "os/WriteFile",
								},
							},
						},
					},
				},
				{
					Id: "go.os.getenv.usage",
					Languages: map[string]*callgraphv1.Signature_LanguageMatcher{
						"go": {
							Match: "any",
							Conditions: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition{
								{
									Type:  "call",
									Value: "os/Getenv",
								},
							},
						},
					},
				},
				{
					Id: "go.fmt.sprintf.usage",
					Languages: map[string]*callgraphv1.Signature_LanguageMatcher{
						"go": {
							Match: "any",
							Conditions: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition{
								{
									Type:  "call",
									Value: "fmt/Sprintf",
								},
							},
						},
					},
				},
			},
			ExpectedMatches: []signatureMatchExpectation{
				{
					SignatureID:      "go.fmt.println.usage",
					ShouldMatch:      true,
					ExpectedLanguage: core.LanguageCodeGo,
					MinEvidenceCount: 1,
					CalleeContains:   "Println",
				},
				{
					SignatureID:      "go.os.writefile.usage",
					ShouldMatch:      true,
					ExpectedLanguage: core.LanguageCodeGo,
					MinEvidenceCount: 1,
					CalleeContains:   "WriteFile",
				},
				{
					SignatureID:      "go.os.getenv.usage",
					ShouldMatch:      true,
					ExpectedLanguage: core.LanguageCodeGo,
					MinEvidenceCount: 1,
					CalleeContains:   "Getenv",
				},
				{
					SignatureID:      "go.fmt.sprintf.usage",
					ShouldMatch:      true,
					ExpectedLanguage: core.LanguageCodeGo,
					MinEvidenceCount: 1,
					CalleeContains:   "Sprintf",
				},
			},
		},
		{
			Name:      "Java signatures",
			Language:  core.LanguageCodeJava,
			FilePaths: []string{"fixtures/CallgraphTestcases.java"},
			Signatures: []*callgraphv1.Signature{
				{
					Id: "java.system.println.usage",
					Languages: map[string]*callgraphv1.Signature_LanguageMatcher{
						"java": {
							Match: "any",
							Conditions: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition{
								{
									Type:  "call",
									Value: "System.out.println",
								},
							},
						},
					},
				},
				{
					Id: "java.math.random.usage",
					Languages: map[string]*callgraphv1.Signature_LanguageMatcher{
						"java": {
							Match: "any",
							Conditions: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition{
								{
									Type:  "call",
									Value: "Math.random",
								},
							},
						},
					},
				},
				{
					Id: "java.awt.dialog.constructor",
					Languages: map[string]*callgraphv1.Signature_LanguageMatcher{
						"java": {
							Match: "any",
							Conditions: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition{
								{
									Type:  "call",
									Value: "java.awt.Dialog",
								},
							},
						},
					},
				},
				{
					Id: "java.string.valueof.usage",
					Languages: map[string]*callgraphv1.Signature_LanguageMatcher{
						"java": {
							Match: "any",
							Conditions: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition{
								{
									Type:  "call",
									Value: "String.valueOf",
								},
							},
						},
					},
				},
			},
			ExpectedMatches: []signatureMatchExpectation{
				{
					SignatureID:      "java.system.println.usage",
					ShouldMatch:      true,
					ExpectedLanguage: core.LanguageCodeJava,
					MinEvidenceCount: 1,
					CalleeContains:   "println",
				},
				{
					SignatureID:      "java.math.random.usage",
					ShouldMatch:      true,
					ExpectedLanguage: core.LanguageCodeJava,
					MinEvidenceCount: 1,
					CalleeContains:   "random",
				},
				{
					SignatureID:      "java.awt.dialog.constructor",
					ShouldMatch:      true,
					ExpectedLanguage: core.LanguageCodeJava,
					MinEvidenceCount: 1,
					CalleeContains:   "Dialog",
				},
				{
					SignatureID:      "java.string.valueof.usage",
					ShouldMatch:      true,
					ExpectedLanguage: core.LanguageCodeJava,
					MinEvidenceCount: 1,
					CalleeContains:   "valueOf",
				},
			},
		},
		{
			Name:      "Java argument matching by index",
			Language:  core.LanguageCodeJava,
			FilePaths: []string{"fixtures/testJavaArguments.java"},
			Signatures: []*callgraphv1.Signature{
				{
					Id: "java.crypto.md5.literal",
					Languages: map[string]*callgraphv1.Signature_LanguageMatcher{
						"java": {
							Match: "any",
							Conditions: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition{
								{
									Type:  "call",
									Value: "java.security.MessageDigest.getInstance",
									Args: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition_Argument{
										{
											Index:  0,
											Values: []string{"\"MD5\"", "\"md5\""},
										},
									},
								},
							},
						},
					},
				},
				{
					Id: "java.crypto.sha256.literal",
					Languages: map[string]*callgraphv1.Signature_LanguageMatcher{
						"java": {
							Match: "any",
							Conditions: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition{
								{
									Type:  "call",
									Value: "java.security.MessageDigest.getInstance",
									Args: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition_Argument{
										{
											Index:  0,
											Values: []string{"\"SHA-256\""},
										},
									},
								},
							},
						},
					},
				},
				{
					Id: "java.awt.canvas.setsize.32.99",
					Languages: map[string]*callgraphv1.Signature_LanguageMatcher{
						"java": {
							Match: "any",
							Conditions: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition{
								{
									Type:  "call",
									Value: "java.awt.Canvas.setSize",
									Args: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition_Argument{
										{
											Index:  0,
											Values: []string{"32"},
										},
										{
											Index:  1,
											Values: []string{"99"},
										},
									},
								},
							},
						},
					},
				},
				{
					Id: "java.awt.dialog.with.window.type",
					Languages: map[string]*callgraphv1.Signature_LanguageMatcher{
						"java": {
							Match: "any",
							Conditions: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition{
								{
									Type:  "call",
									Value: "java.awt.Dialog",
									Args: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition_Argument{
										{
											Index:      0,
											ResolvesTo: []string{"java.awt.Window"},
										},
									},
								},
							},
						},
					},
				},
				{
					Id: "java.awt.canvas.setsize.32.200.should.not.match",
					Languages: map[string]*callgraphv1.Signature_LanguageMatcher{
						"java": {
							Match: "any",
							Conditions: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition{
								{
									Type:  "call",
									Value: "java.awt.Canvas.setSize",
									Args: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition_Argument{
										{
											Index:  0,
											Values: []string{"32"},
										},
										{
											Index:  1,
											Values: []string{"200"},
										},
									},
								},
							},
						},
					},
				},
			},
			ExpectedMatches: []signatureMatchExpectation{
				{
					SignatureID:      "java.crypto.md5.literal",
					ShouldMatch:      true,
					ExpectedLanguage: core.LanguageCodeJava,
					MinEvidenceCount: 1,
					CalleeContains:   "MessageDigest",
				},
				{
					SignatureID:      "java.crypto.sha256.literal",
					ShouldMatch:      true,
					ExpectedLanguage: core.LanguageCodeJava,
					MinEvidenceCount: 1,
					CalleeContains:   "MessageDigest",
				},
				{
					SignatureID:      "java.awt.canvas.setsize.32.99",
					ShouldMatch:      true,
					ExpectedLanguage: core.LanguageCodeJava,
					MinEvidenceCount: 1,
					CalleeContains:   "setSize",
				},
				{
					SignatureID:      "java.awt.dialog.with.window.type",
					ShouldMatch:      true,
					ExpectedLanguage: core.LanguageCodeJava,
					MinEvidenceCount: 1,
					CalleeContains:   "Dialog",
				},
				{
					SignatureID:      "java.awt.canvas.setsize.32.200.should.not.match",
					ShouldMatch:      false,
					ExpectedLanguage: core.LanguageCodeJava,
					MinEvidenceCount: 0,
					CalleeContains:   "",
				},
			},
		},
		{
			Name:      "TypeScript falls back to JavaScript signatures",
			Language:  core.LanguageCodeTypescript,
			FilePaths: []string{"fixtures/testTypescript.ts"},
			Signatures: []*callgraphv1.Signature{
				// JS-only signature: should match TS via fallback
				{
					Id: "js.console.log.usage",
					Languages: map[string]*callgraphv1.Signature_LanguageMatcher{
						"javascript": {
							Match: "any",
							Conditions: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition{
								{
									Type:  "call",
									Value: "console/log",
								},
							},
						},
					},
				},
				// JS-only signature with argument matching
				{
					Id: "js.crypto.createhash.sha256",
					Languages: map[string]*callgraphv1.Signature_LanguageMatcher{
						"javascript": {
							Match: "any",
							Conditions: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition{
								{
									Type:  "call",
									Value: "crypto/createHash",
									Args: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition_Argument{
										{
											Index:  0,
											Values: []string{"\"sha256\""},
										},
									},
								},
							},
						},
					},
				},
				// Signature with explicit typescript key: should use TS matcher, not JS fallback
				{
					Id: "ts.axios.get.usage",
					Languages: map[string]*callgraphv1.Signature_LanguageMatcher{
						"typescript": {
							Match: "any",
							Conditions: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition{
								{
									Type:  "call",
									Value: "axios/get",
								},
							},
						},
						"javascript": {
							Match: "any",
							Conditions: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition{
								{
									// Different condition to prove TS key is preferred
									Type:  "call",
									Value: "nonexistent/function",
								},
							},
						},
					},
				},
				// When typescript key exists but doesn't match, should NOT fall back to javascript
				{
					Id: "ts.no.fallback.when.ts.key.exists",
					Languages: map[string]*callgraphv1.Signature_LanguageMatcher{
						"typescript": {
							Match: "any",
							Conditions: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition{
								{
									Type:  "call",
									Value: "nonexistent/tsFunction",
								},
							},
						},
						"javascript": {
							Match: "any",
							Conditions: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition{
								{
									// This would match, but should not be used since typescript key exists
									Type:  "call",
									Value: "console/log",
								},
							},
						},
					},
				},
				// Python-only signature: should NOT match TS
				{
					Id: "python.only.signature",
					Languages: map[string]*callgraphv1.Signature_LanguageMatcher{
						"python": {
							Match: "any",
							Conditions: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition{
								{
									Type:  "call",
									Value: "print",
								},
							},
						},
					},
				},
			},
			ExpectedMatches: []signatureMatchExpectation{
				{
					SignatureID:      "js.console.log.usage",
					ShouldMatch:      true,
					ExpectedLanguage: core.LanguageCodeTypescript,
					MinEvidenceCount: 1,
					CalleeContains:   "log",
				},
				{
					SignatureID:      "js.crypto.createhash.sha256",
					ShouldMatch:      true,
					ExpectedLanguage: core.LanguageCodeTypescript,
					MinEvidenceCount: 1,
					CalleeContains:   "createHash",
				},
				{
					SignatureID:      "ts.axios.get.usage",
					ShouldMatch:      true,
					ExpectedLanguage: core.LanguageCodeTypescript,
					MinEvidenceCount: 1,
					CalleeContains:   "get",
				},
				{
					// TS key exists with non-matching condition, JS key has matching condition
					// Should NOT match because TS key takes priority and its condition doesn't match
					SignatureID:      "ts.no.fallback.when.ts.key.exists",
					ShouldMatch:      false,
					ExpectedLanguage: core.LanguageCodeTypescript,
					MinEvidenceCount: 0,
					CalleeContains:   "",
				},
				{
					SignatureID:      "python.only.signature",
					ShouldMatch:      false,
					ExpectedLanguage: core.LanguageCodePython,
					MinEvidenceCount: 0,
					CalleeContains:   "",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			// Create signature matcher
			matcher, err := NewSignatureMatcher(tc.Signatures)
			assert.NoError(t, err, "Failed to create signature matcher")
			assert.NotNil(t, matcher, "Expected matcher to be non-nil")

			// Setup test context
			treeWalker, fileSystem, err := test.SetupBasicPluginContext(tc.FilePaths, []core.LanguageCode{tc.Language})
			assert.NoError(t, err, "Failed to setup plugin context")

			// Collect callgraphs
			var capturedCallgraph *CallGraph
			callgraphCallback := func(ctx context.Context, cg *CallGraph) error {
				capturedCallgraph = cg
				return nil
			}

			// Execute plugin
			pluginExecutor, err := plugin.NewTreeWalkPluginExecutor(treeWalker, []core.Plugin{
				NewCallGraphPlugin(callgraphCallback),
			})
			assert.NoError(t, err, "Failed to create plugin executor")

			err = pluginExecutor.Execute(context.Background(), fileSystem)
			assert.NoError(t, err, "Failed to execute plugin")

			// Verify we captured a callgraph
			assert.NotNil(t, capturedCallgraph, "Expected to capture a callgraph")

			// Run signature matching
			matchResults, err := matcher.MatchSignatures(capturedCallgraph)
			assert.NoError(t, err, "Failed to match signatures")

			// Create a map for easier assertion
			matchedSignatureIds := make(map[string]SignatureMatchResult)
			for _, result := range matchResults {
				matchedSignatureIds[result.MatchedSignature.Id] = result
			}

			// Verify expected matches
			for _, expectation := range tc.ExpectedMatches {
				t.Run(expectation.SignatureID, func(t *testing.T) {
					matchResult, found := matchedSignatureIds[expectation.SignatureID]

					if expectation.ShouldMatch {
						assert.True(t, found, "Expected signature %s to match", expectation.SignatureID)
						if !found {
							return
						}

						assert.Equal(t, expectation.ExpectedLanguage, matchResult.MatchedLanguageCode,
							"Expected language code to match")

						assert.NotEmpty(t, matchResult.MatchedConditions, "Expected conditions to match")
						if len(matchResult.MatchedConditions) == 0 {
							return
						}

						totalEvidences := 0
						for _, condition := range matchResult.MatchedConditions {
							totalEvidences += len(condition.Evidences)
						}
						assert.GreaterOrEqual(t, totalEvidences, expectation.MinEvidenceCount,
							"Expected at least %d evidences", expectation.MinEvidenceCount)

						// Verify callee namespace if specified
						if expectation.CalleeContains != "" && totalEvidences > 0 {
							evidence := matchResult.MatchedConditions[0].Evidences[0]
							treeData, err := capturedCallgraph.Tree.Data()
							assert.NoError(t, err)

							metadata := evidence.Metadata(treeData)
							assert.NotEmpty(t, metadata.CallerNamespace, "Expected caller namespace")
							assert.NotEmpty(t, metadata.CalleeNamespace, "Expected callee namespace")
							assert.Contains(t, metadata.CalleeNamespace, expectation.CalleeContains,
								"Expected callee namespace to contain '%s'", expectation.CalleeContains)
						}
					} else {
						assert.False(t, found, "Expected signature %s NOT to match", expectation.SignatureID)
					}
				})
			}
		})
	}
}

// callSignature is a signature of one language with a call condition for each call.
func callSignature(id string, language core.LanguageCode, match string, calls ...string) *callgraphv1.Signature {
	conditions := make([]*callgraphv1.Signature_LanguageMatcher_SignatureCondition, 0, len(calls))
	for _, call := range calls {
		conditions = append(conditions, &callgraphv1.Signature_LanguageMatcher_SignatureCondition{Type: "call", Value: call})
	}

	return &callgraphv1.Signature{
		Id: id,
		Languages: map[string]*callgraphv1.Signature_LanguageMatcher{
			string(language): {Match: match, Conditions: conditions},
		},
	}
}
