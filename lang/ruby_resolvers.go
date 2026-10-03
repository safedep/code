package lang

import (
	"fmt"

	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
	sitter "github.com/smacker/go-tree-sitter"
)

type rubyResolvers struct{}

var _ core.LanguageResolvers = (*rubyResolvers)(nil)

var rubyFunctionShapes = map[string]functionShape{
	"method":           {name: field("name"), parameters: field("parameters"), body: field("body"), functionType: ast.FunctionTypeMethod},
	"singleton_method": {name: field("name"), parameters: field("parameters"), body: field("body"), functionType: ast.FunctionTypeStaticMethod},
}

// ResolveImports returns one wildcard import for each require call with a
// literal path, as in require 'net/http'. A required file defines constants
// in the global scope, so the require does not bind a name. The resolver
// skips require_relative, because it loads a file of the project.
//
// Bundler.require in a Rails application loads the gems of the Gemfile
// without a require call. Such a file has no import.
func (r *rubyResolvers) ResolveImports(tree core.ParseTree) ([]*ast.ImportNode, error) {
	data, err := tree.Data()
	if err != nil {
		return nil, fmt.Errorf("failed to get data from parse tree: %w", err)
	}

	var imports []*ast.ImportNode
	walkNamed(tree.Tree().RootNode(), func(node *sitter.Node) bool {
		if node.Type() != "call" || node.ChildByFieldName("receiver") != nil {
			return true
		}

		method := node.ChildByFieldName("method")
		if method == nil || method.Content(*data) != "require" {
			return true
		}

		if path := rubyLiteralPath(node.ChildByFieldName("arguments")); path != nil {
			imports = append(imports, newModuleImport(data, path, nil, nil, true))
		}
		return false
	})

	return imports, nil
}

// rubyLiteralPath returns the content node of the first argument when it is a
// string without interpolation.
func rubyLiteralPath(arguments *sitter.Node) *sitter.Node {
	if arguments == nil || arguments.NamedChildCount() == 0 {
		return nil
	}

	str := arguments.NamedChild(0)
	if str.Type() != "string" || str.NamedChildCount() != 1 {
		return nil
	}

	content := str.NamedChild(0)
	if content.Type() != "string_content" {
		return nil
	}
	return content
}

func (r *rubyResolvers) ResolveFunctions(tree core.ParseTree) ([]*ast.FunctionDeclarationNode, error) {
	return resolveFunctionsByShape(tree, rubyFunctionShapes)
}
