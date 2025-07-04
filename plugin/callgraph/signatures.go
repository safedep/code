package callgraph

import (
	_ "embed"
	"fmt"
	"slices"
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
	Arguments        []CallArgument
}

// Note - We're only providing content details for the caller identifier since its
// expected to be at most one line from the entire file, but callerNamespace is an entire
// scope (fn/class/module) which consumes a lot of repetitive memory
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

				var dfsResultItemsWithMatchedFunctionName []DfsResultItem

				if isWildcardLookup {
					// Look up any children of the namespace in the trie
					lookupEntries := functionCallTrie.WordsWithPrefix(lookupNamespace + namespaceSeparator)
					dfsResultItemsWithMatchedFunctionName = []DfsResultItem{}
					for _, lookupEntry := range lookupEntries {
						dfsResultItemsWithMatchedFunctionName = append(dfsResultItemsWithMatchedFunctionName, *lookupEntry.Value...)
					}
				} else {
					// Lookup the exact namespace in the trie
					lookupNode, nodeExists := functionCallTrie.GetWord(lookupNamespace)
					if nodeExists && lookupNode != nil {
						dfsResultItemsWithMatchedFunctionName = *lookupNode
					}
				}

				for _, evidenceResultItem := range dfsResultItemsWithMatchedFunctionName {
					if matchesArgumentConstraints(evidenceResultItem, condition.Args, language) {
						// If the arguments match the required constraints, we can add this evidence
						matchCondition.Evidences = append(matchCondition.Evidences, MatchedEvidence{
							Caller:           evidenceResultItem.Caller,
							Callee:           evidenceResultItem.Node,
							CallerIdentifier: evidenceResultItem.CallerIdentifier,
							Arguments:        evidenceResultItem.Arguments,
						})
					} else {
						// Skip this evidence if it doesn't match the argument constraints
						continue
					}
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

// matchesArgumentConstraints checks if the arguments in the DFS result item
// match the required argument constraints specified in the signature condition.
// It returns true if "all" required arguments match their respective constraints,
func matchesArgumentConstraints(
	dfsResultItem DfsResultItem,
	requiredArgs []*callgraphv1.Signature_LanguageMatcher_SignatureCondition_Argument,
	language core.Language,
) bool {
	if len(requiredArgs) == 0 {
		return true // No conditions to match, so it matches by default
	}

	callArguments := dfsResultItem.Arguments

	// Strictly all arguments must match their respective constraints
	for _, requiredArg := range requiredArgs {
		requiredArgResolvesToNamespaces := make([]string, len(requiredArg.ResolvesTo))
		for i, resolvesTo := range requiredArg.ResolvesTo {
			requiredArgResolvesToNamespaces[i] = resolveNamespaceWithSeparator(resolvesTo, language)
		}

		// No values or resolves_to specified, so no constraints to satisfy
		if len(requiredArg.Values) == 0 && len(requiredArg.ResolvesTo) == 0 {
			continue
		}

		// Not enough arguments, so this positioned arg constraint cannot be satisfied
		if requiredArg.Index >= uint64(len(callArguments)) {
			return false
		}

		argAtIndex := callArguments[requiredArg.Index]

		constraintsSatisfied := false

		// an argument can be resolved to multiple namespaces or literal nodes
		// so we need to check if "any" of the resolved nodes satisfy the constraints
		for _, argNode := range argAtIndex.Nodes {
			if argNode.IsLiteralValue() {
				if slices.Contains(requiredArg.Values, argNode.Namespace) {
					// If this is a literal value and it matches any of the required values,
					// then this arg's constraints are satisfied
					constraintsSatisfied = true
					break
				}
			} else {
				if slices.Contains(requiredArgResolvesToNamespaces, argNode.Namespace) {
					// If this is a resolved type and it matches any of the required
					// arg resolves_to namespaces, then this arg's constraints are satisfied
					constraintsSatisfied = true
					break
				}
			}
		}

		if !constraintsSatisfied {
			// If this arg's constraints are not satisfied, then the entire condition is not satisfied
			return false
		}
	}

	return true
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
