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
	Assignments   map[string]*assignmentNode     // Map of identifier to possible namespaces or other identifiers
	nodeCount     int                            // Track number of assignment nodes
	limitExceeded bool                           // Flag when limit is exceeded
	resolveCache  map[string][]*assignmentNode   // Cache for resolve() results
}

func newAssignmentGraph() *assignmentGraph {
	return &assignmentGraph{
		Assignments:   make(map[string]*assignmentNode),
		nodeCount:     0,
		limitExceeded: false,
		resolveCache:  make(map[string][]*assignmentNode),
	}
}

func (ag *assignmentGraph) addNode(identifier string, treeNode *sitter.Node) *assignmentNode {
	existingAssignmentNode, exists := ag.Assignments[identifier]

	if !exists {
		// Check limit before adding new node
		if ag.nodeCount >= maxAssignmentGraphNodes && !ag.limitExceeded {
			// Log warning once when limit is first exceeded
			// We still add the node to prevent nil errors but mark as exceeded
			ag.limitExceeded = true
		}

		ag.Assignments[identifier] = newAssignmentGraphNode(identifier, treeNode)
		ag.nodeCount++
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
		ag.nodeCount++
	}
	if _, exists := ag.Assignments[target]; !exists {
		ag.Assignments[target] = newAssignmentGraphNode(target, targetTreeNode)
		ag.nodeCount++
	}
	if !slices.Contains(ag.Assignments[identifier].AssignedTo, target) {
		ag.Assignments[identifier].AssignedTo = append(ag.Assignments[identifier].AssignedTo, target)
		// Invalidate cache for this identifier since assignment changed
		delete(ag.resolveCache, identifier)
	}
	if !slices.Contains(ag.Assignments[target].AssignedBy, identifier) {
		ag.Assignments[target].AssignedBy = append(ag.Assignments[target].AssignedBy, identifier)
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
