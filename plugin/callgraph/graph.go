package callgraph

import (
	"fmt"
	"slices"

	"github.com/safedep/code/core"
	"github.com/safedep/dry/log"
	sitter "github.com/smacker/go-tree-sitter"
)

const namespaceSeparator = "//"

// graphNode represents a single node in the call graph
type graphNode struct {
	Namespace string
	CallsTo   []string
	TreeNode  *sitter.Node
}

type ContentDetails struct {
	StartLine   uint32
	EndLine     uint32
	StartColumn uint32
	EndColumn   uint32
	Content     string
}

func (gn *graphNode) GetContentDetails(treeData *[]byte) (ContentDetails, error) {
	if gn.TreeNode == nil {
		return ContentDetails{}, fmt.Errorf("TreeNode is nil")
	}
	return ContentDetails{
		StartLine:   gn.TreeNode.StartPoint().Row,
		EndLine:     gn.TreeNode.EndPoint().Row,
		StartColumn: gn.TreeNode.StartPoint().Column,
		EndColumn:   gn.TreeNode.EndPoint().Column,
		Content:     gn.TreeNode.Content(*treeData),
	}, nil
}

func newCallGraphNode(namespace string, treeNode *sitter.Node) *graphNode {
	return &graphNode{
		Namespace: namespace,
		CallsTo:   []string{},
		TreeNode:  treeNode,
	}
}

type CallGraph struct {
	FileName          string
	Nodes             map[string]*graphNode
	assignmentGraph   AssignmentGraph
	classConstructors map[string]bool
	Tree              core.ParseTree
}

func NewCallGraph(fileName string, importedIdentifiers map[string]parsedImport, tree core.ParseTree) (*CallGraph, error) {
	language, err := tree.Language()
	if err != nil {
		return nil, fmt.Errorf("failed to get language from parse tree: %w", err)
	}

	builtIns := GetBuiltins(language)

	cg := &CallGraph{
		FileName:          fileName,
		Nodes:             make(map[string]*graphNode),
		assignmentGraph:   *NewAssignmentGraph(),
		classConstructors: make(map[string]bool),
		Tree:              tree,
	}

	for identifier, importedIdentifier := range importedIdentifiers {
		if identifier == importedIdentifier.Namespace {
			cg.assignmentGraph.AddIdentifier(importedIdentifier.Namespace, importedIdentifier.NamespaceTreeNode)
			cg.AddNode(importedIdentifier.Namespace, importedIdentifier.NamespaceTreeNode)
		} else {
			cg.assignmentGraph.AddAssignment(identifier, importedIdentifier.IdentifierTreeNode, importedIdentifier.Namespace, importedIdentifier.NamespaceTreeNode)
			cg.AddEdge(identifier, importedIdentifier.IdentifierTreeNode, importedIdentifier.Namespace, importedIdentifier.NamespaceTreeNode)
		}
	}

	for _, namespace := range builtIns {
		cg.assignmentGraph.AddIdentifier(namespace, nil) // @TODO - Can't create sitter node for keywords
	}

	return cg, nil
}

func (cg *CallGraph) AddNode(identifier string, treeNode *sitter.Node) {
	if _, exists := cg.Nodes[identifier]; !exists {
		cg.Nodes[identifier] = newCallGraphNode(identifier, treeNode)
	}
}

// AddEdge adds an edge from one function to another
func (cg *CallGraph) AddEdge(caller string, callerTreeNode *sitter.Node, callee string, calleeTreeNode *sitter.Node) {
	cg.AddNode(caller, callerTreeNode)
	cg.AddNode(callee, calleeTreeNode)
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
	for assignmentNamespace, assignmentNode := range cg.assignmentGraph.Assignments {
		fmt.Printf("  %s => %v\n", assignmentNamespace, assignmentNode.AssignedTo)
	}
	fmt.Println()
}

type DfsResultItem struct {
	Node     *graphNode
	Depth    int
	Terminal bool
}

func (cg *CallGraph) DFS() []DfsResultItem {
	visited := make(map[string]bool)
	var dfsResult []DfsResultItem
	cg.dfsUtil(cg.FileName, visited, &dfsResult, 0)
	return dfsResult
}

func (cg *CallGraph) dfsUtil(startNode string, visited map[string]bool, result *[]DfsResultItem, depth int) {
	if visited[startNode] {
		// For debugging
		// *result = append(*result, DfsResultItem{
		// 	Namespace: fmt.Sprintf("|- Stopped at %s (Already visited)", startNode),
		// 	Depth:     depth,
		// 	Terminal:  false,
		// })
		return
	}

	callgraphNode, callgraphNodeExists := cg.Nodes[startNode]

	// Mark the current node as visited and add it to the result
	visited[startNode] = true
	*result = append(*result, DfsResultItem{
		Node:     callgraphNode,
		Depth:    depth,
		Terminal: !callgraphNodeExists || len(callgraphNode.CallsTo) == 0,
	})

	assignmentGraphNode, assignmentNodeExists := cg.assignmentGraph.Assignments[startNode]
	if assignmentNodeExists {
		// Recursively visit all the nodes assigned to the current node
		for _, assigned := range assignmentGraphNode.AssignedTo {
			cg.dfsUtil(assigned, visited, result, depth)
		}
	}

	// Recursively visit all the nodes called by the current node
	// Any variable assignment would be ignored here, since it won't be in callgraph
	if callgraphNodeExists {
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
