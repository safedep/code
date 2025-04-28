package callgraph

import (
	"slices"

	"github.com/safedep/dry/utils"
	sitter "github.com/smacker/go-tree-sitter"
)

type assignmentNode struct {
	Namespace  string
	AssignedTo []string
	TreeNode   *sitter.Node
}

func newAssignmentGraphNode(namespace string, treeNode *sitter.Node) *assignmentNode {
	return &assignmentNode{
		Namespace:  namespace,
		AssignedTo: []string{},
		TreeNode:   treeNode,
	}
}

type assignmentGraph struct {
	Assignments map[string]*assignmentNode // Map of identifier to possible namespaces or other identifiers
}

func newAssignmentGraph() *assignmentGraph {
	return &assignmentGraph{Assignments: make(map[string]*assignmentNode)}
}

func (ag *assignmentGraph) AddIdentifier(identifier string, treeNode *sitter.Node) *assignmentNode {
	if _, exists := ag.Assignments[identifier]; !exists {
		ag.Assignments[identifier] = newAssignmentGraphNode(identifier, treeNode)
	}
	return ag.Assignments[identifier]
}

// Add an assignment
func (ag *assignmentGraph) AddAssignment(identifier string, identifierTreeNode *sitter.Node, target string, targetTreeNode *sitter.Node) {
	if _, exists := ag.Assignments[identifier]; !exists {
		ag.Assignments[identifier] = newAssignmentGraphNode(identifier, identifierTreeNode)
	}
	if _, exists := ag.Assignments[target]; !exists {
		ag.Assignments[target] = newAssignmentGraphNode(target, targetTreeNode)
	}
	if !slices.Contains(ag.Assignments[identifier].AssignedTo, target) {
		ag.Assignments[identifier].AssignedTo = append(ag.Assignments[identifier].AssignedTo, target)
	}
}

// Resolve an identifier to its targets (leaf nodes of the DFS tree)
func (ag *assignmentGraph) Resolve(identifier string) []*assignmentNode {
	targets := utils.PtrTo([]*assignmentNode{})
	visited := make(map[string]bool)
	ag.resolveUtil(identifier, visited, targets)
	return *targets
}

// Utility function to resolve the identifier to its targets recursively
func (ag *assignmentGraph) resolveUtil(currentIdentifier string, visited map[string]bool, targets *[]*assignmentNode) {
	if visited[currentIdentifier] {
		return
	}
	visited[currentIdentifier] = true

	identifierNode, exists := ag.Assignments[currentIdentifier]
	if !exists {
		return
	}

	// If the current identifier has no assignments, it's a leaf node
	if len(identifierNode.AssignedTo) == 0 {
		*targets = append(*targets, identifierNode)
		return
	}

	for _, targetIdentifier := range identifierNode.AssignedTo {
		ag.resolveUtil(targetIdentifier, visited, targets)
	}
}
