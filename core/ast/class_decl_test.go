package ast

import (
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/stretchr/testify/assert"
)

func TestNewClassDeclarationNode(t *testing.T) {
	content := ToContent([]byte("class TestClass: pass"))
	node := NewClassDeclarationNode(content)

	assert.NotNil(t, node, "Expected non-nil ClassDeclarationNode")

	// Test default values
	assert.False(t, node.IsAbstract(), "Expected new class to not be abstract by default")
	assert.Equal(t, AccessModifierPublic, node.AccessModifier(), "Expected default access modifier to be public")
	assert.False(t, node.HasInheritance(), "Expected new class to have no inheritance by default")

	// Test empty collections
	assert.Empty(t, node.BaseClasses(), "Expected empty base classes list")
	assert.Empty(t, node.Methods(), "Expected empty methods list")
	assert.Empty(t, node.Fields(), "Expected empty fields list")
	assert.Empty(t, node.Decorators(), "Expected empty decorators list")
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
		assert.Equal(t, test.expected, string(test.modifier), "Access modifier string representation should match expected value")
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
	assert.Equal(t, mockClassNode, node.GetClassNameNode(), "Class name node setter/getter should work correctly")

	// Test base class nodes
	node.AddBaseClassNode(mockBaseClassNode)
	baseClassNodes := node.GetBaseClassNodes()
	assert.Len(t, baseClassNodes, 1, "Should have one base class node after addition")
	assert.Equal(t, mockBaseClassNode, baseClassNodes[0], "Base class node should match the added node")

	node.SetBaseClassNodes([]*sitter.Node{mockBaseClassNode, mockBaseClassNode})
	assert.Len(t, node.GetBaseClassNodes(), 2, "Should have two base class nodes after setting")

	// Test inheritance detection
	assert.True(t, node.HasInheritance(), "Expected class to have inheritance after adding base class")

	// Test method nodes
	node.AddMethodNode(mockMethodNode)
	methodNodes := node.GetMethodNodes()
	assert.Len(t, methodNodes, 1, "Should have one method node after addition")
	assert.Equal(t, mockMethodNode, methodNodes[0], "Method node should match the added node")

	node.SetMethodNodes([]*sitter.Node{mockMethodNode, mockMethodNode})
	assert.Len(t, node.GetMethodNodes(), 2, "Should have two method nodes after setting")

	// Test field nodes
	node.AddFieldNode(mockFieldNode)
	fieldNodes := node.GetFieldNodes()
	assert.Len(t, fieldNodes, 1, "Should have one field node after addition")
	assert.Equal(t, mockFieldNode, fieldNodes[0], "Field node should match the added node")

	node.SetFieldNodes([]*sitter.Node{mockFieldNode, mockFieldNode})
	assert.Len(t, node.GetFieldNodes(), 2, "Should have two field nodes after setting")

	// Test constructor node
	node.SetConstructorNode(mockConstructorNode)
	assert.Equal(t, mockConstructorNode, node.GetConstructorNode(), "Constructor node setter/getter should work correctly")

	// Test decorator nodes
	node.AddDecoratorNode(mockDecoratorNode)
	decoratorNodes := node.GetDecoratorNodes()
	assert.Len(t, decoratorNodes, 1, "Should have one decorator node after addition")
	assert.Equal(t, mockDecoratorNode, decoratorNodes[0], "Decorator node should match the added node")

	node.SetDecoratorNodes([]*sitter.Node{mockDecoratorNode, mockDecoratorNode})
	assert.Len(t, node.GetDecoratorNodes(), 2, "Should have two decorator nodes after setting")

	// Test abstract flag
	node.SetIsAbstract(true)
	assert.True(t, node.IsAbstract(), "Abstract flag setter should work correctly")

	// Test access modifier
	node.SetAccessModifier(AccessModifierPrivate)
	assert.Equal(t, AccessModifierPrivate, node.AccessModifier(), "Access modifier setter should work correctly")
}

func TestClassDeclarationNodeStringMethod(t *testing.T) {
	content := ToContent([]byte("class TestClass: pass"))
	node := NewClassDeclarationNode(content)

	// Test basic string representation
	str := node.String()
	assert.NotEmpty(t, str, "String method should not return empty string")

	// Should contain class information
	expectedSubstrings := []string{"ClassDeclarationNode", "class:", "methods:", "fields:", "constructor:"}
	for _, substring := range expectedSubstrings {
		assert.Contains(t, str, substring, "String representation should contain expected substring")
	}

	// Test with abstract class
	node.SetIsAbstract(true)
	str = node.String()
	assert.Contains(t, str, "abstract", "String representation should contain 'abstract' for abstract class")

	// Test with private access modifier
	node.SetAccessModifier(AccessModifierPrivate)
	str = node.String()
	assert.Contains(t, str, "private", "String representation should contain 'private' for private class")
}

func TestClassDeclarationNodeContentMethods(t *testing.T) {
	// This test verifies that content methods work correctly when nodes are nil
	content := ToContent([]byte(""))
	node := NewClassDeclarationNode(content)

	// Test with nil nodes (should return empty strings/slices)
	assert.Empty(t, node.ClassName(), "Expected empty class name for nil class name node")
	assert.Empty(t, node.BaseClasses(), "Expected empty base classes for no base class nodes")
	assert.Empty(t, node.Methods(), "Expected empty methods for no method nodes")
	assert.Empty(t, node.Fields(), "Expected empty fields for no field nodes")
	assert.Empty(t, node.Constructor(), "Expected empty constructor for nil constructor node")
	assert.Empty(t, node.Decorators(), "Expected empty decorators for no decorator nodes")
}
