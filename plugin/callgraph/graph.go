package callgraph

import (
	"fmt"
	"strings"
)

const namespaceSeparator = "//"

// graphNode represents a single node in the call graph
type graphNode struct {
	Namespace string
	CallsTo   []string
}

func newGraphNode(namespace string) *graphNode {
	return &graphNode{
		Namespace: namespace,
		CallsTo:   []string{},
	}
}

type CallGraph struct {
	FileName                     string
	Nodes                        map[string]*graphNode
	assignments                  AssignmentGraph
	importedIdentifierNamespaces map[string]string
}

func NewCallGraph(fileName string, importedIdentifierNamespaces map[string]string) *CallGraph {
	cg := &CallGraph{FileName: fileName, Nodes: make(map[string]*graphNode), assignments: *NewAssignmentGraph(), importedIdentifierNamespaces: importedIdentifierNamespaces}
	for identifier, namespace := range importedIdentifierNamespaces {
		cg.assignments.AddAssignment(identifier, namespace)
	}
	return cg
}

// AddEdge adds an edge from one function to another
func (cg *CallGraph) AddEdge(caller, callee string) {
	if _, exists := cg.Nodes[caller]; !exists {
		cg.Nodes[caller] = newGraphNode(caller)
	}
	if _, exists := cg.Nodes[callee]; !exists {
		cg.Nodes[callee] = newGraphNode(callee)
	}
	cg.Nodes[caller].CallsTo = append(cg.Nodes[caller].CallsTo, callee)
}

func (cg *CallGraph) PrintCallGraph() {
	fmt.Println("Call Graph:")
	for caller, node := range cg.Nodes {
		fmt.Printf("  %s (calls)=> %v\n", caller, node.CallsTo)
	}
	fmt.Println()
}

func (cg *CallGraph) DFS() []string {
	visited := make(map[string]bool)
	var dfsResult []string
	cg.dfsUtil(cg.FileName, visited, &dfsResult, 0)
	return dfsResult
}

func (cg *CallGraph) dfsUtil(startNode string, visited map[string]bool, result *[]string, depth int) {
	if visited[startNode] {
		// append that not going inside this on prev level
		*result = append(*result, fmt.Sprintf("%s Stopped at %s", strings.Repeat("|", depth), startNode))
		return
	}

	// Mark the current node as visited and add it to the result
	visited[startNode] = true
	*result = append(*result, fmt.Sprintf("%s %s", strings.Repeat(">", depth), startNode))

	// Recursively visit all the nodes called by the current node
	for _, callee := range cg.Nodes[startNode].CallsTo {
		cg.dfsUtil(callee, visited, result, depth+1)
	}
}
