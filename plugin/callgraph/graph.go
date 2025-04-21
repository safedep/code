package callgraph

import (
	"fmt"
	"slices"

	"github.com/priyakdey/trie"
	"github.com/safedep/code/core"
	"github.com/safedep/dry/log"
)

const namespaceSeparator = "//"

// graphNode represents a single node in the call graph
type graphNode struct {
	Namespace string
	CallsTo   []string
}

func newCallGraphNode(namespace string) *graphNode {
	return &graphNode{
		Namespace: namespace,
		CallsTo:   []string{},
	}
}

type CallGraph struct {
	FileName                     string
	Nodes                        map[string]*graphNode
	assignments                  AssignmentGraph
	classConstructors            map[string]bool
	importedIdentifierNamespaces map[string]string
	Tree                         core.ParseTree
}

func NewCallGraph(fileName string, importedIdentifierNamespaces map[string]string, tree core.ParseTree) (*CallGraph, error) {
	language, err := tree.Language()
	if err != nil {
		return nil, fmt.Errorf("failed to get language from parse tree: %w", err)
	}

	builtIns := GetBuiltins(language)

	cg := &CallGraph{
		FileName:                     fileName,
		Nodes:                        make(map[string]*graphNode),
		assignments:                  *NewAssignmentGraph(),
		classConstructors:            make(map[string]bool),
		importedIdentifierNamespaces: importedIdentifierNamespaces,
		Tree:                         tree,
	}

	for identifier, namespace := range importedIdentifierNamespaces {
		if identifier == namespace {
			cg.assignments.AddIdentifier(identifier)
			cg.AddNode(identifier)
		} else {
			cg.assignments.AddAssignment(identifier, namespace)
			cg.AddEdge(identifier, namespace)
		}
	}
	for identifier, namespace := range builtIns {
		cg.assignments.AddAssignment(identifier, namespace)
	}

	return cg, nil
}

func (cg *CallGraph) AddNode(identifier string) {
	if _, exists := cg.Nodes[identifier]; !exists {
		cg.Nodes[identifier] = newCallGraphNode(identifier)
	}
}

// AddEdge adds an edge from one function to another
func (cg *CallGraph) AddEdge(caller, callee string) {
	cg.AddNode(caller)
	cg.AddNode(callee)
	if !slices.Contains(cg.Nodes[caller].CallsTo, callee) {
		cg.Nodes[caller].CallsTo = append(cg.Nodes[caller].CallsTo, callee)
	}
}

func (cg *CallGraph) PrintCallGraph() {
	fmt.Println("Call Graph:")
	for caller, node := range cg.Nodes {
		fmt.Printf("  %s (calls)=> %v\n", caller, node.CallsTo)
	}
	fmt.Println()
}
func (cg *CallGraph) PrintAssignmentGraph() {
	fmt.Println("Assignment Graph:")
	for identifier, namespaces := range cg.assignments.Assignments {
		fmt.Printf("  %s => %s\n", identifier, namespaces)
	}
	fmt.Println()
}

type DfsResultItem struct {
	Namespace string
	Depth     int
}

func (cg *CallGraph) DFS() []DfsResultItem {
	visited := make(map[string]bool)
	var dfsResult []DfsResultItem
	cg.dfsUtil(cg.FileName, visited, &dfsResult, 0)
	return dfsResult
}

func (cg *CallGraph) dfsUtil(startNode string, visited map[string]bool, result *[]DfsResultItem, depth int) {
	// fmt.Println(startNode, depth)
	if visited[startNode] {
		// @TODO - Only for debugging
		*result = append(*result, DfsResultItem{
			Namespace: fmt.Sprintf("|- Stopped at %s (Already visited)", startNode),
			Depth:     depth,
		})
		return
	}

	// Mark the current node as visited and add it to the result
	visited[startNode] = true
	*result = append(*result, DfsResultItem{
		Namespace: startNode,
		Depth:     depth,
	})

	// Recursively visit all the nodes assigned to the current node
	for _, assigned := range cg.assignments.Assignments[startNode] {
		cg.dfsUtil(assigned, visited, result, depth)
	}

	// Recursively visit all the nodes called by the current node
	// Any variable assignment would be ignored here, since it won't be in callgraph
	callgraphNode, exists := cg.Nodes[startNode]
	if exists {
		for _, callee := range callgraphNode.CallsTo {
			cg.dfsUtil(callee, visited, result, depth+1)
		}
	}
}

func (cg *CallGraph) GetInstanceKeyword() (string, bool) {
	language, err := cg.Tree.Language()
	if err != nil {
		log.Errorf("failed to get language from parse tree: %v", err)
		return "", false
	}
	return resolveInstanceKeyword(language)
}

type SignatureMatchResult struct {
	MatchedSignature    *Signature
	MatchedLanguageCode core.LanguageCode
	// MatchedConditions    []string
}

func (cg *CallGraph) MatchSignatures(targetSignatures []Signature) ([]SignatureMatchResult, error) {
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
		functionCallTrie.Insert(resultItem.Namespace)
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
