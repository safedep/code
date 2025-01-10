package callgraph

type AssignmentGraph struct {
	Assignments map[string][]string // Map of identifier to possible namespaces or other identifiers
}

func NewAssignmentGraph() *AssignmentGraph {
	return &AssignmentGraph{Assignments: make(map[string][]string)}
}

// Add an assignment
func (ag *AssignmentGraph) AddAssignment(identifier string, target string) {
	ag.Assignments[identifier] = append(ag.Assignments[identifier], target)
}

// Resolve an identifier to its targets
func (ag *AssignmentGraph) Resolve(identifier string) []string {
	return ag.Assignments[identifier]
}
