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
