package callgraph

import (
	"slices"

	"github.com/safedep/code/pkg/helpers"
)

type AssignmentGraph struct {
	Assignments map[string][]string // Map of identifier to possible namespaces or other identifiers
}

func NewAssignmentGraph() *AssignmentGraph {
	return &AssignmentGraph{Assignments: make(map[string][]string)}
}

// Add an assignment
func (ag *AssignmentGraph) AddAssignment(identifier string, target string) {
	if _, exists := ag.Assignments[identifier]; !exists {
		ag.Assignments[identifier] = []string{}
	}
	if _, exists := ag.Assignments[target]; !exists {
		ag.Assignments[target] = []string{}
	}
	if !slices.Contains(ag.Assignments[identifier], target) {
		ag.Assignments[identifier] = append(ag.Assignments[identifier], target)
	}
}

func (ag *AssignmentGraph) resolveUtil(currentIdentifier string, visited map[string]bool, targets *[]string) {
	if visited[currentIdentifier] {
		return
	}
	visited[currentIdentifier] = true

	targetIdentifiers := ag.Assignments[currentIdentifier]

	// If the current identifier has no assignments, it's a leaf node
	if len(targetIdentifiers) == 0 {
		*targets = append(*targets, currentIdentifier)
		return
	}

	for _, target := range targetIdentifiers {
		ag.resolveUtil(target, visited, targets)
	}
}

// Resolve an identifier to its targets (leaf nodes of the DFS tree)
func (ag *AssignmentGraph) Resolve(identifier string) []string {
	targets := helpers.PtrTo(ag.Assignments[identifier])
	visited := make(map[string]bool)
	ag.resolveUtil(identifier, visited, targets)
	return *targets
}
