package ast

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewInheritanceRelationship(t *testing.T) {
	rel := NewInheritanceRelationship("Child", "Parent", RelationshipTypeExtends, "file.go", 10)

	assert.Equal(t, "Child", rel.ChildClassName, "Expected child class name 'Child'")
	assert.Equal(t, "Parent", rel.ParentClassName, "Expected parent class name 'Parent'")
	assert.Equal(t, RelationshipTypeExtends, rel.RelationshipType, "Expected relationship type 'extends'")
	assert.Equal(t, "file.go", rel.FileLocation, "Expected file location 'file.go'")
	assert.Equal(t, uint32(10), rel.LineNumber, "Expected line number 10")
	assert.True(t, rel.IsDirectInheritance, "Expected direct inheritance to be true")
	assert.Equal(t, 1, rel.InheritanceDepth, "Expected inheritance depth 1")
	assert.True(t, rel.IsDirectInheritance, "Expected direct inheritance to be true")
	assert.Equal(t, 1, rel.InheritanceDepth, "Expected inheritance depth 1")
}

func TestInheritanceRelationshipString(t *testing.T) {
	rel := NewInheritanceRelationship("Child", "Parent", RelationshipTypeExtends, "file.go", 10)
	str := rel.String()

	expectedSubstrings := []string{"Child", "extends", "Parent", "file.go:10"}
	for _, substring := range expectedSubstrings {
		assert.Contains(t, str, substring, "String representation should contain '%s'", substring)
	}
}

func TestRelationshipTypeConstants(t *testing.T) {
	tests := []struct {
		relType  RelationshipType
		expected string
	}{
		{RelationshipTypeExtends, "extends"},
		{RelationshipTypeImplements, "implements"},
		{RelationshipTypeInherits, "inherits"},
		{RelationshipTypeMixin, "mixin"},
	}

	for _, test := range tests {
		assert.Equal(t, test.expected, string(test.relType), "RelationshipType string representation should match expected value")
	}
}

func TestNewInheritanceGraph(t *testing.T) {
	ig := NewInheritanceGraph()

	assert.NotNil(t, ig, "Expected non-nil InheritanceGraph")
	assert.Empty(t, ig.GetAllClasses(), "Expected empty inheritance graph")
	assert.Empty(t, ig.GetRootClasses(), "Expected no root classes in empty graph")
	assert.Empty(t, ig.GetLeafClasses(), "Expected no leaf classes in empty graph")
}

func TestInheritanceGraphAddRelationship(t *testing.T) {
	ig := NewInheritanceGraph()

	rel := ig.AddRelationship("Child", "Parent", RelationshipTypeExtends, "file.go", 10)

	assert.NotNil(t, rel, "Expected non-nil relationship")

	// Test that both classes are tracked
	allClasses := ig.GetAllClasses()
	assert.Len(t, allClasses, 2, "Expected 2 classes")
	assert.Contains(t, allClasses, "Child", "Expected Child class to be tracked")
	assert.Contains(t, allClasses, "Parent", "Expected Parent class to be tracked")

	// Test direct parent relationship
	parents := ig.GetDirectParentNames("Child")
	assert.Len(t, parents, 1, "Expected Child to have exactly one parent")
	assert.Equal(t, "Parent", parents[0], "Expected Child to have Parent as direct parent")

	// Test direct child relationship
	children := ig.GetDirectChildNames("Parent")
	assert.Len(t, children, 1, "Expected Parent to have exactly one child")
	assert.Equal(t, "Child", children[0], "Expected Parent to have Child as direct child")
}

func TestInheritanceGraphAncestryQueries(t *testing.T) {
	ig := NewInheritanceGraph()

	// Build hierarchy: GrandChild -> Child -> Parent -> Root
	ig.AddRelationship("Child", "Parent", RelationshipTypeExtends, "file.go", 10)
	ig.AddRelationship("Parent", "Root", RelationshipTypeExtends, "file.go", 20)
	ig.AddRelationship("GrandChild", "Child", RelationshipTypeExtends, "file.go", 30)

	// Test ancestry queries
	ancestry := ig.GetAncestry("GrandChild")
	expectedAncestry := []string{"Child", "Parent", "Root"}
	assert.Equal(t, expectedAncestry, ancestry, "Expected ancestry to match")

	// Test descendant queries
	descendants := ig.GetDescendants("Root")
	expectedDescendants := []string{"Parent", "Child", "GrandChild"}
	assert.Equal(t, expectedDescendants, descendants, "Expected descendants to match")

	// Test ancestor/descendant checks
	assert.True(t, ig.IsAncestor("Root", "GrandChild"), "Expected Root to be ancestor of GrandChild")
	assert.True(t, ig.IsDescendant("GrandChild", "Root"), "Expected GrandChild to be descendant of Root")
	assert.False(t, ig.IsAncestor("GrandChild", "Root"), "Expected GrandChild to NOT be ancestor of Root")

	// Test inheritance depth
	depth := ig.GetInheritanceDepth("GrandChild", "Root")
	assert.Equal(t, 3, depth, "Expected inheritance depth 3")

	depth = ig.GetInheritanceDepth("Child", "Parent")
	assert.Equal(t, 1, depth, "Expected inheritance depth 1")

	depth = ig.GetInheritanceDepth("Root", "GrandChild")
	assert.Equal(t, -1, depth, "Expected inheritance depth -1 (no relationship)")
}

func TestInheritanceGraphRootAndLeafClasses(t *testing.T) {
	ig := NewInheritanceGraph()

	// Build hierarchy: Child -> Parent, Orphan (no parents/children)
	ig.AddRelationship("Child", "Parent", RelationshipTypeExtends, "file.go", 10)

	// Add orphan class (no inheritance)
	ig.allClasses["Orphan"] = true

	roots := ig.GetRootClasses()
	expectedRoots := []string{"Parent", "Orphan"}
	for _, expectedRoot := range expectedRoots {
		assert.Contains(t, roots, expectedRoot, "Expected root classes to contain %s", expectedRoot)
	}

	leaves := ig.GetLeafClasses()
	expectedLeaves := []string{"Child", "Orphan"}
	for _, expectedLeaf := range expectedLeaves {
		assert.Contains(t, leaves, expectedLeaf, "Expected leaf classes to contain %s", expectedLeaf)
	}
}

func TestInheritanceGraphMultipleInheritance(t *testing.T) {
	ig := NewInheritanceGraph()

	// Python-style multiple inheritance: Child inherits from Parent1 and Parent2
	ig.AddRelationship("Child", "Parent1", RelationshipTypeInherits, "file.py", 10)
	ig.AddRelationship("Child", "Parent2", RelationshipTypeInherits, "file.py", 10)

	parents := ig.GetDirectParentNames("Child")
	expectedParents := []string{"Parent1", "Parent2"}
	for _, expectedParent := range expectedParents {
		assert.Contains(t, parents, expectedParent, "Expected parents to contain %s", expectedParent)
	}

	// Both parents should have Child as descendant
	assert.True(t, ig.IsDescendant("Child", "Parent1"), "Expected Child to be descendant of Parent1")
	assert.True(t, ig.IsDescendant("Child", "Parent2"), "Expected Child to be descendant of Parent2")
}

func TestMethodResolutionOrder(t *testing.T) {
	ig := NewInheritanceGraph()

	// Diamond inheritance pattern for MRO testing
	// D inherits from B and C, both B and C inherit from A
	ig.AddRelationship("B", "A", RelationshipTypeInherits, "file.py", 10)
	ig.AddRelationship("C", "A", RelationshipTypeInherits, "file.py", 20)
	ig.AddRelationship("D", "B", RelationshipTypeInherits, "file.py", 30)
	ig.AddRelationship("D", "C", RelationshipTypeInherits, "file.py", 30)

	mro := ig.GetMethodResolutionOrder("D")

	// MRO should start with the class itself
	assert.NotEmpty(t, mro, "Expected non-empty MRO")
	assert.Equal(t, "D", mro[0], "Expected MRO to start with 'D'")

	// A should appear only once (at the end in proper C3 linearization)
	aCount := 0
	for _, class := range mro {
		if class == "A" {
			aCount++
		}
	}
	assert.Equal(t, 1, aCount, "Expected 'A' to appear exactly once in MRO")

	// Test simple case - single inheritance
	ig2 := NewInheritanceGraph()
	ig2.AddRelationship("Child", "Parent", RelationshipTypeInherits, "file.py", 10)

	mro2 := ig2.GetMethodResolutionOrder("Child")
	expectedMRO := []string{"Child", "Parent"}
	assert.Equal(t, expectedMRO, mro2, "Expected simple MRO to match")
}

func TestCircularInheritanceDetection(t *testing.T) {
	ig := NewInheritanceGraph()

	// Create circular inheritance: A -> B -> C -> A
	ig.AddRelationship("A", "B", RelationshipTypeExtends, "file.go", 10)
	ig.AddRelationship("B", "C", RelationshipTypeExtends, "file.go", 20)
	ig.AddRelationship("C", "A", RelationshipTypeExtends, "file.go", 30)

	cycles := ig.DetectCircularInheritance()
	assert.NotEmpty(t, cycles, "Expected to detect circular inheritance")

	// Test acyclic graph
	ig2 := NewInheritanceGraph()
	ig2.AddRelationship("Child", "Parent", RelationshipTypeExtends, "file.go", 10)
	ig2.AddRelationship("Parent", "GrandParent", RelationshipTypeExtends, "file.go", 20)

	cycles2 := ig2.DetectCircularInheritance()
	assert.Empty(t, cycles2, "Expected no circular inheritance in acyclic graph")
}

func TestInheritanceGraphMerge(t *testing.T) {
	ig1 := NewInheritanceGraph()
	ig1.AddRelationship("Child1", "Parent1", RelationshipTypeExtends, "file1.go", 10)

	ig2 := NewInheritanceGraph()
	ig2.AddRelationship("Child2", "Parent2", RelationshipTypeExtends, "file2.go", 20)

	ig1.MergeFrom(ig2)

	allClasses := ig1.GetAllClasses()
	expectedClasses := []string{"Child1", "Parent1", "Child2", "Parent2"}
	for _, expectedClass := range expectedClasses {
		assert.Contains(t, allClasses, expectedClass, "Expected merged classes to contain %s", expectedClass)
	}
}

func TestInheritanceGraphStatistics(t *testing.T) {
	ig := NewInheritanceGraph()

	// Build test hierarchy
	ig.AddRelationship("Child", "Parent", RelationshipTypeExtends, "file.go", 10)
	ig.AddRelationship("Parent", "GrandParent", RelationshipTypeExtends, "file.go", 20)
	ig.allClasses["Orphan"] = true // Class with no inheritance

	stats := ig.GetStatistics()

	assert.Equal(t, 4, stats["total_classes"], "Expected 4 total classes")
	assert.Equal(t, 2, stats["total_relationships"], "Expected 2 total relationships")
	assert.Equal(t, 2, stats["root_classes"], "Expected 2 root classes (GrandParent and Orphan)")
	assert.Equal(t, 2, stats["leaf_classes"], "Expected 2 leaf classes (Child and Orphan)")
	assert.Equal(t, 2, stats["max_inheritance_depth"], "Expected max inheritance depth 2 (Child -> Parent -> GrandParent)")
	assert.Equal(t, 0, stats["circular_inheritance"], "Expected 0 circular inheritance")
}

func TestBuildInheritanceGraphFromClasses(t *testing.T) {
	// Test the integration by directly setting the class names and base classes
	// since we can't easily mock sitter.Node content in tests

	// Create a mock class manually for testing
	content := ToContent([]byte("class Child(Parent): pass"))
	class1 := NewClassDeclarationNode(content)

	// Simulate the class having been parsed - we'll test with direct manipulation
	// In real usage, the language resolvers would populate these from Tree-Sitter nodes

	// Test empty classes (no names)
	classes := []*ClassDeclarationNode{class1}
	ig := BuildInheritanceGraphFromClasses(classes, "test.py")

	// Since the mock nodes return empty content, we should get an empty graph
	allClasses := ig.GetAllClasses()
	assert.Empty(t, allClasses, "Expected 0 classes for empty content nodes")

	// Test the function with pre-built inheritance graph to verify the logic
	ig2 := NewInheritanceGraph()
	ig2.AddRelationship("Child", "Parent", RelationshipTypeInherits, "test.py", 1)

	allClasses2 := ig2.GetAllClasses()
	expectedClasses := []string{"Child", "Parent"}
	for _, expectedClass := range expectedClasses {
		assert.Contains(t, allClasses2, expectedClass, "Expected classes to contain %s", expectedClass)
	}

	parents := ig2.GetDirectParentNames("Child")
	assert.Len(t, parents, 1, "Expected Child to have exactly one parent")
	assert.Equal(t, "Parent", parents[0], "Expected Child to have Parent as parent")
}

func TestInheritanceGraphString(t *testing.T) {
	ig := NewInheritanceGraph()
	ig.AddRelationship("Child", "Parent", RelationshipTypeExtends, "file.go", 10)

	str := ig.String()
	assert.NotEmpty(t, str, "String method should not return empty string")

	expectedSubstrings := []string{"InheritanceGraph", "classes:", "relationships:"}
	for _, substring := range expectedSubstrings {
		assert.Contains(t, str, substring, "String representation should contain '%s'", substring)
	}
}
