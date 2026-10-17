package callgraph

import (
	"slices"

	sitter "github.com/smacker/go-tree-sitter"
)

type assignmentNode struct {
	Namespace  string
	AssignedTo []string
	AssignedBy []string
	syntax
}

func newAssignmentGraphNode(namespace string, treeNode *sitter.Node) *assignmentNode {
	return &assignmentNode{
		Namespace:  namespace,
		AssignedTo: []string{},
		AssignedBy: []string{},
		syntax:     syntaxOf(treeNode),
	}
}

// assignmentGraph maps each name to the names and namespaces that the file
// assigns to it.
type assignmentGraph struct {
	Assignments map[string]*assignmentNode

	// resolveCache holds the results of resolve. An edge can change the
	// result of every name that reaches it, so a new edge empties the cache.
	resolveCache map[string][]*assignmentNode

	// truncations gets the record of the node limit. It can be nil.
	truncations *truncations

	// literals are the node types of a literal value in the language.
	literals map[string]bool
}

func newAssignmentGraph(truncations *truncations, literals map[string]bool) *assignmentGraph {
	return &assignmentGraph{
		Assignments: make(map[string]*assignmentNode),
		truncations: truncations,
		literals:    literals,
	}
}

func (ag *assignmentGraph) argumentOf(nodes []*assignmentNode) CallArgument {
	values := make([]ArgumentValue, len(nodes))
	for i, node := range nodes {
		values[i] = ArgumentValue{Namespace: node.Namespace, Literal: ag.literals[node.kind]}
	}
	return CallArgument{Values: values}
}

// addNode adds a node for identifier, or returns the node that exists. Past
// maxAssignmentGraphNodes it adds no node and returns nil.
func (ag *assignmentGraph) addNode(identifier string, treeNode *sitter.Node) *assignmentNode {
	if node, exists := ag.Assignments[identifier]; exists {
		if !node.known() {
			node.syntax = syntaxOf(treeNode)
		}
		return node
	}

	if len(ag.Assignments) >= maxAssignmentGraphNodes {
		ag.truncations.add(TruncationAssignmentNodes, identifier, treeNode)
		return nil
	}

	node := newAssignmentGraphNode(identifier, treeNode)
	ag.Assignments[identifier] = node
	delete(ag.resolveCache, identifier)
	return node
}

// addAssignment adds the edge from identifier to target.
func (ag *assignmentGraph) addAssignment(identifier string, identifierTreeNode *sitter.Node, target string, targetTreeNode *sitter.Node) {
	assignee := ag.addNode(identifier, identifierTreeNode)
	assigned := ag.addNode(target, targetTreeNode)
	if assignee == nil || assigned == nil {
		return
	}

	if !slices.Contains(assignee.AssignedTo, target) {
		assignee.AssignedTo = append(assignee.AssignedTo, target)
		clear(ag.resolveCache)
	}
	if !slices.Contains(assigned.AssignedBy, identifier) {
		assigned.AssignedBy = append(assigned.AssignedBy, identifier)
	}
}

// resolve returns the terminal values of identifier. After a = b, b = c
// and b = d, a resolves to c and d. A name in a cycle is a terminal value.
func (ag *assignmentGraph) resolve(identifier string) []*assignmentNode {
	if cached, exists := ag.resolveCache[identifier]; exists {
		return cached
	}
	if ag.resolveCache == nil {
		ag.resolveCache = make(map[string][]*assignmentNode)
	}

	targets := []*assignmentNode{}
	ag.resolveUtil(identifier, map[string]bool{}, map[string]bool{}, &targets)
	ag.resolveCache[identifier] = targets
	return targets
}

// resolveUtil adds the terminal values of identifier to targets. inProgress
// holds the names on the current path. A name that the path reaches again
// closes a cycle, and it is a terminal value.
func (ag *assignmentGraph) resolveUtil(identifier string, visited, inProgress map[string]bool, targets *[]*assignmentNode) {
	node, exists := ag.Assignments[identifier]
	switch {
	case !exists:
		return
	case inProgress[identifier]:
		*targets = append(*targets, node)
		return
	case visited[identifier]:
		return
	}

	visited[identifier] = true
	if len(node.AssignedTo) == 0 {
		*targets = append(*targets, node)
		return
	}

	inProgress[identifier] = true
	for _, target := range node.AssignedTo {
		ag.resolveUtil(target, visited, inProgress, targets)
	}
	delete(inProgress, identifier)
}
