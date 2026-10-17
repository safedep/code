package callgraph

import (
	"context"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
)

const namespaceSeparator = "//"

const (
	// The limits keep the memory of a large or hostile file bounded. A
	// build past a limit records a Truncation.
	maxCallGraphNodes       = 50000
	maxAssignmentGraphNodes = 100000
	maxDFSResultItems       = 100000
)

// CallArgument is one argument of a call. In foo(a, b, c), the call has
// three arguments.
type CallArgument struct {
	// Values are the terminal values that the argument resolves to. An
	// argument can resolve to more than one value, as a after a = 1 and a = 2.
	Values []ArgumentValue
}

// ArgumentValue is one value of an argument.
type ArgumentValue struct {
	// Namespace is the key of the value. For a literal it is the source of
	// the literal.
	Namespace string
	Literal   bool
}

type CallReference struct {
	// CalleeNamespace is the callee as the call writes it, as the local name
	// load after import { load } from 'js-yaml'.
	CalleeNamespace string

	// Targets are the namespaces that the assignment graph resolves the
	// callee to at the end of the build, sorted, as js-yaml//load for load.
	// A callee with no assignment is its own target. An unbound callee that
	// the assignment graph does not know has no target.
	Targets []string

	// Site is the position of the expression that makes the call, and Text
	// its source, at most maxSiteText bytes.
	Site ast.NodePosition
	Text string

	Arguments []CallArgument

	// Unbound marks a bare callee that the scope chain did not bind, as a
	// function that the file declares after the call. CalleeNamespace is
	// then a fallback key.
	Unbound bool
}

// CallGraphNode represents a single node in the call graph. Kind is the
// tree-sitter type of the node that defines it, and is empty when the build
// saw no definition. Position is 1-based.
type CallGraphNode struct {
	Namespace string
	Kind      string
	Position  ast.NodePosition
	CallsTo   []CallReference
}

// CallGraph is the call graph of one file. It keeps no tree, so it outlives
// the parse of the file.
type CallGraph struct {
	FileName string
	Language core.LanguageCode
	Nodes    map[string]*CallGraphNode
	RootNode *CallGraphNode

	assignmentGraph   assignmentGraph
	classConstructors map[string]bool

	// order holds the nodes in the order of the build, so DFS gives one
	// result for one file.
	order          []*CallGraphNode
	truncations    truncations
	unresolved     []UnresolvedCall
	unresolvedSeen map[UnresolvedCall]bool

	// ctx stops processChildren when it is done, and source gives the text
	// of a call site. build sets them for the build and clears them after,
	// so the graph keeps neither.
	ctx     context.Context
	source  []byte
	visited int
	ctxErr  error
}

// Dump writes the assignment graph and the call graph to w, in the order of
// the keys. It leaves out the builtins that have no edge.
func (cg *CallGraph) Dump(w io.Writer) error {
	builtins := getBuiltinsMap(cg.Language)
	var out strings.Builder

	out.WriteString("Assignment Graph:\n")
	for _, namespace := range slices.Sorted(maps.Keys(cg.assignmentGraph.Assignments)) {
		node := cg.assignmentGraph.Assignments[namespace]
		if builtins[namespace] && len(node.AssignedTo) == 0 {
			continue
		}
		fmt.Fprintf(&out, "  %s (->%d) => %v\n", namespace, len(node.AssignedBy), node.AssignedTo)
	}

	out.WriteString("\nCall Graph:\n")
	for _, namespace := range slices.Sorted(maps.Keys(cg.Nodes)) {
		node := cg.Nodes[namespace]
		if builtins[namespace] && len(node.CallsTo) == 0 {
			continue
		}
		callees := make([]string, len(node.CallsTo))
		for i, ref := range node.CallsTo {
			callees[i] = ref.CalleeNamespace
		}
		fmt.Fprintf(&out, "  %s (calls)=> %v\n", namespace, callees)
	}
	out.WriteString("\n")

	_, err := io.WriteString(w, out.String())
	return err
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

// DfsResultItem is one node that DFS reached. Site and Text give the call
// that reached it, and are empty for a node that DFS starts from.
type DfsResultItem struct {
	Namespace string
	Node      *CallGraphNode
	Caller    *CallGraphNode
	Site      ast.NodePosition
	Text      string
	Arguments []CallArgument
	Depth     int
	Terminal  bool
}

func (dri DfsResultItem) ToString() string {
	argNamespacesStr := ""
	for _, arg := range dri.Arguments {
		argNamespaces := make([]string, len(arg.Values))
		for i, value := range arg.Values {
			argNamespaces[i] = value.Namespace
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
	cg.dfsUtil(cg.FileName, cg.RootNode, CallReference{Arguments: []CallArgument{}}, visited, &dfsResult, 0, &resultCount)

	// Assumption - All functions and class constructors are reachable
	// This is required because most files only expose their classes/functions
	// which are imported and used by other files, so an entrypoint may not be
	// present in every file.
	for _, node := range cg.order {
		if resultCount >= maxDFSResultItems {
			break
		}

		if dfsSourceNodeTypes[node.Kind] && !visited[node.Namespace] {
			cg.dfsUtil(node.Namespace, cg.RootNode, CallReference{Arguments: []CallArgument{}}, visited, &dfsResult, 0, &resultCount)
		}
	}

	if resultCount >= maxDFSResultItems {
		cg.truncate(TruncationDFSResults, cg.FileName, nil)
	}

	return dfsResult
}

// dfsUtil visits namespace, which call reaches from caller.
func (cg *CallGraph) dfsUtil(namespace string, caller *CallGraphNode, call CallReference, visited map[string]bool, result *[]DfsResultItem, depth int, resultCount *int) {
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
					Namespace: terminalAssignmentNode.Namespace,
					Node:      terminalCallgraphNode,
					Caller:    caller,
					Site:      call.Site,
					Text:      call.Text,
					Arguments: call.Arguments,
					Depth:     depth,
					Terminal:  true,
				})
				*resultCount++
			}
		}
		return
	}

	// Mark the current node as visited and add it to the result
	visited[namespace] = true
	*result = append(*result, DfsResultItem{
		Namespace: namespace,
		Node:      callgraphNode,
		Caller:    caller,
		Site:      call.Site,
		Text:      call.Text,
		Arguments: call.Arguments,
		Depth:     depth,
		Terminal:  !callgraphNodeExists || len(callgraphNode.CallsTo) == 0,
	})
	*resultCount++

	assignmentGraphNode, assignmentNodeExists := cg.assignmentGraph.Assignments[namespace]
	if assignmentNodeExists {
		// Recursively visit all the nodes assigned to the current node
		for _, assigned := range assignmentGraphNode.AssignedTo {
			if *resultCount >= maxDFSResultItems {
				return
			}
			cg.dfsUtil(assigned, caller, call, visited, result, depth, resultCount)
		}
	}

	// Recursively visit all the nodes called by the current node
	// Any variable assignment would be ignored here, since it won't be in callgraph
	if callgraphNodeExists {
		for _, callRef := range callgraphNode.CallsTo {
			if *resultCount >= maxDFSResultItems {
				return
			}
			cg.dfsUtil(callRef.CalleeNamespace, callgraphNode, callRef, visited, result, depth+1, resultCount)
		}
	}
}

func (cg *CallGraph) getInstanceKeyword() (string, bool) {
	keyword, exists := instanceKeywordMapping[string(cg.Language)]
	return keyword, exists
}
