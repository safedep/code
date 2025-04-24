package callgraph

import (
	_ "embed"
	"strings"

	"github.com/priyakdey/trie"
	"github.com/safedep/code/core"
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
	Match      string      `yaml:"match"`
	Conditions []Condition `yaml:"conditions"`
}

type Condition struct {
	Type  string `yaml:"type"`  // "call" or "import_module"
	Value string `yaml:"value"` // function or module name
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

	functionCallTrie := trie.New()
	functionCallResultItems := cg.DFS()
	for _, resultItem := range functionCallResultItems {
		functionCallTrie.Insert(resultItem.Node.Namespace)
	}

	for _, signature := range targetSignatures {
		languageSignature, exists := signature.Languages[languageCode]
		if !exists {
			continue
		}

		signatureConditionsMet := 0
		for _, condition := range languageSignature.Conditions {
			if condition.Type == "call" {
				lookupNamespace := resolveNamespaceWithSeparator(condition.Value, language)
				// Check if any of the functionCalls starts with the prefix - condition.Value
				// matched := false
				// for _, resultItem := range functionCallResultItems {
				// 	if strings.HasPrefix(resultItem.Namespace, lookupNamespace) {
				// 		matched = true
				// 		break
				// 	}
				// }
				matched := functionCallTrie.Contains(lookupNamespace) || functionCallTrie.ContainsPrefix(lookupNamespace+namespaceSeparator)
				if matched {
					signatureConditionsMet++
				}
			}
		}

		if (languageSignature.Match == MatchAny && signatureConditionsMet > 0) || (languageSignature.Match == MatchAll && signatureConditionsMet == len(languageSignature.Conditions)) {
			matcherResults = append(matcherResults, SignatureMatchResult{
				MatchedSignature:    &signature,
				MatchedLanguageCode: languageCode,
			})
		}
	}
	return matcherResults, nil
}
