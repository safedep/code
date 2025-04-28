package callgraph

import (
	_ "embed"

	"github.com/safedep/code/core"
	"github.com/safedep/dry/ds/trie"
	"github.com/safedep/dry/log"
)

type Signature struct {
	ID          string                                 `yaml:"id"`
	Description string                                 `yaml:"description"`
	Tags        []string                               `yaml:"tags"`
	Languages   map[core.LanguageCode]LanguageMatchers `yaml:"languages"`
}

const (
	MatchAny = "any"
	MatchAll = "all"
)

type LanguageMatchers struct {
	Match      string               `yaml:"match"`
	Conditions []SignatureCondition `yaml:"conditions"`
}

type SignatureCondition struct {
	Type  string `yaml:"type"`  // "call" or "import_module"
	Value string `yaml:"value"` // function or module name
}

type MatchCondition struct {
	Condition SignatureCondition
	Evidences []*CallGraphNode
}

type SignatureMatchResult struct {
	MatchedSignature    *Signature
	MatchedLanguageCode core.LanguageCode
	MatchedConditions   []MatchCondition
}

type SignatureMatcher struct {
	targetSignatures []Signature
}

func NewSignatureMatcher(targetSignatures []Signature) *SignatureMatcher {
	return &SignatureMatcher{
		targetSignatures: targetSignatures,
	}
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
		languageSignature, exists := signature.Languages[languageCode]
		if !exists {
			continue
		}

		matchedConditions := []MatchCondition{}
		for _, condition := range languageSignature.Conditions {
			if condition.Type == "call" {
				matchCondition := MatchCondition{
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
				MatchedSignature:    &signature,
				MatchedLanguageCode: languageCode,
				MatchedConditions:   matchedConditions,
			})
		}
	}
	return matcherResults, nil
}
