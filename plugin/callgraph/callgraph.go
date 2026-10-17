package callgraph

import (
	"context"
	"fmt"
	"io"
	"maps"
	"slices"

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

// CallKind tells what makes the edge of a call reference.
type CallKind string

const (
	// CallCall is a call in the source, as f(x) or obj.m(x).
	CallCall CallKind = "call"

	// CallNew is an object creation, as new Client(x).
	CallNew CallKind = "new"

	// CallImport is a dynamic import of a module that a string names, as
	// import('x'). The callee is x//*.
	CallImport CallKind = "import"

	// CallWildcardImport is the edge from the root to module//* for a
	// wildcard import, as from os import *.
	CallWildcardImport CallKind = "wildcard-import"

	// CallSynthetic is an edge that no call in the source makes: from the
	// instance to a method, from a class to its constructor method, and
	// from the root to the entry function or to each Go function. It has
	// no site.
	CallSynthetic CallKind = "synthetic"
)

// CallReference is one edge of the call graph. Site is the position of the
// expression that names the callee, and Text its source, at most
// maxSiteText bytes.
type CallReference struct {
	// CalleeNamespace is the callee as the call writes it, as the local name
	// load after import { load } from 'js-yaml'.
	CalleeNamespace string

	Kind CallKind

	// Targets are the namespaces that the assignment graph resolves the
	// callee to at the end of the build, sorted, as js-yaml//load for load.
	// A callee with no assignment is its own target. An unbound callee that
	// the assignment graph does not know has no target.
	Targets []string

	Site ast.NodePosition
	Text string

	// Receiver is the source of the object of a member call, as client in
	// client.send(), at most maxSiteText bytes. Method is the called name,
	// send. Receiver is empty for a bare call, and Method is empty when the
	// call computes the name.
	Receiver string
	Method   string

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

	// Imports are the imports of the file, in source order.
	Imports []Import

	profile           *languageProfile
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
	builtins := cg.languageProfile().builtinSet()

	if _, err := io.WriteString(w, "Assignment Graph:\n"); err != nil {
		return err
	}
	for _, namespace := range slices.Sorted(maps.Keys(cg.assignmentGraph.Assignments)) {
		node := cg.assignmentGraph.Assignments[namespace]
		if builtins[namespace] && len(node.AssignedTo) == 0 {
			continue
		}
		if _, err := fmt.Fprintf(w, "  %s (->%d) => %v\n", namespace, len(node.AssignedBy), node.AssignedTo); err != nil {
			return err
		}
	}

	if _, err := io.WriteString(w, "\nCall Graph:\n"); err != nil {
		return err
	}
	for _, namespace := range slices.Sorted(maps.Keys(cg.Nodes)) {
		node := cg.Nodes[namespace]
		if builtins[namespace] && len(node.CallsTo) == 0 {
			continue
		}
		callees := make([]string, len(node.CallsTo))
		for i, ref := range node.CallsTo {
			callees[i] = ref.CalleeNamespace
		}
		if _, err := fmt.Fprintf(w, "  %s (calls)=> %v\n", namespace, callees); err != nil {
			return err
		}
	}

	_, err := io.WriteString(w, "\n")
	return err
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

// DFS walks the graph from the root and then from each reachable
// definition that the walk from the root did not reach. A file often
// exports its definitions and does not call them.
func (cg *CallGraph) DFS() []DfsResultItem {
	visited := make(map[string]bool)
	var dfsResult []DfsResultItem
	resultCount := 0

	cg.dfsUtil(cg.FileName, cg.RootNode, CallReference{Arguments: []CallArgument{}}, visited, &dfsResult, 0, &resultCount)

	reachable := cg.languageProfile().reachable
	for _, node := range cg.nodesInOrder() {
		if resultCount >= maxDFSResultItems {
			break
		}

		if reachable[node.Kind] && !visited[node.Namespace] {
			cg.dfsUtil(node.Namespace, cg.RootNode, CallReference{Arguments: []CallArgument{}}, visited, &dfsResult, 0, &resultCount)
		}
	}

	if resultCount >= maxDFSResultItems {
		cg.truncate(TruncationDFSResults, cg.FileName, nil)
	}

	return dfsResult
}

// nodesInOrder returns the nodes in the order of the build. A decoded graph
// has no build order, so its nodes come in the order of their keys.
func (cg *CallGraph) nodesInOrder() []*CallGraphNode {
	if len(cg.order) > 0 {
		return cg.order
	}
	nodes := make([]*CallGraphNode, 0, len(cg.Nodes))
	for _, namespace := range slices.Sorted(maps.Keys(cg.Nodes)) {
		nodes = append(nodes, cg.Nodes[namespace])
	}
	return nodes
}

// dfsUtil visits namespace, which call reaches from caller.
func (cg *CallGraph) dfsUtil(namespace string, caller *CallGraphNode, call CallReference, visited map[string]bool, result *[]DfsResultItem, depth int, resultCount *int) {
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
		for _, assigned := range assignmentGraphNode.AssignedTo {
			if *resultCount >= maxDFSResultItems {
				return
			}
			cg.dfsUtil(assigned, caller, call, visited, result, depth, resultCount)
		}
	}

	if callgraphNodeExists {
		for _, callRef := range callgraphNode.CallsTo {
			if *resultCount >= maxDFSResultItems {
				return
			}
			cg.dfsUtil(callRef.CalleeNamespace, callgraphNode, callRef, visited, result, depth+1, resultCount)
		}
	}
}
