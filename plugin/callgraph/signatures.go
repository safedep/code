package callgraph

import (
	_ "embed"
	"fmt"

	callgraphv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/code/callgraph/v1"
	"buf.build/go/protovalidate"
	"github.com/safedep/code/core"
	"github.com/safedep/dry/ds/trie"
	"github.com/safedep/dry/log"
)

const (
	MatchAny = "any"
	MatchAll = "all"
)

type MatchedCondition struct {
	Condition *callgraphv1.Signature_LanguageMatcher_SignatureCondition
	Evidences []*CallGraphNode
}

type SignatureMatchResult struct {
	MatchedSignature    *callgraphv1.Signature
	MatchedLanguageCode core.LanguageCode
	MatchedConditions   []MatchedCondition
}

type SignatureMatcher struct {
	targetSignatures []*callgraphv1.Signature
}

// Creates a new SignatureMatcher instance with the provided target signatures.
// It validates the signatures using the ValidateSignatures function.
// If the validation fails, it returns an error.
func NewSignatureMatcher(targetSignatures []*callgraphv1.Signature) (*SignatureMatcher, error) {
	validationErr := ValidateSignatures(targetSignatures)
	if validationErr != nil {
		return nil, fmt.Errorf("failed to validate signatures: %w", validationErr)
	}

	return &SignatureMatcher{
		targetSignatures: targetSignatures,
	}, nil
}

func (sm *SignatureMatcher) MatchSignatures(cg *CallGraph) ([]SignatureMatchResult, error) {
	language, err := cg.Tree.Language()
	if err != nil {
		log.Errorf("failed to get language from parse tree: %v", err)
		return nil, err
	}

	languageCode := language.Meta().Code

	matcherResults := []SignatureMatchResult{}

	functionCallTrie := trie.NewTrie[CallGraphNode]()
	functionCallResultItems := cg.DFS()
	for _, resultItem := range functionCallResultItems {
		// We record the caller node in the trie for every namespace,
		// since the caller is evidence of that namespace's usage
		functionCallTrie.Insert(resultItem.Namespace, resultItem.Caller)
	}

	for _, signature := range sm.targetSignatures {
		languageSignature, exists := signature.Languages[string(languageCode)]
		if !exists {
			continue
		}

		matchedConditions := []MatchedCondition{}
		for _, condition := range languageSignature.Conditions {
			if condition.Type == "call" {
				matchCondition := MatchedCondition{
					Condition: condition,
					Evidences: []*CallGraphNode{},
				}

				lookupNamespace := resolveNamespaceWithSeparator(condition.Value, language)
				lookupEntries := functionCallTrie.WordsWithPrefix(lookupNamespace)
				for _, lookupEntry := range lookupEntries {
					matchCondition.Evidences = append(matchCondition.Evidences, lookupEntry.Value)
				}

				if len(matchCondition.Evidences) > 0 {
					matchedConditions = append(matchedConditions, matchCondition)
				}
			}
		}

		if (languageSignature.Match == MatchAny && len(matchedConditions) > 0) || (languageSignature.Match == MatchAll && len(matchedConditions) == len(languageSignature.Conditions)) {
			matcherResults = append(matcherResults, SignatureMatchResult{
				MatchedSignature:    signature,
				MatchedLanguageCode: languageCode,
				MatchedConditions:   matchedConditions,
			})
		}
	}
	return matcherResults, nil
}

// Validates list of callgraphv1.Signature based on protovalidate specification
func ValidateSignatures(signatures []*callgraphv1.Signature) error {
	v, err := protovalidate.New()
	if err != nil {
		return err
	}

	for i := range signatures {
		if signatures[i] == nil {
			return fmt.Errorf("signature %d is nil", i)
		}

		signature := &signatures[i]
		if err := v.Validate(*signature); err != nil {
			return err
		}
	}

	log.Infof("Successfully validated %d signatures", len(signatures))

	return nil
}
