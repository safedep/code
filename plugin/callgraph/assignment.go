package callgraph

import (
	"slices"

	"github.com/safedep/dry/utils"
	sitter "github.com/smacker/go-tree-sitter"
)

type assignmentNode struct {
	Namespace  string
	AssignedTo []string
	AssignedBy []string
	TreeNode   *sitter.Node
}

func newAssignmentGraphNode(namespace string, treeNode *sitter.Node) *assignmentNode {
	return &assignmentNode{
		Namespace:  namespace,
		AssignedTo: []string{},
		AssignedBy: []string{},
		TreeNode:   treeNode,
	}
}

func (an *assignmentNode) IsLiteralValue() bool {
	return an.TreeNode != nil && literalNodeTypes[an.TreeNode.Type()]
}

type assignmentGraph struct {
	Assignments map[string]*assignmentNode // Map of identifier to possible namespaces or other identifiers
}

func newAssignmentGraph() *assignmentGraph {
	return &assignmentGraph{Assignments: make(map[string]*assignmentNode)}
}

func (ag *assignmentGraph) addNode(identifier string, treeNode *sitter.Node) *assignmentNode {
	existingAssignmentNode, exists := ag.Assignments[identifier]

	if !exists {
		ag.Assignments[identifier] = newAssignmentGraphNode(identifier, treeNode)
	} else if treeNode != nil && existingAssignmentNode.TreeNode == nil {
		// If the existing node has no tree node, we can set it now
		ag.Assignments[identifier].TreeNode = treeNode
	}

	return ag.Assignments[identifier]
}

// Add an assignment
func (ag *assignmentGraph) addAssignment(identifier string, identifierTreeNode *sitter.Node, target string, targetTreeNode *sitter.Node) {
	if _, exists := ag.Assignments[identifier]; !exists {
		ag.Assignments[identifier] = newAssignmentGraphNode(identifier, identifierTreeNode)
	}
	if _, exists := ag.Assignments[target]; !exists {
		ag.Assignments[target] = newAssignmentGraphNode(target, targetTreeNode)
	}
	if !slices.Contains(ag.Assignments[identifier].AssignedTo, target) {
		ag.Assignments[identifier].AssignedTo = append(ag.Assignments[identifier].AssignedTo, target)
	}
	if !slices.Contains(ag.Assignments[target].AssignedBy, identifier) {
		ag.Assignments[target].AssignedBy = append(ag.Assignments[target].AssignedBy, identifier)
	}
}

// resolves an identifier to its assignment targets (leaf nodes of the DFS tree)
// For example, if a = b, b = c, b = d, then resolving a will return {c, d}
func (ag *assignmentGraph) resolve(identifier string) []*assignmentNode {
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
