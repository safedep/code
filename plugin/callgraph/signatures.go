package callgraph

import (
	_ "embed"
	"fmt"
	"strings"

	callgraphv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/code/callgraph/v1"
	"buf.build/go/protovalidate"
	"github.com/safedep/code/core"
	"github.com/safedep/dry/ds/trie"
	"github.com/safedep/dry/log"
	"github.com/safedep/dry/utils"
	sitter "github.com/smacker/go-tree-sitter"
)

const (
	MatchAny = "any"
	MatchAll = "all"
)

type MatchedEvidence struct {
	Caller           *CallGraphNode
	Callee           *CallGraphNode
	CallerIdentifier *sitter.Node
}

// Note - We're only providing content details for the caller identifier since its
// expected to be at most one line from entire file, but calllerNamespace is an entire
// scope (fn/class/module) which consumes lot of repetetive memory
type EvidenceMetadata struct {
	CallerNamespace string
	CallerMetadata  *TreeNodeMetadata

	CalleeNamespace string
	CalleeMetadata  *TreeNodeMetadata

	// Keyword / statement that caused the match
	CallerIdentifierContent  string
	CallerIdentifierMetadata *TreeNodeMetadata
}

func (evidence *MatchedEvidence) Metadata(treeData *[]byte) EvidenceMetadata {
	result := EvidenceMetadata{}

	if evidence.Caller != nil {
		result.CallerNamespace = evidence.Caller.Namespace
		callerMetadata, exists := evidence.Caller.Metadata()
		if exists {
			result.CallerMetadata = &callerMetadata
		}
	}

	if evidence.Callee != nil {
		result.CalleeNamespace = evidence.Callee.Namespace
		calleeMetadata, exists := evidence.Callee.Metadata()
		if exists {
			result.CalleeMetadata = &calleeMetadata
		}
	}

	if evidence.CallerIdentifier != nil {
		result.CallerIdentifierContent = evidence.CallerIdentifier.Content(*treeData)
		result.CallerIdentifierMetadata = &TreeNodeMetadata{
			StartLine:   evidence.CallerIdentifier.StartPoint().Row,
			EndLine:     evidence.CallerIdentifier.EndPoint().Row,
			StartColumn: evidence.CallerIdentifier.StartPoint().Column,
			EndColumn:   evidence.CallerIdentifier.EndPoint().Column,
		}
	}

	return result
}

type MatchedCondition struct {
	Condition *callgraphv1.Signature_LanguageMatcher_SignatureCondition
	Evidences []MatchedEvidence
}

type SignatureMatchResult struct {
	FilePath            string
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

	functionCallTrie := trie.NewTrie[[]DfsResultItem]()
	functionCallResultItems := cg.DFS()
	for _, resultItem := range functionCallResultItems {
		// We record the caller node in the trie for every namespace,
		// since the caller is evidence of that namespace's usage
		data := "not avl"
		if resultItem.CallerIdentifier != nil {
			tmpCg := newCallGraphNode(resultItem.Namespace, resultItem.CallerIdentifier)
			mtd, avl := tmpCg.Metadata()
			if avl {
				data = fmt.Sprint(mtd)
			}
		}
		fmt.Println("register -", resultItem.Namespace, "->", data)

		existingResultItem, exists := functionCallTrie.GetWord(resultItem.Namespace)
		if !exists {
			existingResultItem = utils.PtrTo(make([]DfsResultItem, 0))
		}

		*existingResultItem = append(*existingResultItem, resultItem)

		functionCallTrie.Insert(resultItem.Namespace, existingResultItem)
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
					Evidences: []MatchedEvidence{},
				}

				lookupNamespace := resolveNamespaceWithSeparator(condition.Value, language)
				lookupNamespace, isWildcardLookup := trimWildcardLookupNamespace(lookupNamespace, language)

				var evidenceDfsResultItems []DfsResultItem

				if isWildcardLookup {
					// Look up any children of the namespace in the trie
					lookupEntries := functionCallTrie.WordsWithPrefix(lookupNamespace + namespaceSeparator)
					evidenceDfsResultItems = []DfsResultItem{}
					for _, lookupEntry := range lookupEntries {
						evidenceDfsResultItems = append(evidenceDfsResultItems, *lookupEntry.Value...)
					}
				} else {
					// Lookup the exact namespace in the trie
					lookupNode, nodeExists := functionCallTrie.GetWord(lookupNamespace)
					if nodeExists && lookupNode != nil {
						evidenceDfsResultItems = *lookupNode
					}
				}

				for _, evidenceResultItem := range evidenceDfsResultItems {
					matchCondition.Evidences = append(matchCondition.Evidences, MatchedEvidence{
						Caller:           evidenceResultItem.Caller,
						Callee:           evidenceResultItem.Node,
						CallerIdentifier: evidenceResultItem.CallerIdentifier,
					})
				}

				if len(matchCondition.Evidences) > 0 {
					matchedConditions = append(matchedConditions, matchCondition)
				}
			}
		}

		if (languageSignature.Match == MatchAny && len(matchedConditions) > 0) || (languageSignature.Match == MatchAll && len(matchedConditions) == len(languageSignature.Conditions)) {
			matcherResults = append(matcherResults, SignatureMatchResult{
				FilePath:            cg.FileName,
				MatchedSignature:    signature,
				MatchedLanguageCode: languageCode,
				MatchedConditions:   matchedConditions,
			})
		}
	}
	return matcherResults, nil
}

// Identifies if the namespace is a wildcard lookup and returns the namespace without the wildcard
// qualifier and a boolean indicating if it was a wildcard lookup.
// Note - We only support wildcard lookup qualifier at the end of the namespace.
// eg. "foo//bar//*" is a valid wildcard lookup namespace, but "foo//*//bar" is not valid
func trimWildcardLookupNamespace(namespace string, language core.Language) (string, bool) {
	if strings.HasSuffix(namespace, "//*") {
		// Remove the wildcard qualifier from the namespace
		return strings.TrimSuffix(namespace, "//*"), true
	}
	return namespace, false
}

// Validates list of callgraphv1.Signature based on protovalidate specification
func ValidateSignatures(signatures []*callgraphv1.Signature) error {
	v, err := protovalidate.New()
	if err != nil {
		return err
	}

	for i, signature := range signatures {
		if signature == nil {
			return fmt.Errorf("signature %d is nil", i)
		}

		if err := v.Validate(signature); err != nil {
			return err
		}
	}

	return nil
}
