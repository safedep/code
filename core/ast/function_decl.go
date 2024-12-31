package ast

import sitter "github.com/smacker/go-tree-sitter"

// TODO: Does this makes sense?
type FunctionDeclarationNode struct {
	Node

	// Name of the function
	FunctionNameNode *sitter.Node

	// Function parameters
	FunctionParameterNodes []*sitter.Node

	// Function return type
	FunctionReturnTypeNode *sitter.Node

	// Function body
	FunctionBodyNode *sitter.Node
}
