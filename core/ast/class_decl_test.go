package ast

import (
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
)

func TestNewClassDeclarationNode(t *testing.T) {
	content := ToContent([]byte("class TestClass: pass"))
	node := NewClassDeclarationNode(content)

	if node == nil {
		t.Fatal("Expected non-nil ClassDeclarationNode")
	}

	// Test default values
	if node.IsAbstract() {
		t.Error("Expected new class to not be abstract by default")
	}

	if node.AccessModifier() != AccessModifierPublic {
		t.Errorf("Expected default access modifier to be public, got %s", node.AccessModifier())
	}

	if node.HasInheritance() {
		t.Error("Expected new class to have no inheritance by default")
	}

	// Test empty collections
	if len(node.BaseClasses()) != 0 {
		t.Error("Expected empty base classes list")
	}

	if len(node.Methods()) != 0 {
		t.Error("Expected empty methods list")
	}

	if len(node.Fields()) != 0 {
		t.Error("Expected empty fields list")
	}

	if len(node.Decorators()) != 0 {
		t.Error("Expected empty decorators list")
	}
}

func TestAccessModifierConstants(t *testing.T) {
	tests := []struct {
		modifier AccessModifier
		expected string
	}{
		{AccessModifierPublic, "public"},
		{AccessModifierPrivate, "private"},
		{AccessModifierProtected, "protected"},
		{AccessModifierPackage, "package"},
		{AccessModifierUnknown, "unknown"},
	}

	for _, test := range tests {
		if string(test.modifier) != test.expected {
			t.Errorf("Expected %s, got %s", test.expected, string(test.modifier))
		}
	}
}

func TestClassDeclarationNodeSettersAndGetters(t *testing.T) {
	content := ToContent([]byte("class TestClass: pass"))
	node := NewClassDeclarationNode(content)

	// Create mock sitter nodes for testing
	mockClassNode := &sitter.Node{}
	mockBaseClassNode := &sitter.Node{}
	mockMethodNode := &sitter.Node{}
	mockFieldNode := &sitter.Node{}
	mockConstructorNode := &sitter.Node{}
	mockDecoratorNode := &sitter.Node{}

	// Test class name node
	node.SetClassNameNode(mockClassNode)
	if node.GetClassNameNode() != mockClassNode {
		t.Error("Class name node setter/getter failed")
	}

	// Test base class nodes
	node.AddBaseClassNode(mockBaseClassNode)
	baseClassNodes := node.GetBaseClassNodes()
	if len(baseClassNodes) != 1 || baseClassNodes[0] != mockBaseClassNode {
		t.Error("Base class node addition failed")
	}

	node.SetBaseClassNodes([]*sitter.Node{mockBaseClassNode, mockBaseClassNode})
	if len(node.GetBaseClassNodes()) != 2 {
		t.Error("Base class nodes setter failed")
	}

	// Test inheritance detection
	if !node.HasInheritance() {
		t.Error("Expected class to have inheritance after adding base class")
	}

	// Test method nodes
	node.AddMethodNode(mockMethodNode)
	methodNodes := node.GetMethodNodes()
	if len(methodNodes) != 1 || methodNodes[0] != mockMethodNode {
		t.Error("Method node addition failed")
	}

	node.SetMethodNodes([]*sitter.Node{mockMethodNode, mockMethodNode})
	if len(node.GetMethodNodes()) != 2 {
		t.Error("Method nodes setter failed")
	}

	// Test field nodes
	node.AddFieldNode(mockFieldNode)
	fieldNodes := node.GetFieldNodes()
	if len(fieldNodes) != 1 || fieldNodes[0] != mockFieldNode {
		t.Error("Field node addition failed")
	}

	node.SetFieldNodes([]*sitter.Node{mockFieldNode, mockFieldNode})
	if len(node.GetFieldNodes()) != 2 {
		t.Error("Field nodes setter failed")
	}

	// Test constructor node
	node.SetConstructorNode(mockConstructorNode)
	if node.GetConstructorNode() != mockConstructorNode {
		t.Error("Constructor node setter/getter failed")
	}

	// Test decorator nodes
	node.AddDecoratorNode(mockDecoratorNode)
	decoratorNodes := node.GetDecoratorNodes()
	if len(decoratorNodes) != 1 || decoratorNodes[0] != mockDecoratorNode {
		t.Error("Decorator node addition failed")
	}

	node.SetDecoratorNodes([]*sitter.Node{mockDecoratorNode, mockDecoratorNode})
	if len(node.GetDecoratorNodes()) != 2 {
		t.Error("Decorator nodes setter failed")
	}

	// Test abstract flag
	node.SetIsAbstract(true)
	if !node.IsAbstract() {
		t.Error("Abstract flag setter failed")
	}

	// Test access modifier
	node.SetAccessModifier(AccessModifierPrivate)
	if node.AccessModifier() != AccessModifierPrivate {
		t.Error("Access modifier setter failed")
	}
}

func TestClassDeclarationNodeStringMethod(t *testing.T) {
	content := ToContent([]byte("class TestClass: pass"))
	node := NewClassDeclarationNode(content)

	// Test basic string representation
	str := node.String()
	if str == "" {
		t.Error("String method should not return empty string")
	}

	// Should contain class information
	expectedSubstrings := []string{"ClassDeclarationNode", "class:", "methods:", "fields:", "constructor:"}
	for _, substring := range expectedSubstrings {
		if !containsSubstring(str, substring) {
			t.Errorf("String representation should contain '%s', got: %s", substring, str)
		}
	}

	// Test with abstract class
	node.SetIsAbstract(true)
	str = node.String()
	if !containsSubstring(str, "abstract") {
		t.Errorf("String representation should contain 'abstract' for abstract class, got: %s", str)
	}

	// Test with private access modifier
	node.SetAccessModifier(AccessModifierPrivate)
	str = node.String()
	if !containsSubstring(str, "private") {
		t.Errorf("String representation should contain 'private' for private class, got: %s", str)
	}
}

func TestClassDeclarationNodeContentMethods(t *testing.T) {
	// This test verifies that content methods work correctly when nodes are nil
	content := ToContent([]byte(""))
	node := NewClassDeclarationNode(content)

	// Test with nil nodes (should return empty strings/slices)
	if node.ClassName() != "" {
		t.Error("Expected empty class name for nil class name node")
	}

	if len(node.BaseClasses()) != 0 {
		t.Error("Expected empty base classes for no base class nodes")
	}

	if len(node.Methods()) != 0 {
		t.Error("Expected empty methods for no method nodes")
	}

	if len(node.Fields()) != 0 {
		t.Error("Expected empty fields for no field nodes")
	}

	if node.Constructor() != "" {
		t.Error("Expected empty constructor for nil constructor node")
	}

	if len(node.Decorators()) != 0 {
		t.Error("Expected empty decorators for no decorator nodes")
	}
}

// Helper function to check if a string contains a substring
func containsSubstring(str, substr string) bool {
	return len(str) >= len(substr) && findSubstring(str, substr)
}

func findSubstring(str, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(str) < len(substr) {
		return false
	}
	for i := 0; i <= len(str)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if str[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
