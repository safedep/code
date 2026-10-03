package lang

import (
	"fmt"

	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
	sitter "github.com/smacker/go-tree-sitter"
)

type csharpResolvers struct{}

var _ core.LanguageResolvers = (*csharpResolvers)(nil)

// csharpImportContainers are the nodes that can hold a using directive.
var csharpImportContainers = map[string]bool{
	"compilation_unit":      true,
	"namespace_declaration": true,
	"declaration_list":      true,
}

var csharpFunctionShapes = map[string]functionShape{
	"method_declaration":       {name: field("name"), parameters: field("parameters"), body: field("body"), functionType: ast.FunctionTypeMethod},
	"constructor_declaration":  {name: field("name"), parameters: field("parameters"), body: field("body"), functionType: ast.FunctionTypeConstructor},
	"local_function_statement": {name: field("name"), parameters: field("parameters"), body: field("body"), functionType: ast.FunctionTypeFunction},
}

// ResolveImports returns one import for each using directive. A using
// directive without an alias brings every type of a namespace into scope, so
// it is a wildcard import. An alias imports one namespace or type by name.
func (r *csharpResolvers) ResolveImports(tree core.ParseTree) ([]*ast.ImportNode, error) {
	data, err := tree.Data()
	if err != nil {
		return nil, fmt.Errorf("failed to get data from parse tree: %w", err)
	}

	var imports []*ast.ImportNode
	walkNamed(tree.Tree().RootNode(), func(node *sitter.Node) bool {
		if node.Type() != "using_directive" {
			return csharpImportContainers[node.Type()]
		}

		alias := node.ChildByFieldName("name")
		module := lastNamedChildOfType(node, "qualified_name", "identifier", "generic_name")
		if module == nil {
			return false
		}

		imports = append(imports, newModuleImport(data, module, nil, alias, alias == nil))
		return false
	})

	return imports, nil
}

func (r *csharpResolvers) ResolveFunctions(tree core.ParseTree) ([]*ast.FunctionDeclarationNode, error) {
	return resolveFunctionsByShape(tree, csharpFunctionShapes)
}
