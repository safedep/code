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
	Assignments  map[string]*assignmentNode   // Map of identifier to possible namespaces or other identifiers
	resolveCache map[string][]*assignmentNode // Cache for resolve() results

	// truncations gets the record of the node limit. It can be nil.
	truncations *truncations
}

func newAssignmentGraph(truncations *truncations) *assignmentGraph {
	return &assignmentGraph{
		Assignments:  make(map[string]*assignmentNode),
		resolveCache: make(map[string][]*assignmentNode),
		truncations:  truncations,
	}
}

// addNode adds a node for identifier, or returns the node that exists. Past
// maxAssignmentGraphNodes it adds no node and returns nil.
func (ag *assignmentGraph) addNode(identifier string, treeNode *sitter.Node) *assignmentNode {
	if node, exists := ag.Assignments[identifier]; exists {
		if node.TreeNode == nil {
			node.TreeNode = treeNode
		}
		return node
	}

	if len(ag.Assignments) >= maxAssignmentGraphNodes {
		ag.truncations.add(TruncationAssignmentNodes, identifier, treeNode)
		return nil
	}

	node := newAssignmentGraphNode(identifier, treeNode)
	ag.Assignments[identifier] = node
	return node
}

// Add an assignment
func (ag *assignmentGraph) addAssignment(identifier string, identifierTreeNode *sitter.Node, target string, targetTreeNode *sitter.Node) {
	assignee := ag.addNode(identifier, identifierTreeNode)
	assigned := ag.addNode(target, targetTreeNode)
	if assignee == nil || assigned == nil {
		return
	}

	if !slices.Contains(assignee.AssignedTo, target) {
		assignee.AssignedTo = append(assignee.AssignedTo, target)
		// Invalidate cache for this identifier since assignment changed
		delete(ag.resolveCache, identifier)
	}
	if !slices.Contains(assigned.AssignedBy, identifier) {
		assigned.AssignedBy = append(assigned.AssignedBy, identifier)
	}
}

// resolves an identifier to its assignment targets (leaf nodes of the DFS tree)
// For example, if a = b, b = c, b = d, then resolving a will return {c, d}
// Detects cycles and returns the node itself if a cycle is encountered
// Results are cached to avoid recomputing expensive resolution chains
func (ag *assignmentGraph) resolve(identifier string) []*assignmentNode {
	// Check cache first
	if cached, exists := ag.resolveCache[identifier]; exists {
		return cached
	}

	targets := utils.PtrTo([]*assignmentNode{})
	visited := make(map[string]bool)
	inProgress := make(map[string]bool) // Track nodes currently being processed
	ag.resolveUtil(identifier, visited, inProgress, targets)

	// Cache the result
	ag.resolveCache[identifier] = *targets

	return *targets
}

// Utility function to resolve the identifier to its targets recursively
// Uses cycle detection: inProgress tracks nodes in current recursion path
func (ag *assignmentGraph) resolveUtil(currentIdentifier string, visited map[string]bool, inProgress map[string]bool, targets *[]*assignmentNode) {
	// Cycle detected: current node is already being processed in recursion stack
	if inProgress[currentIdentifier] {
		// Break the cycle by treating this as a leaf node
		identifierNode, exists := ag.Assignments[currentIdentifier]
		if exists {
			*targets = append(*targets, identifierNode)
		}
		return
	}

	// Already fully processed in a different branch - skip to avoid duplicates
	if visited[currentIdentifier] {
		return
	}

	// Mark as in progress before recursing
	inProgress[currentIdentifier] = true
	visited[currentIdentifier] = true

	identifierNode, exists := ag.Assignments[currentIdentifier]
	if !exists {
		delete(inProgress, currentIdentifier)
		return
	}

	// If the current identifier has no assignments, it's a leaf node
	if len(identifierNode.AssignedTo) == 0 {
		*targets = append(*targets, identifierNode)
		delete(inProgress, currentIdentifier)
		return
	}

	for _, targetIdentifier := range identifierNode.AssignedTo {
		ag.resolveUtil(targetIdentifier, visited, inProgress, targets)
	}

	// Remove from in-progress after processing all children
	delete(inProgress, currentIdentifier)
}
