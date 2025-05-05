package signatures

import (
	"testing"

	callgraphv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/code/callgraph/v1"
	"github.com/stretchr/testify/assert"
)

func TestValidateSignatures(t *testing.T) {
	err := validateSignatures(ParsedSignatures)
	assert.NoError(t, err, "Expected no error during validation of default valid signatures")

	// Add invalid signatures for testing
	invalidSignatures := []callgraphv1.Signature{
		{
			Id: "invalid.match",
			Languages: map[string]*callgraphv1.Signature_LanguageMatcher{
				"python": {
					Match:      "invalid_match_type",
					Conditions: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition{},
				},
			},
		},
		{
			Id: "invalid.language",
			Languages: map[string]*callgraphv1.Signature_LanguageMatcher{
				"invalid_language": {
					Match:      "any",
					Conditions: []*callgraphv1.Signature_LanguageMatcher_SignatureCondition{},
				},
			},
		},
	}

	for i := range invalidSignatures {
		invalidSignature := &invalidSignatures[i]
		err := validateSignatures([]*callgraphv1.Signature{invalidSignature})
		assert.Error(t, err, "Expected error during validation of invalid signature")
	}
}
