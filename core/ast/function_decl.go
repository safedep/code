package ast

import sitter "github.com/smacker/go-tree-sitter"

// FunctionType represents the type/category of function
type FunctionType string

const (
	// FunctionTypeFunction represents a regular function
	FunctionTypeFunction FunctionType = "function"

	// FunctionTypeMethod represents a class method
	FunctionTypeMethod FunctionType = "method"

	// FunctionTypeConstructor represents a constructor function
	FunctionTypeConstructor FunctionType = "constructor"

	// FunctionTypeStaticMethod represents a static class method
	FunctionTypeStaticMethod FunctionType = "static_method"

	// FunctionTypeAsync represents an async function
	FunctionTypeAsync FunctionType = "async"

	// FunctionTypeArrow represents an arrow function (JavaScript)
	FunctionTypeArrow FunctionType = "arrow"

	// FunctionTypeUnknown represents unknown function type
	FunctionTypeUnknown FunctionType = "unknown"
)

// FunctionDeclarationNode represents a function declaration with comprehensive metadata
// This is a language agnostic representation of a function declaration.
// Not all attributes may be present in all languages
type FunctionDeclarationNode struct {
	Node

	// Core function information
	functionNameNode *sitter.Node

	// Function signature
	functionParameterNodes []*sitter.Node
	functionReturnTypeNode *sitter.Node

	// Function body and implementation
	functionBodyNode *sitter.Node

	// Language-specific decorators/annotations (Python @decorator, Java @Annotation)
	decoratorNodes []*sitter.Node

	// Function metadata
	functionType   FunctionType
	accessModifier AccessModifier
	isAbstract     bool
	isStatic       bool
	isAsync        bool

	// Parent class context (for methods)
	parentClassName string
}

// NewFunctionDeclarationNode creates a new FunctionDeclarationNode instance
func NewFunctionDeclarationNode(content Content) *FunctionDeclarationNode {
	return &FunctionDeclarationNode{
		Node:                   Node{content},
		functionParameterNodes: []*sitter.Node{},
		decoratorNodes:         []*sitter.Node{},
		functionType:           FunctionTypeFunction, // Default to regular function
		accessModifier:         AccessModifierPublic, // Default to public access
		isAbstract:             false,
		isStatic:               false,
		isAsync:                false,
	}
}

// FunctionName returns the name of the function
func (f *FunctionDeclarationNode) FunctionName() string {
	return f.contentForNode(f.functionNameNode)
}

// Parameters returns the content of all parameter nodes
func (f *FunctionDeclarationNode) Parameters() []string {
	var parameters []string
	for _, paramNode := range f.functionParameterNodes {
		if param := f.contentForNode(paramNode); param != "" {
			parameters = append(parameters, param)
		}
	}
	return parameters
}

// ReturnType returns the return type of the function if available
func (f *FunctionDeclarationNode) ReturnType() string {
	return f.contentForNode(f.functionReturnTypeNode)
}

// Body returns the function body content
func (f *FunctionDeclarationNode) Body() string {
	return f.contentForNode(f.functionBodyNode)
}

// Decorators returns the content of all decorator/annotation nodes
func (f *FunctionDeclarationNode) Decorators() []string {
	var decorators []string
	for _, decoratorNode := range f.decoratorNodes {
		if decorator := f.contentForNode(decoratorNode); decorator != "" {
			decorators = append(decorators, decorator)
		}
	}
	return decorators
}

// HasDecorators returns true if the function has decorators/annotations
func (f *FunctionDeclarationNode) HasDecorators() bool {
	return len(f.decoratorNodes) > 0
}

// IsMethod returns true if this function is a class method
func (f *FunctionDeclarationNode) IsMethod() bool {
	return f.functionType == FunctionTypeMethod ||
		f.functionType == FunctionTypeConstructor ||
		f.functionType == FunctionTypeStaticMethod
}

// IsConstructor returns true if this function is a constructor
func (f *FunctionDeclarationNode) IsConstructor() bool {
	return f.functionType == FunctionTypeConstructor
}

// GetFunctionType returns the type/category of the function
func (f *FunctionDeclarationNode) GetFunctionType() FunctionType {
	return f.functionType
}

// GetAccessModifier returns the access modifier of the function
func (f *FunctionDeclarationNode) GetAccessModifier() AccessModifier {
	return f.accessModifier
}

// GetParentClassName returns the name of the parent class (for methods)
func (f *FunctionDeclarationNode) GetParentClassName() string {
	return f.parentClassName
}

// GetFunctionNameNode returns the function name node
func (f *FunctionDeclarationNode) GetFunctionNameNode() *sitter.Node {
	return f.functionNameNode
}

// IsAbstract returns true if the function is abstract
func (f *FunctionDeclarationNode) IsAbstract() bool {
	return f.isAbstract
}

// IsStatic returns true if the function is static
func (f *FunctionDeclarationNode) IsStatic() bool {
	return f.isStatic
}

// IsAsync returns true if the function is async
func (f *FunctionDeclarationNode) IsAsync() bool {
	return f.isAsync
}

// Setter methods

// SetFunctionNameNode sets the function name node
func (f *FunctionDeclarationNode) SetFunctionNameNode(node *sitter.Node) {
	f.functionNameNode = node
}

// SetFunctionParameterNodes sets all parameter nodes
func (f *FunctionDeclarationNode) SetFunctionParameterNodes(nodes []*sitter.Node) {
	f.functionParameterNodes = nodes
}

// AddFunctionParameterNode adds a parameter node
func (f *FunctionDeclarationNode) AddFunctionParameterNode(node *sitter.Node) {
	f.functionParameterNodes = append(f.functionParameterNodes, node)
}

// SetFunctionReturnTypeNode sets the return type node
func (f *FunctionDeclarationNode) SetFunctionReturnTypeNode(node *sitter.Node) {
	f.functionReturnTypeNode = node
}

// SetFunctionBodyNode sets the function body node
func (f *FunctionDeclarationNode) SetFunctionBodyNode(node *sitter.Node) {
	f.functionBodyNode = node
}

// AddDecoratorNode adds a decorator/annotation node
func (f *FunctionDeclarationNode) AddDecoratorNode(node *sitter.Node) {
	f.decoratorNodes = append(f.decoratorNodes, node)
}

// SetFunctionType sets the function type/category
func (f *FunctionDeclarationNode) SetFunctionType(funcType FunctionType) {
	f.functionType = funcType
}

// SetAccessModifier sets the access modifier
func (f *FunctionDeclarationNode) SetAccessModifier(modifier AccessModifier) {
	f.accessModifier = modifier
}

// SetIsAbstract sets the abstract flag
func (f *FunctionDeclarationNode) SetIsAbstract(abstract bool) {
	f.isAbstract = abstract
}

// SetIsStatic sets the static flag
func (f *FunctionDeclarationNode) SetIsStatic(static bool) {
	f.isStatic = static
}

// SetIsAsync sets the async flag
func (f *FunctionDeclarationNode) SetIsAsync(async bool) {
	f.isAsync = async
}

// SetParentClassName sets the parent class name (for methods)
func (f *FunctionDeclarationNode) SetParentClassName(className string) {
	f.parentClassName = className
}
