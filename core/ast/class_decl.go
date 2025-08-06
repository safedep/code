package ast

import (
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

// AccessModifier represents the access level of class members
type AccessModifier string

const (
	// AccessModifierPublic represents public access (default in most languages)
	AccessModifierPublic AccessModifier = "public"

	// AccessModifierPrivate represents private access
	AccessModifierPrivate AccessModifier = "private"

	// AccessModifierProtected represents protected access (inheritance-accessible)
	AccessModifierProtected AccessModifier = "protected"

	// AccessModifierPackage represents package/internal access
	AccessModifierPackage AccessModifier = "package"

	// AccessModifierUnknown represents unknown or unsupported access level
	AccessModifierUnknown AccessModifier = "unknown"
)

// ClassDeclarationNode represents a class definition with inheritance support
// This is a language agnostic representation of a class declaration.
// Not all attributes may be present in all languages
type ClassDeclarationNode struct {
	Node

	// Core class information
	classNameNode *sitter.Node

	// Inheritance information - supports multiple inheritance (Python)
	baseClassNodes []*sitter.Node

	// Class body components
	methodNodes []*sitter.Node
	fieldNodes  []*sitter.Node

	// Constructor method (e.g., __init__ in Python, constructor in JS)
	constructorNode *sitter.Node

	// Language-specific decorators (Python @decorator, Java @Annotation)
	decoratorNodes []*sitter.Node

	// Metadata
	isAbstract     bool
	accessModifier AccessModifier
}

// NewClassDeclarationNode creates a new ClassDeclarationNode instance
// using the class definition node from the tree-sitter parser
func NewClassDeclarationNode(content Content) *ClassDeclarationNode {
	return &ClassDeclarationNode{
		Node:           Node{content},
		baseClassNodes: []*sitter.Node{},
		methodNodes:    []*sitter.Node{},
		fieldNodes:     []*sitter.Node{},
		decoratorNodes: []*sitter.Node{},
		isAbstract:     false,
		accessModifier: AccessModifierPublic, // Default to public access
	}
}

// ClassName returns the name of the class
func (c *ClassDeclarationNode) ClassName() string {
	return c.contentForNode(c.classNameNode)
}

// BaseClasses returns the names of all base classes this class inherits from
func (c *ClassDeclarationNode) BaseClasses() []string {
	var baseClasses []string
	for _, baseClassNode := range c.baseClassNodes {
		if baseClass := c.contentForNode(baseClassNode); baseClass != "" {
			baseClasses = append(baseClasses, baseClass)
		}
	}
	return baseClasses
}

// HasInheritance returns true if this class inherits from one or more base classes
func (c *ClassDeclarationNode) HasInheritance() bool {
	return len(c.baseClassNodes) > 0
}

// Methods returns the content of all method nodes
func (c *ClassDeclarationNode) Methods() []string {
	var methods []string
	for _, methodNode := range c.methodNodes {
		if method := c.contentForNode(methodNode); method != "" {
			methods = append(methods, method)
		}
	}
	return methods
}

// Fields returns the content of all field nodes
func (c *ClassDeclarationNode) Fields() []string {
	var fields []string
	for _, fieldNode := range c.fieldNodes {
		if field := c.contentForNode(fieldNode); field != "" {
			fields = append(fields, field)
		}
	}
	return fields
}

// Constructor returns the content of the constructor method
func (c *ClassDeclarationNode) Constructor() string {
	return c.contentForNode(c.constructorNode)
}

// Decorators returns the content of all decorator nodes
func (c *ClassDeclarationNode) Decorators() []string {
	var decorators []string
	for _, decoratorNode := range c.decoratorNodes {
		if decorator := c.contentForNode(decoratorNode); decorator != "" {
			decorators = append(decorators, decorator)
		}
	}
	return decorators
}

// IsAbstract returns true if this is an abstract class
func (c *ClassDeclarationNode) IsAbstract() bool {
	return c.isAbstract
}

// AccessModifier returns the access modifier of the class
func (c *ClassDeclarationNode) AccessModifier() AccessModifier {
	return c.accessModifier
}

// Getter methods for Tree-Sitter nodes
func (c *ClassDeclarationNode) GetClassNameNode() *sitter.Node {
	return c.classNameNode
}

func (c *ClassDeclarationNode) GetBaseClassNodes() []*sitter.Node {
	return c.baseClassNodes
}

func (c *ClassDeclarationNode) GetMethodNodes() []*sitter.Node {
	return c.methodNodes
}

func (c *ClassDeclarationNode) GetFieldNodes() []*sitter.Node {
	return c.fieldNodes
}

func (c *ClassDeclarationNode) GetConstructorNode() *sitter.Node {
	return c.constructorNode
}

func (c *ClassDeclarationNode) GetDecoratorNodes() []*sitter.Node {
	return c.decoratorNodes
}

// Setter methods for Tree-Sitter nodes
func (c *ClassDeclarationNode) SetClassNameNode(node *sitter.Node) {
	c.classNameNode = node
}

func (c *ClassDeclarationNode) SetBaseClassNodes(nodes []*sitter.Node) {
	c.baseClassNodes = nodes
}

func (c *ClassDeclarationNode) AddBaseClassNode(node *sitter.Node) {
	c.baseClassNodes = append(c.baseClassNodes, node)
}

func (c *ClassDeclarationNode) SetMethodNodes(nodes []*sitter.Node) {
	c.methodNodes = nodes
}

func (c *ClassDeclarationNode) AddMethodNode(node *sitter.Node) {
	c.methodNodes = append(c.methodNodes, node)
}

func (c *ClassDeclarationNode) SetFieldNodes(nodes []*sitter.Node) {
	c.fieldNodes = nodes
}

func (c *ClassDeclarationNode) AddFieldNode(node *sitter.Node) {
	c.fieldNodes = append(c.fieldNodes, node)
}

func (c *ClassDeclarationNode) SetConstructorNode(node *sitter.Node) {
	c.constructorNode = node
}

func (c *ClassDeclarationNode) SetDecoratorNodes(nodes []*sitter.Node) {
	c.decoratorNodes = nodes
}

func (c *ClassDeclarationNode) AddDecoratorNode(node *sitter.Node) {
	c.decoratorNodes = append(c.decoratorNodes, node)
}

func (c *ClassDeclarationNode) SetIsAbstract(isAbstract bool) {
	c.isAbstract = isAbstract
}

func (c *ClassDeclarationNode) SetAccessModifier(accessModifier AccessModifier) {
	c.accessModifier = accessModifier
}

// String returns a string representation of the ClassDeclarationNode for debugging
func (c *ClassDeclarationNode) String() string {
	var parts []string

	// Access modifier
	if c.accessModifier != AccessModifierPublic {
		parts = append(parts, string(c.accessModifier))
	}

	// Abstract modifier
	if c.isAbstract {
		parts = append(parts, "abstract")
	}

	// Class name
	className := c.ClassName()
	if className == "" {
		className = "<unnamed>"
	}

	// Inheritance
	baseClasses := c.BaseClasses()
	inheritance := ""
	if len(baseClasses) > 0 {
		inheritance = fmt.Sprintf("(%s)", strings.Join(baseClasses, ", "))
	}

	// Decorators
	decorators := c.Decorators()
	decoratorStr := ""
	if len(decorators) > 0 {
		decoratorStr = fmt.Sprintf("[%s] ", strings.Join(decorators, ", "))
	}

	// Method and field counts
	methodCount := len(c.methodNodes)
	fieldCount := len(c.fieldNodes)
	hasConstructor := c.constructorNode != nil

	classModifiers := strings.Join(parts, " ")
	if classModifiers != "" {
		classModifiers += " "
	}

	return fmt.Sprintf("%sClassDeclarationNode{%sclass: %s%s, methods: %d, fields: %d, constructor: %t}",
		decoratorStr,
		classModifiers,
		className,
		inheritance,
		methodCount,
		fieldCount,
		hasConstructor)
}
