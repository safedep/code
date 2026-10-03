package lang

import (
	"fmt"

	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
	sitter "github.com/smacker/go-tree-sitter"
)

type kotlinResolvers struct{}

var _ core.LanguageResolvers = (*kotlinResolvers)(nil)

// The Kotlin grammar does not name the fields of a function declaration.
var kotlinFunctionShapes = map[string]functionShape{
	"function_declaration": {
		name:         childOfType("simple_identifier"),
		parameters:   childOfType("function_value_parameters"),
		body:         childOfType("function_body"),
		functionType: ast.FunctionTypeFunction,
	},
}

// ResolveImports returns one import for each import header, as in
// import okhttp3.OkHttpClient or import kotlinx.coroutines.*. The last
// segment of the name is the name in scope unless an alias renames it.
func (r *kotlinResolvers) ResolveImports(tree core.ParseTree) ([]*ast.ImportNode, error) {
	data, err := tree.Data()
	if err != nil {
		return nil, fmt.Errorf("failed to get data from parse tree: %w", err)
	}

	var imports []*ast.ImportNode
	walkNamed(tree.Tree().RootNode(), func(node *sitter.Node) bool {
		switch node.Type() {
		case "source_file", "import_list":
			return true
		case "import_header":
		default:
			return false
		}

		module := firstNamedChildOfType(node, "identifier")
		if module == nil {
			return false
		}

		if firstNamedChildOfType(node, "wildcard_import") != nil {
			imports = append(imports, newModuleImport(data, module, nil, nil, true))
			return false
		}

		alias := lastNamedChildOfType(module, "simple_identifier")
		if aliasing := firstNamedChildOfType(node, "import_alias"); aliasing != nil {
			alias = firstNamedChildOfType(aliasing, "type_identifier", "simple_identifier")
		}

		imports = append(imports, newModuleImport(data, module, nil, alias, false))
		return false
	})

	return imports, nil
}

func (r *kotlinResolvers) ResolveFunctions(tree core.ParseTree) ([]*ast.FunctionDeclarationNode, error) {
	return resolveFunctionsByShape(tree, kotlinFunctionShapes)
}
