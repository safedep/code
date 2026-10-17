package callgraph

import (
	"context"
	"fmt"

	"github.com/safedep/code/core"
	"github.com/safedep/dry/log"
)

type CallgraphCallback core.PluginCallback[*CallGraph]

type callgraphPlugin struct {
	// Callback function which is called with the callgraph
	callgraphCallback CallgraphCallback
}

// Verify contract
var _ core.TreePlugin = (*callgraphPlugin)(nil)

func NewCallGraphPlugin(callgraphCallback CallgraphCallback) *callgraphPlugin {
	return &callgraphPlugin{
		callgraphCallback: callgraphCallback,
	}
}

func (p *callgraphPlugin) Name() string {
	return "CallgraphPlugin"
}

func (p *callgraphPlugin) SupportedLanguages() []core.LanguageCode {
	return supportedLanguages()
}

func (p *callgraphPlugin) AnalyzeTree(ctx context.Context, tree core.ParseTree) error {
	lang, err := tree.Language()
	if err != nil {
		return fmt.Errorf("failed to get language: %w", err)
	}

	file, err := tree.File()
	if err != nil {
		return fmt.Errorf("failed to get file: %w", err)
	}

	log.Debugf("callgraph - Analyzing tree for language: %s, file: %s\n", lang.Meta().Code, file.Name())

	cg, err := buildCallGraph(ctx, tree, lang, file.Name())
	if err != nil {
		return fmt.Errorf("failed to build call graph: %w", err)
	}

	if err := p.callgraphCallback(ctx, cg); err != nil {
		return err
	}

	// The callback gets the partial graph of a file that the deadline cut,
	// but the file is not a success.
	if cg.ctxErr != nil {
		return fmt.Errorf("call graph of %s is partial: %w", file.Name(), cg.ctxErr)
	}
	return nil
}

// buildCallGraph builds a call graph from the syntax tree. When ctx is done
// during the walk of the tree, it returns the graph so far with a deadline
// truncation. The graph keeps neither the tree nor its data.
func buildCallGraph(ctx context.Context, tree core.ParseTree, lang core.Language, filePath string) (*CallGraph, error) {
	treeData, err := tree.Data()
	if err != nil {
		return nil, fmt.Errorf("failed to get tree data: %w", err)
	}

	imports, err := lang.Resolvers().ResolveImports(ctx, tree)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve imports: %w", err)
	}

	callGraph := newCallGraph(filePath, lang.Meta().Code)
	callGraph.build(ctx, tree.Tree().RootNode(), imports, *treeData)
	return callGraph, nil
}
