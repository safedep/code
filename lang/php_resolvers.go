package lang

import (
	"fmt"

	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
	sitter "github.com/smacker/go-tree-sitter"
)

type phpResolvers struct{}

var _ core.LanguageResolvers = (*phpResolvers)(nil)

// phpImportContainers are the nodes that can hold a use declaration.
var phpImportContainers = map[string]bool{
	"program":              true,
	"namespace_definition": true,
	"compound_statement":   true,
}

var phpFunctionShapes = map[string]functionShape{
	"function_definition": {name: field("name"), parameters: field("parameters"), body: field("body"), functionType: ast.FunctionTypeFunction},
	"method_declaration":  {name: field("name"), parameters: field("parameters"), body: field("body"), functionType: ast.FunctionTypeMethod},
}

// ResolveImports returns one import for each name in a use declaration, as in
// use GuzzleHttp\Client as Http or use Illuminate\Support\{Str, Arr}. The
// last segment of the name is the name in scope unless an alias renames it.
func (r *phpResolvers) ResolveImports(tree core.ParseTree) ([]*ast.ImportNode, error) {
	data, err := tree.Data()
	if err != nil {
		return nil, fmt.Errorf("failed to get data from parse tree: %w", err)
	}

	var imports []*ast.ImportNode
	walkNamed(tree.Tree().RootNode(), func(node *sitter.Node) bool {
		if node.Type() != "namespace_use_declaration" {
			return phpImportContainers[node.Type()]
		}

		prefix := firstNamedChildOfType(node, "namespace_name")
		for _, child := range namedChildren(node) {
			switch child.Type() {
			case "namespace_use_clause":
				module := firstNamedChildOfType(child, "qualified_name", "name")
				if module != nil {
					imports = append(imports, newModuleImport(data, module, nil, phpAlias(child, module), false))
				}
			case "namespace_use_group":
				for _, clause := range namedChildren(child) {
					item := firstNamedChildOfType(clause, "namespace_name", "qualified_name", "name")
					if prefix != nil && item != nil {
						imports = append(imports, newModuleImport(data, prefix, item, phpAlias(clause, item), false))
					}
				}
			}
		}
		return false
	})

	return imports, nil
}

// phpAlias returns the alias of a use clause, or else the last segment of the
// imported name.
func phpAlias(clause, name *sitter.Node) *sitter.Node {
	if aliasing := firstNamedChildOfType(clause, "namespace_aliasing_clause"); aliasing != nil {
		if alias := firstNamedChildOfType(aliasing, "name"); alias != nil {
			return alias
		}
	}
	if last := lastNamedChildOfType(name, "name"); last != nil {
		return last
	}
	return name
}

func (r *phpResolvers) ResolveFunctions(tree core.ParseTree) ([]*ast.FunctionDeclarationNode, error) {
	return resolveFunctionsByShape(tree, phpFunctionShapes)
}
