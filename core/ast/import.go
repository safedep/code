package ast

import (
	"fmt"

	sitter "github.com/smacker/go-tree-sitter"
)

// ImportNode represents an import statement in a source file
// This is a language agnostic representation of an import statement.
// Not all attributes may be present in all languages.
type ImportNode struct {
	Node

	// The package or module being imported
	moduleNameNode *sitter.Node

	// The object being imported
	moduleItemNode *sitter.Node

	// The alias of the import in the current scope
	moduleAliasNode *sitter.Node
}

// NewImportNode creates a new ImportNode instance
// using the import statement node from the tree-sitter parser
func NewImportNode(content []byte) *ImportNode {
	return &ImportNode{
		Node: Node{content},
	}
}

func (i *ImportNode) ModuleName() string {
	if i.moduleNameNode == nil {
		return ""
	}

	return i.moduleNameNode.Content(i.content)
}

func (i *ImportNode) ModuleItem() string {
	if i.moduleItemNode == nil {
		return ""
	}

	return i.moduleItemNode.Content(i.content)
}

func (i *ImportNode) ModuleAlias() string {
	if i.moduleAliasNode == nil {
		return ""
	}

	return i.moduleAliasNode.Content(i.content)
}

func (i *ImportNode) SetModuleNameNode(node *sitter.Node) {
	i.moduleNameNode = node
}

func (i *ImportNode) SetModuleItemNode(node *sitter.Node) {
	i.moduleItemNode = node
}

func (i *ImportNode) SetModuleAliasNode(node *sitter.Node) {
	i.moduleAliasNode = node
}

func (i *ImportNode) String() string {
	return fmt.Sprintf("ImportNode{ModuleName: %s, ModuleItem: %s, ModuleAlias: %s}",
		i.ModuleName(), i.ModuleItem(), i.ModuleAlias())
}
