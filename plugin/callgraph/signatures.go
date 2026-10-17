package callgraph

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	callgraphv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/code/callgraph/v1"
	"buf.build/go/protovalidate"
	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
	"github.com/safedep/dry/log"
)

const (
	MatchAny = "any"
	MatchAll = "all"
)

// MatchedEvidence is one call that matched a condition. Site is the
// position of the expression that makes the call, and Text its source.
type MatchedEvidence struct {
	Caller    *CallGraphNode
	Callee    *CallGraphNode
	Site      ast.NodePosition
	Text      string
	Arguments []CallArgument
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
	languageCode := cg.Language

	matcherResults := []SignatureMatchResult{}

	dfsResults := cg.DFS()

	if cg.LimitExceeded() {
		log.Warnf("CallGraph for %s was truncated by %v, signature matching may be incomplete", cg.FileName, cg.Truncations())
	}

	if len(dfsResults) > 50000 {
		log.Warnf("Large DFS result set (%d items) for file %s, signature matching may be slow", len(dfsResults), cg.FileName)
	}

	calls := newCallIndex(dfsResults)

	for _, signature := range sm.targetSignatures {
		languageSignature, exists := signature.Languages[string(languageCode)]
		if !exists {
			// TypeScript falls back to JavaScript signatures since they share
			// the same library ecosystem and call patterns
			if languageCode == core.LanguageCodeTypescript {
				languageSignature, exists = signature.Languages[string(core.LanguageCodeJavascript)]
			}
			if !exists {
				continue
			}
		}

		matchedConditions := []MatchedCondition{}
		for _, condition := range languageSignature.Conditions {
			if condition.Type != "call" {
				continue
			}

			matchCondition := MatchedCondition{
				Condition: condition,
				Evidences: []MatchedEvidence{},
			}

			for _, item := range calls.lookup(profileOf(languageCode).moduleNamespace(condition.Value)) {
				if !matchesArgumentConstraints(item, condition.Args, languageCode) {
					continue
				}
				matchCondition.Evidences = append(matchCondition.Evidences, MatchedEvidence{
					Caller:    item.Caller,
					Callee:    calleeOf(item),
					Site:      item.Site,
					Text:      item.Text,
					Arguments: item.Arguments,
				})
			}

			if len(matchCondition.Evidences) > 0 {
				matchedConditions = append(matchedConditions, matchCondition)
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

// callIndex holds the DFS results of a graph by namespace. The keys are
// sorted, so a wildcard lookup gives its results in one order.
type callIndex struct {
	items map[string][]DfsResultItem
	keys  []string
}

func newCallIndex(results []DfsResultItem) callIndex {
	items := make(map[string][]DfsResultItem)
	for _, item := range results {
		items[item.Namespace] = append(items[item.Namespace], item)
	}
	return callIndex{items: items, keys: slices.Sorted(maps.Keys(items))}
}

// lookup returns the results of namespace. A namespace that ends in //*,
// as os//*, gives the results of every namespace below it, in key order.
// Only the last segment can be a wildcard.
func (ci callIndex) lookup(namespace string) []DfsResultItem {
	prefix, wildcard := strings.CutSuffix(namespace, namespaceSeparator+"*")
	if !wildcard {
		return ci.items[namespace]
	}

	prefix += namespaceSeparator
	var items []DfsResultItem
	start, _ := slices.BinarySearch(ci.keys, prefix)
	for _, key := range ci.keys[start:] {
		if !strings.HasPrefix(key, prefix) {
			break
		}
		items = append(items, ci.items[key]...)
	}
	return items
}

// calleeOf returns the call graph node of a DFS result. A namespace that
// only the assignment graph knows, such as a Python attribute of an
// imported module, has no node. It gets a node with the namespace, so the
// evidence still names the callee.
func calleeOf(item DfsResultItem) *CallGraphNode {
	if item.Node != nil {
		return item.Node
	}
	return &CallGraphNode{Namespace: item.Namespace}
}

// matchesArgumentConstraints checks if the arguments in the DFS result item
// match the required argument constraints specified in the signature condition.
// It returns true if "all" required arguments match their respective constraints,
func matchesArgumentConstraints(
	dfsResultItem DfsResultItem,
	requiredArgs []*callgraphv1.Signature_LanguageMatcher_SignatureCondition_Argument,
	languageCode core.LanguageCode,
) bool {
	if len(requiredArgs) == 0 {
		return true // No conditions to match, so it matches by default
	}

	callArguments := dfsResultItem.Arguments

	profile := profileOf(languageCode)

	// Strictly all arguments must match their respective constraints
	for _, requiredArg := range requiredArgs {
		requiredArgResolvesToNamespaces := make([]string, len(requiredArg.ResolvesTo))
		for i, resolvesTo := range requiredArg.ResolvesTo {
			requiredArgResolvesToNamespaces[i] = profile.moduleNamespace(resolvesTo)
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
		for _, value := range argAtIndex.Values {
			if value.Literal {
				if slices.Contains(requiredArg.Values, value.Namespace) {
					// If this is a literal value and it matches any of the required values,
					// then this arg's constraints are satisfied
					constraintsSatisfied = true
					break
				}
			} else {
				if slices.Contains(requiredArgResolvesToNamespaces, value.Namespace) {
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

		if err := withoutSupportedLanguageKeys(v.Validate(signature)); err != nil {
			return err
		}
	}

	return nil
}

// withoutSupportedLanguageKeys drops the violations of the language list in
// the signature schema for a language that this plugin supports. The schema
// in safedep/api lists fewer languages than the plugin supports.
func withoutSupportedLanguageKeys(err error) error {
	var validationErr *protovalidate.ValidationError
	if !errors.As(err, &validationErr) {
		return err
	}

	var kept []*protovalidate.Violation
	for _, violation := range validationErr.Violations {
		if !isSupportedLanguageKey(violation) {
			kept = append(kept, violation)
		}
	}

	if len(kept) == 0 {
		return nil
	}
	return &protovalidate.ValidationError{Violations: kept}
}

func isSupportedLanguageKey(violation *protovalidate.Violation) bool {
	if !violation.Proto.GetForKey() || violation.Proto.GetRuleId() != "string.in" {
		return false
	}

	elements := violation.Proto.GetField().GetElements()
	if len(elements) != 1 || elements[0].GetFieldName() != "languages" {
		return false
	}

	_, supported := profilesByCode[core.LanguageCode(elements[0].GetStringKey())]
	return supported
}
