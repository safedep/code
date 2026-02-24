package lang

import (
	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
)

type javascriptResolvers struct {
	language *javascriptLanguage
}

var _ core.LanguageResolvers = (*javascriptResolvers)(nil)

func (r *javascriptResolvers) ResolveImports(tree core.ParseTree) ([]*ast.ImportNode, error) {
	return resolveESImports(r.language, tree)
}

func (r *javascriptResolvers) ResolveFunctions(tree core.ParseTree) ([]*ast.FunctionDeclarationNode, error) {
	return resolveESFunctions(r.language, tree)
}
