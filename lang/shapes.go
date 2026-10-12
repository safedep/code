package lang

import (
	"context"
	"fmt"

	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
	"github.com/safedep/code/pkg/ts"
	"github.com/safedep/dry/log"
	sitter "github.com/smacker/go-tree-sitter"
)

// childRef finds a child of a node by its field name, or else by the type of
// its first named child. Some grammars, such as Kotlin, do not name fields.
type childRef struct {
	field     string
	childType string
}

func (r childRef) find(node *sitter.Node) *sitter.Node {
	if r.field != "" {
		return node.ChildByFieldName(r.field)
	}
	if r.childType != "" {
		return firstNamedChildOfType(node, r.childType)
	}
	return nil
}

func field(name string) childRef {
	return childRef{field: name}
}

func childOfType(nodeType string) childRef {
	return childRef{childType: nodeType}
}

// functionShape tells where a function declaration node keeps its parts.
type functionShape struct {
	name         childRef
	parameters   childRef
	body         childRef
	functionType ast.FunctionType
}

// resolveFunctionsByShape returns a declaration for each node in the tree
// whose type has a shape.
func resolveFunctionsByShape(tree core.ParseTree, shapes map[string]functionShape) ([]*ast.FunctionDeclarationNode, error) {
	data, err := tree.Data()
	if err != nil {
		return nil, fmt.Errorf("failed to get data from parse tree: %w", err)
	}

	var functions []*ast.FunctionDeclarationNode
	walkNamed(tree.Tree().RootNode(), func(node *sitter.Node) bool {
		shape, ok := shapes[node.Type()]
		if !ok {
			return true
		}

		nameNode := shape.name.find(node)
		if nameNode == nil {
			return true
		}

		function := ast.NewFunctionDeclarationNode(data)
		function.SetFunctionNameNode(nameNode)
		function.SetFunctionType(shape.functionType)
		function.SetFunctionBodyNode(shape.body.find(node))
		if parameters := shape.parameters.find(node); parameters != nil {
			function.SetFunctionParameterNodes(namedChildren(parameters))
		}

		functions = append(functions, function)
		return true
	})

	return functions, nil
}

// walkNamed visits the node and its named descendants in source order. It does
// not visit the children of a node for which visit returns false. The walk
// has no node limit, as the resolvers had before ts.Walk.
func walkNamed(node *sitter.Node, visit func(*sitter.Node) bool) {
	if err := ts.Walk(context.Background(), node, ts.WalkOptions{MaxNodes: -1}, visit); err != nil {
		log.Warnf("failed to walk the tree: %v", err)
	}
}

func namedChildren(node *sitter.Node) []*sitter.Node {
	return ts.NamedChildren(node)
}

func firstNamedChildOfType(node *sitter.Node, types ...string) *sitter.Node {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		for _, t := range types {
			if child.Type() == t {
				return child
			}
		}
	}
	return nil
}

func lastNamedChildOfType(node *sitter.Node, types ...string) *sitter.Node {
	for i := int(node.NamedChildCount()) - 1; i >= 0; i-- {
		child := node.NamedChild(i)
		for _, t := range types {
			if child.Type() == t {
				return child
			}
		}
	}
	return nil
}

func newModuleImport(data *[]byte, module, item, alias *sitter.Node, wildcard bool) *ast.ImportNode {
	node := ast.NewImportNode(data)
	node.SetModuleNameNode(module)
	node.SetModuleItemNode(item)
	node.SetModuleAliasNode(alias)
	node.SetIsWildcardImport(wildcard)
	return node
}
