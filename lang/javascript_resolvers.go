package lang

import (
	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
)

type javascriptResolvers struct{}

var _ core.LanguageResolvers = (*javascriptResolvers)(nil)

func (r *javascriptResolvers) ResolveImports(tree core.ParseTree) ([]*ast.ImportNode, error) {
	return resolveESImports(tree)
}

func (r *javascriptResolvers) ResolveFunctions(tree core.ParseTree) ([]*ast.FunctionDeclarationNode, error) {
	return resolveESFunctions(tree)
}
