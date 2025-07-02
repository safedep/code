package callgraph

import (
	"fmt"
	"strings"

	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
	"github.com/safedep/dry/log"
	sitter "github.com/smacker/go-tree-sitter"
)

const namespaceSeparator = "//"

// Refers to one argument passed to a function call
// For example, in the function call `foo(a, b, c)`, there are
// three CallArgument instances, one for each of `a`, `b`, and `c
type CallArgument struct {
	// An argument may be resolved to multiple values (due to assignments)
	// For example, a function call may have an argument like `foo(a)` where
	// a was assigneed to multiple values in the code previously
	// eg. `a = 1`, `a = 2`, `a = 3
	Nodes []*assignmentNode
}

type CallReference struct {
	// Namespace of the called function or method
	CalleeNamespace string

	// Tree sitter node for the called function or method
	CalleeTreeNode *sitter.Node

	// Identifier keyword node which actually invokes the function call
	CallerIdentifier *sitter.Node

	// Arguments passed to the function call
	Arguments []CallArgument
}

// CallGraphNode represents a single node in the call graph
type CallGraphNode struct {
	Namespace string
	CallsTo   []CallReference
	TreeNode  *sitter.Node
}

type TreeNodeMetadata struct {
	StartLine   uint32
	EndLine     uint32
	StartColumn uint32
	EndColumn   uint32
}

// If tree sitter node is nil, it returns false indicating that the content details are not available
// else, it returns the content details and true
func (gn *CallGraphNode) Metadata() (TreeNodeMetadata, bool) {
	if gn.TreeNode == nil {
		return TreeNodeMetadata{}, false
	}

	return TreeNodeMetadata{
		StartLine:   gn.TreeNode.StartPoint().Row,
		EndLine:     gn.TreeNode.EndPoint().Row,
		StartColumn: gn.TreeNode.StartPoint().Column,
		EndColumn:   gn.TreeNode.EndPoint().Column,
	}, true
}

// If tree sitter node is nil, it returns false indicating that the content details are not available
// else, it returns the content details and true
func (gn *CallGraphNode) Content(treeData *[]byte) (string, bool) {
	if gn.TreeNode == nil {
		return "", false
	}
	return gn.TreeNode.Content(*treeData), true
}

func newCallGraphNode(namespace string, treeNode *sitter.Node) *CallGraphNode {
	return &CallGraphNode{
		Namespace: namespace,
		CallsTo:   []CallReference{},
		TreeNode:  treeNode,
	}
}

type CallGraph struct {
	FileName          string
	Nodes             map[string]*CallGraphNode
	RootNode          *CallGraphNode
	Tree              core.ParseTree
	assignmentGraph   assignmentGraph
	classConstructors map[string]bool
}

func newCallGraph(fileName string, rootNode *sitter.Node, imports []*ast.ImportNode, tree core.ParseTree) (*CallGraph, error) {
	language, err := tree.Language()
	if err != nil {
		return nil, fmt.Errorf("failed to get language from parse tree: %w", err)
	}

	builtIns := getBuiltins(language)

	cg := &CallGraph{
		FileName:          fileName,
		Nodes:             make(map[string]*CallGraphNode),
		Tree:              tree,
		assignmentGraph:   *newAssignmentGraph(),
		classConstructors: make(map[string]bool),
	}

	// Add root node to the call graph
	cg.addRootNode(rootNode)

	// Required to map identifiers to imported modules as assignments
	// and register default calls for wildcard imports
	importedIdentifiers, wildcardImports := parseImports(imports, language)

	for _, wildcardImport := range wildcardImports {
		// For wildcard imports, we add a call to importeditem//*
		// assuming that anything under that namespace is posssibly used
		cg.AddEdge(fileName, rootNode, wildcardImport.NamespaceTreeNode, wildcardImport.Namespace, wildcardImport.NamespaceTreeNode, []CallArgument{})
	}

	for identifier, importedIdentifier := range importedIdentifiers {
		cg.AddNode(importedIdentifier.Namespace, importedIdentifier.NamespaceTreeNode)

		if identifier == importedIdentifier.Namespace {
			cg.assignmentGraph.AddNode(importedIdentifier.Namespace, importedIdentifier.NamespaceTreeNode)
		} else {
			cg.assignmentGraph.AddAssignment(identifier, importedIdentifier.IdentifierTreeNode, importedIdentifier.Namespace, importedIdentifier.NamespaceTreeNode)
		}
	}

	for _, namespace := range builtIns {
		// @TODO - Can't create sitter node for imported keywords
		cg.assignmentGraph.AddNode(namespace, nil)
	}

	return cg, nil
}

func (cg *CallGraph) addRootNode(treeNode *sitter.Node) {
	cg.AddNode(cg.FileName, treeNode)
	cg.RootNode = cg.Nodes[cg.FileName]
}

func (cg *CallGraph) ensureNodeInAssignmentGraph(identifier string, treeNode *sitter.Node) {
	cg.assignmentGraph.AddNode(identifier, treeNode)
}

func (cg *CallGraph) AddNode(identifier string, treeNode *sitter.Node) *CallGraphNode {
	cg.ensureNodeInAssignmentGraph(identifier, treeNode)

	existingCgNode, exists := cg.Nodes[identifier]
	if !exists {
		cg.Nodes[identifier] = newCallGraphNode(identifier, treeNode)
	} else if treeNode != nil && existingCgNode.TreeNode == nil {
		// If the existing node has no tree node, we can set it now
		cg.Nodes[identifier].TreeNode = treeNode
	}

	return cg.Nodes[identifier]
}

// AddEdge adds an edge from one function to another
func (cg *CallGraph) AddEdge(caller string, callerTreeNode *sitter.Node, CallerIdentifier *sitter.Node, callee string, calleeTreeNode *sitter.Node, arguments []CallArgument) {
	cg.AddNode(caller, callerTreeNode)
	cg.AddNode(callee, calleeTreeNode)

	cg.Nodes[caller].CallsTo = append(cg.Nodes[caller].CallsTo, CallReference{
		CalleeNamespace:  callee,
		CalleeTreeNode:   calleeTreeNode,
		CallerIdentifier: CallerIdentifier,
		Arguments:        arguments,
	})
}

func (cg *CallGraph) PrintCallGraph() error {
	lang, err := cg.Tree.Language()
	if err != nil {
		return fmt.Errorf("failed to get language from callgraph: %w", err)
	}

	builtInsMap := getBuiltinsMap(lang)

	fmt.Println("Call Graph:")
	for caller, node := range cg.Nodes {
		if builtInsMap[caller] && len(node.CallsTo) == 0 {
			continue // Skip built-in functions with no outgoing calls
		}

		callsToNamespaces := make([]string, len(node.CallsTo))
		for _, callRef := range node.CallsTo {
			callsToNamespaces = append(callsToNamespaces, callRef.CalleeNamespace)
		}
		fmt.Printf("  %s (calls)=> %v\n", caller, callsToNamespaces)
	}
	fmt.Println()

	return nil
}

func (cg *CallGraph) PrintAssignmentGraph() error {
	lang, err := cg.Tree.Language()
	if err != nil {
		return fmt.Errorf("failed to get language from callgraph: %w", err)
	}

	builtInsMap := getBuiltinsMap(lang)

	fmt.Println("Assignment Graph:")
	for assignmentNamespace, assignmentNode := range cg.assignmentGraph.Assignments {
		if builtInsMap[assignmentNamespace] && len(assignmentNode.AssignedTo) == 0 {
			continue // Skip built-in functions with no calls
		}
		fmt.Printf("  %s (->%d) => %v\n", assignmentNamespace, len(assignmentNode.AssignedBy), assignmentNode.AssignedTo)
	}
	fmt.Println()

	return nil
}

// Assumption - All functions and class constructors are reachable
var dfsSourceNodeTypes = map[string]bool{
	"program":             true,
	"file":                true,
	"module":              true,
	"function_definition": true,
	"method_declaration":  true,
	"class_definition":    true,
	"class_body":          true,
	"class_declaration":   true,
}

type DfsResultItem struct {
	Namespace        string
	Node             *CallGraphNode
	Caller           *CallGraphNode
	CallerIdentifier *sitter.Node // This is the identifier keyword node from which the fn call was made
	Arguments        []CallArgument
	Depth            int
	Terminal         bool
}

func (dri DfsResultItem) ToString() string {
	argNamespacesStr := ""
	for _, arg := range dri.Arguments {
		argNamespaces := make([]string, len(arg.Nodes))
		for i, argNode := range arg.Nodes {
			argNamespaces[i] = argNode.Namespace
		}
		argNamespacesStr += fmt.Sprintf("(%s), ", strings.Join(argNamespaces, ", "))
	}

	return fmt.Sprintf("Namespace: %s, Caller: %s, Depth: %d, Terminal: %t, Arguments: [%s]",
		dri.Namespace, dri.Caller.Namespace, dri.Depth, dri.Terminal, argNamespacesStr)
}

func (cg *CallGraph) DFS() []DfsResultItem {
	visited := make(map[string]bool)
	var dfsResult []DfsResultItem

	// Initially Interpret callgraph in its natural execution order starting from
	// the file name which has reference for entrypoints (if any)
	cg.dfsUtil(cg.FileName, cg.RootNode, nil, []CallArgument{}, visited, &dfsResult, 0)

	// Assumption - All functions and class constructors are reachable
	// This is required because most files only expose their classes/functions
	// which are imported and used by other files, so an entrypoint may not be
	// present in every file.
	for namespace, node := range cg.Nodes {
		if node.TreeNode != nil && dfsSourceNodeTypes[node.TreeNode.Type()] {
			if !visited[namespace] {
				cg.dfsUtil(namespace, cg.RootNode, nil, []CallArgument{}, visited, &dfsResult, 0)
			}
		}
	}

	return dfsResult
}

func (cg *CallGraph) dfsUtil(namespace string, caller *CallGraphNode, callerIdentifier *sitter.Node, arguments []CallArgument, visited map[string]bool, result *[]DfsResultItem, depth int) {
	callgraphNode, callgraphNodeExists := cg.Nodes[namespace]

	if visited[namespace] {
		resolvedAssignmentTerminals := cg.assignmentGraph.Resolve(namespace)
		for _, terminalAssignmentNode := range resolvedAssignmentTerminals {
			terminalCallgraphNode, terminalCallgraphNodeExists := cg.Nodes[terminalAssignmentNode.Namespace]
			if terminalCallgraphNodeExists {
				*result = append(*result, DfsResultItem{
					Namespace:        terminalAssignmentNode.Namespace,
					Node:             terminalCallgraphNode,
					Caller:           caller,
					CallerIdentifier: callerIdentifier,
					Arguments:        arguments,
					Depth:            depth,
					Terminal:         true,
				})
			}
		}
		return
	}

	// Mark the current node as visited and add it to the result
	visited[namespace] = true
	*result = append(*result, DfsResultItem{
		Namespace:        namespace,
		Node:             callgraphNode,
		Caller:           caller,
		CallerIdentifier: callerIdentifier,
		Arguments:        arguments,
		Depth:            depth,
		Terminal:         !callgraphNodeExists || len(callgraphNode.CallsTo) == 0,
	})

	assignmentGraphNode, assignmentNodeExists := cg.assignmentGraph.Assignments[namespace]
	if assignmentNodeExists {
		// Recursively visit all the nodes assigned to the current node
		for _, assigned := range assignmentGraphNode.AssignedTo {
			cg.dfsUtil(assigned, caller, callerIdentifier, arguments, visited, result, depth)
		}
	}

	// Recursively visit all the nodes called by the current node
	// Any variable assignment would be ignored here, since it won't be in callgraph
	if callgraphNodeExists {
		for _, callRef := range callgraphNode.CallsTo {
			cg.dfsUtil(callRef.CalleeNamespace, callgraphNode, callRef.CallerIdentifier, callRef.Arguments, visited, result, depth+1)
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
