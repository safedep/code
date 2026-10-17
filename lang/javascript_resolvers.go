package lang

import (
	"context"
	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
)

type javascriptResolvers struct{}

var _ core.LanguageResolvers = (*javascriptResolvers)(nil)

func (r *javascriptResolvers) ResolveImports(ctx context.Context, tree core.ParseTree) ([]*ast.ImportNode, error) {
	return resolveESImports(ctx, tree)
}

func (r *javascriptResolvers) ResolveFunctions(ctx context.Context, tree core.ParseTree) ([]*ast.FunctionDeclarationNode, error) {
	return resolveESFunctions(ctx, tree)
}
