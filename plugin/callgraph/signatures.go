package callgraph

import (
	_ "embed"
	"strings"

	"github.com/safedep/code/core"
	"github.com/safedep/dry/ds/trie"
	"github.com/safedep/dry/log"
	"gopkg.in/yaml.v3"
)

//go:embed signatures.yaml
var signatureYAML []byte

type SignatureFile struct {
	Version    string      `yaml:"version"`
	Signatures []Signature `yaml:"signatures"`
}

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
	Evidences []*graphNode
}

type SignatureMatchResult struct {
	MatchedSignature    *Signature
	MatchedLanguageCode core.LanguageCode
	MatchedConditions   []MatchCondition
}

var (
	ParsedSignatures   SignatureFile
	signatureByID      map[string]*Signature
	signaturesByPrefix map[string][]*Signature
)

func init() {
	err := yaml.Unmarshal(signatureYAML, &ParsedSignatures)
	if err != nil {
		log.Fatalf("Failed to parse signature YAML: %v", err)
	}

	// Initialize lookup maps
	signatureByID = make(map[string]*Signature)
	signaturesByPrefix = make(map[string][]*Signature)

	for i := range ParsedSignatures.Signatures {
		sig := &ParsedSignatures.Signatures[i]
		signatureByID[sig.ID] = sig

		// build hierarchical prefix map (e.g., "gcp", "gcp.storage", etc.)
		parts := strings.Split(sig.ID, ".")
		for i := 1; i <= len(parts); i++ {
			prefix := strings.Join(parts[:i], ".")
			signaturesByPrefix[prefix] = append(signaturesByPrefix[prefix], sig)
		}
	}
}

func GetSignatureByID(id string) (*Signature, bool) {
	sig, ok := signatureByID[id]
	return sig, ok
}

func GetSignaturesByPrefix(prefix string) []*Signature {
	return signaturesByPrefix[prefix]
}

func MatchSignatures(cg *CallGraph, targetSignatures []Signature) ([]SignatureMatchResult, error) {
	language, err := cg.Tree.Language()
	if err != nil {
		log.Errorf("failed to get language from parse tree: %v", err)
		return nil, err
	}

	languageCode := language.Meta().Code

	matcherResults := []SignatureMatchResult{}

	functionCallTrie := trie.NewTrie[graphNode]()
	functionCallResultItems := cg.DFS()
	for _, resultItem := range functionCallResultItems {
		// We record the caller node in the trie for every namespace,
		// since the caller is evidence of that namespace's usage
		functionCallTrie.Insert(resultItem.Namespace, resultItem.Caller)
	}

	for _, signature := range targetSignatures {
		languageSignature, exists := signature.Languages[languageCode]
		if !exists {
			continue
		}

		matchedConditions := []MatchCondition{}
		for _, condition := range languageSignature.Conditions {
			if condition.Type == "call" {
				matchCondition := MatchCondition{
					Condition: condition,
					Evidences: []*graphNode{},
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
