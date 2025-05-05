package callgraph

import (
	"testing"

	callgraphv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/code/callgraph/v1"
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
