package callgraph

import (
	"testing"
)

// TestAssignmentGraphCycleDetection tests that cycle detection works correctly
// and that inProgress map is properly cleaned up
func TestAssignmentGraphCycleDetection(t *testing.T) {
	tests := []struct {
		name        string
		assignments map[string][]string // identifier -> assignedTo list
		resolveKey  string
		wantLeaves  []string // expected leaf namespaces
	}{
		{
			name: "simple_cycle_A_to_B_to_A",
			assignments: map[string][]string{
				"A": {"B"},
				"B": {"A"},
			},
			resolveKey: "A",
			wantLeaves: []string{"A"}, // Cycle detected at A (first node in recursion path)
		},
		{
			name: "three_node_cycle",
			assignments: map[string][]string{
				"A": {"B"},
				"B": {"C"},
				"C": {"A"},
			},
			resolveKey: "A",
			wantLeaves: []string{"A"}, // First cycle detection point
		},
		{
			name: "cycle_with_branch",
			assignments: map[string][]string{
				"A": {"B"},
				"B": {"C", "D"},
				"C": {"B"}, // Cycle: B -> C -> B
				"D": {},    // Leaf
			},
			resolveKey: "A",
			wantLeaves: []string{"B", "D"}, // B is cycle point, D is true leaf
		},
		{
			name: "self_cycle",
			assignments: map[string][]string{
				"A": {"A"}, // Self-referential
			},
			resolveKey: "A",
			wantLeaves: []string{"A"},
		},
		{
			name: "no_cycle_simple_chain",
			assignments: map[string][]string{
				"A": {"B"},
				"B": {"C"},
				"C": {},
			},
			resolveKey: "A",
			wantLeaves: []string{"C"},
		},
		{
			name: "multiple_resolves_same_graph",
			assignments: map[string][]string{
				"A": {"B"},
				"B": {"A"},
			},
			resolveKey: "B", // Resolve from different starting point
			wantLeaves: []string{"B"}, // Cycle detected at B when resolving from B
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ag := newAssignmentGraph()

			// Build the assignment graph
			for identifier, targets := range tt.assignments {
				for _, target := range targets {
					ag.addAssignment(identifier, nil, target, nil)
				}
			}

			// Resolve from the specified key
			resolved := ag.resolve(tt.resolveKey)

			// Check that we got the expected number of leaves
			if len(resolved) != len(tt.wantLeaves) {
				t.Errorf("resolve(%s) returned %d leaves, want %d", tt.resolveKey, len(resolved), len(tt.wantLeaves))
				t.Logf("Got leaves: %v", getNamespaces(resolved))
				t.Logf("Want leaves: %v", tt.wantLeaves)
			}

			// Check that all expected leaves are present
			gotNamespaces := make(map[string]bool)
			for _, node := range resolved {
				gotNamespaces[node.Namespace] = true
			}

			for _, wantNs := range tt.wantLeaves {
				if !gotNamespaces[wantNs] {
					t.Errorf("resolve(%s) missing expected leaf %s", tt.resolveKey, wantNs)
				}
			}

			// Test that multiple resolves work correctly (cache + no dangling inProgress)
			resolved2 := ag.resolve(tt.resolveKey)
			if len(resolved2) != len(resolved) {
				t.Errorf("Second resolve(%s) returned different result count: %d vs %d",
					tt.resolveKey, len(resolved2), len(resolved))
			}
		})
	}
}

// TestAssignmentGraphInProgressCleanup specifically tests that inProgress map
// doesn't have dangling entries that could cause issues
func TestAssignmentGraphInProgressCleanup(t *testing.T) {
	ag := newAssignmentGraph()

	// Create a complex cycle scenario
	// A -> B -> C -> B (cycle)
	// A -> D -> E (no cycle)
	ag.addAssignment("A", nil, "B", nil)
	ag.addAssignment("A", nil, "D", nil)
	ag.addAssignment("B", nil, "C", nil)
	ag.addAssignment("C", nil, "B", nil) // Cycle
	ag.addAssignment("D", nil, "E", nil)

	// First resolve - should work
	resolved1 := ag.resolve("A")
	if len(resolved1) == 0 {
		t.Fatal("First resolve returned no results")
	}

	// Second resolve - if inProgress had dangling entries, this might behave differently
	resolved2 := ag.resolve("A")
	if len(resolved2) != len(resolved1) {
		t.Errorf("Second resolve returned different count: %d vs %d", len(resolved2), len(resolved1))
	}

	// Resolve from different starting point - should also work
	resolved3 := ag.resolve("B")
	if len(resolved3) == 0 {
		t.Fatal("Resolve from B returned no results")
	}

	// Cache should work - resolve again
	resolved4 := ag.resolve("B")
	if len(resolved4) != len(resolved3) {
		t.Errorf("Cached resolve returned different count: %d vs %d", len(resolved4), len(resolved3))
	}
}

func getNamespaces(nodes []*assignmentNode) []string {
	result := make([]string, len(nodes))
	for i, node := range nodes {
		result[i] = node.Namespace
	}
	return result
}
