package callgraph

import (
	"context"
	"fmt"
	"strings"

	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
	"github.com/safedep/dry/log"
	sitter "github.com/smacker/go-tree-sitter"
)

const namespaceSeparator = "//"

const (
	// The limits keep the memory of a large or hostile file bounded. A
	// build past a limit records a Truncation.
	maxCallGraphNodes       = 50000
	maxAssignmentGraphNodes = 100000
	maxDFSResultItems       = 100000
)

// Refers to one argument passed to a function call
// For example, in the function call `foo(a, b, c)`, there are
// three CallArgument instances, one for each of `a`, `b`, and `c
type CallArgument struct {
	// An argument may be resolved to multiple values (due to assignments)
	// For example, a function call may have an argument like `foo(a)` where
	// a was assigned to multiple values in the code previously
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

	// Unbound marks a bare callee that the scope chain did not bind, as a
	// function that the file declares after the call. CalleeNamespace is
	// then a fallback key.
	Unbound bool
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
	languageCode      core.LanguageCode

	// order holds the nodes in the order of the build, so DFS gives one
	// result for one file.
	order          []*CallGraphNode
	truncations    truncations
	unresolved     []UnresolvedCall
	unresolvedSeen map[UnresolvedCall]bool

	// ctx stops processChildren when it is done. buildCallGraph sets it for
	// the walk of the tree and clears it after, so the graph does not keep
	// the context.
	ctx     context.Context
	visited int
	ctxErr  error
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
		classConstructors: make(map[string]bool),
		languageCode:      language.Meta().Code,
		unresolvedSeen:    make(map[UnresolvedCall]bool),
	}
	cg.assignmentGraph = *newAssignmentGraph(&cg.truncations)

	// Add root node to the call graph
	cg.addRootNode(rootNode)

	// Required to map identifiers to imported modules as assignments
	// and register default calls for wildcard imports
	importedIdentifiers, wildcardImports := parseImports(imports, language)

	for _, wildcardImport := range wildcardImports {
		// For wildcard imports, we add a call to importeditem//*
		// assuming that anything under that namespace is posssibly used
		cg.addEdge(fileName, rootNode, wildcardImport.NamespaceTreeNode, wildcardImport.Namespace, wildcardImport.NamespaceTreeNode, []CallArgument{})
	}

	for _, importedIdentifier := range importedIdentifiers {
		cg.addNode(importedIdentifier.Namespace, importedIdentifier.NamespaceTreeNode)

		if importedIdentifier.Identifier != importedIdentifier.Namespace {
			cg.assignmentGraph.addAssignment(importedIdentifier.Identifier, importedIdentifier.IdentifierTreeNode, importedIdentifier.Namespace, importedIdentifier.NamespaceTreeNode)
		}
	}

	for _, namespace := range builtIns {
		// @TODO - Can't create sitter node for imported keywords
		cg.assignmentGraph.addNode(namespace, nil)
	}

	return cg, nil
}

// ctxCheckInterval is the number of nodes between two checks of the context.
const ctxCheckInterval = 4096

// stopped reports that the build must stop, because ctx is done. It checks
// ctx every ctxCheckInterval nodes, so the check costs little per node.
func (cg *CallGraph) stopped() bool {
	if cg.ctxErr != nil {
		return true
	}
	if cg.ctx == nil {
		return false
	}
	cg.visited++
	if cg.visited%ctxCheckInterval != 0 {
		return false
	}
	if err := cg.ctx.Err(); err != nil {
		log.Debugf("CallGraph build stopped for file %s: %v", cg.FileName, err)
		cg.ctxErr = err
		cg.truncate(TruncationDeadline, cg.FileName, nil)
		return true
	}
	return false
}

func (cg *CallGraph) addRootNode(treeNode *sitter.Node) {
	cg.RootNode = cg.addNode(cg.FileName, treeNode)
}

// addNode adds a node to the call graph and to the assignment graph. Past
// maxCallGraphNodes it adds no node to the call graph and returns nil.
func (cg *CallGraph) addNode(identifier string, treeNode *sitter.Node) *CallGraphNode {
	cg.assignmentGraph.addNode(identifier, treeNode)
	return cg.addCallGraphNode(identifier, treeNode)
}

func (cg *CallGraph) addCallGraphNode(identifier string, treeNode *sitter.Node) *CallGraphNode {
	if node, exists := cg.Nodes[identifier]; exists {
		if node.TreeNode == nil {
			node.TreeNode = treeNode
		}
		return node
	}

	if len(cg.order) >= maxCallGraphNodes {
		cg.truncate(TruncationNodes, identifier, treeNode)
		return nil
	}

	node := newCallGraphNode(identifier, treeNode)
	cg.Nodes[identifier] = node
	cg.order = append(cg.order, node)
	return node
}

// addEdge adds an edge from one function to another
func (cg *CallGraph) addEdge(caller string, callerTreeNode *sitter.Node, callerIdentifier *sitter.Node, callee string, calleeTreeNode *sitter.Node, arguments []CallArgument) {
	cg.addCall(caller, callerTreeNode, CallReference{
		CalleeNamespace:  callee,
		CalleeTreeNode:   calleeTreeNode,
		CallerIdentifier: callerIdentifier,
		Arguments:        arguments,
	})
}

// addCall adds the edge of ref from caller. It adds no edge when the graph
// has no room for the caller or the callee.
func (cg *CallGraph) addCall(caller string, callerTreeNode *sitter.Node, ref CallReference) {
	callerNode := cg.addNode(caller, callerTreeNode)

	// An unbound callee stays out of the assignment graph. Otherwise the
	// scope chain finds the fallback key, and a second call to the name
	// looks bound.
	var calleeNode *CallGraphNode
	if ref.Unbound {
		calleeNode = cg.addCallGraphNode(ref.CalleeNamespace, ref.CalleeTreeNode)
	} else {
		calleeNode = cg.addNode(ref.CalleeNamespace, ref.CalleeTreeNode)
	}

	if callerNode == nil || calleeNode == nil {
		return
	}
	callerNode.CallsTo = append(callerNode.CallsTo, ref)
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
		for i, callRef := range node.CallsTo {
			callsToNamespaces[i] = callRef.CalleeNamespace
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
	"program":                    true,
	"file":                       true,
	"module":                     true,
	"function_definition":        true,
	"method_declaration":         true,
	"class_definition":           true,
	"class_body":                 true,
	"class_declaration":          true,
	"abstract_class_declaration": true,
	// JavaScript and TypeScript functions, which a file often exports and
	// does not call
	"function_declaration":           true,
	"generator_function_declaration": true,
	"function_expression":            true,
	"arrow_function":                 true,
	"method_definition":              true,
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
	resultCount := 0

	// Initially Interpret callgraph in its natural execution order starting from
	// the file name which has reference for entrypoints (if any)
	cg.dfsUtil(cg.FileName, cg.RootNode, nil, []CallArgument{}, visited, &dfsResult, 0, &resultCount)

	// Assumption - All functions and class constructors are reachable
	// This is required because most files only expose their classes/functions
	// which are imported and used by other files, so an entrypoint may not be
	// present in every file.
	for _, node := range cg.order {
		if resultCount >= maxDFSResultItems {
			break
		}

		if node.TreeNode != nil && dfsSourceNodeTypes[node.TreeNode.Type()] && !visited[node.Namespace] {
			cg.dfsUtil(node.Namespace, cg.RootNode, nil, []CallArgument{}, visited, &dfsResult, 0, &resultCount)
		}
	}

	if resultCount >= maxDFSResultItems {
		cg.truncate(TruncationDFSResults, cg.FileName, nil)
	}

	return dfsResult
}

func (cg *CallGraph) dfsUtil(namespace string, caller *CallGraphNode, callerIdentifier *sitter.Node, arguments []CallArgument, visited map[string]bool, result *[]DfsResultItem, depth int, resultCount *int) {
	// Check result count limit
	if *resultCount >= maxDFSResultItems {
		return
	}

	callgraphNode, callgraphNodeExists := cg.Nodes[namespace]

	if visited[namespace] {
		resolvedAssignmentTerminals := cg.assignmentGraph.resolve(namespace)
		for _, terminalAssignmentNode := range resolvedAssignmentTerminals {
			if *resultCount >= maxDFSResultItems {
				return
			}

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
				*resultCount++
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
	*resultCount++

	assignmentGraphNode, assignmentNodeExists := cg.assignmentGraph.Assignments[namespace]
	if assignmentNodeExists {
		// Recursively visit all the nodes assigned to the current node
		for _, assigned := range assignmentGraphNode.AssignedTo {
			if *resultCount >= maxDFSResultItems {
				return
			}
			cg.dfsUtil(assigned, caller, callerIdentifier, arguments, visited, result, depth, resultCount)
		}
	}

	// Recursively visit all the nodes called by the current node
	// Any variable assignment would be ignored here, since it won't be in callgraph
	if callgraphNodeExists {
		for _, callRef := range callgraphNode.CallsTo {
			if *resultCount >= maxDFSResultItems {
				return
			}
			cg.dfsUtil(callRef.CalleeNamespace, callgraphNode, callRef.CallerIdentifier, callRef.Arguments, visited, result, depth+1, resultCount)
		}
	}
}

func (cg *CallGraph) getInstanceKeyword() (string, bool) {
	keyword, exists := instanceKeywordMapping[string(cg.languageCode)]
	return keyword, exists
}
