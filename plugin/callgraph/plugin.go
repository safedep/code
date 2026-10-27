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
	return SupportedLanguages()
}

func (p *callgraphPlugin) AnalyzeTree(ctx context.Context, tree core.ParseTree) error {
	cg, err := Build(ctx, tree)
	if err != nil {
		return fmt.Errorf("failed to build call graph: %w", err)
	}
	return p.AnalyzeGraph(ctx, cg)
}

// AnalyzeGraph runs the callback on a call graph that the caller built, as
// the indexer builds one for the index of each file. It fails on a partial
// graph, as AnalyzeTree does.
func (p *callgraphPlugin) AnalyzeGraph(ctx context.Context, cg *CallGraph) error {
	if err := p.callgraphCallback(ctx, cg); err != nil {
		return err
	}

	// The callback gets the partial graph of a file that the deadline cut,
	// but the file is not a success.
	if cg.ctxErr != nil {
		return fmt.Errorf("call graph of %s is partial: %w", cg.FileName, cg.ctxErr)
	}
	return nil
}

// Build builds the call graph of the file of tree. When ctx is done during
// the walk of the tree, it returns the graph so far with a deadline
// truncation. The graph keeps neither the tree nor its data, so the caller
// can close the tree.
func Build(ctx context.Context, tree core.ParseTree) (*CallGraph, error) {
	lang, err := tree.Language()
	if err != nil {
		return nil, fmt.Errorf("failed to get language: %w", err)
	}

	file, err := tree.File()
	if err != nil {
		return nil, fmt.Errorf("failed to get file: %w", err)
	}

	treeData, err := tree.Data()
	if err != nil {
		return nil, fmt.Errorf("failed to get tree data: %w", err)
	}

	imports, err := lang.Resolvers().ResolveImports(ctx, tree)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve imports: %w", err)
	}

	log.Debugf("callgraph - Analyzing tree for language: %s, file: %s", lang.Meta().Code, file.Name())

	callGraph := newCallGraph(file.Name(), lang.Meta().Code)
	callGraph.build(ctx, tree.Tree().RootNode(), imports, *treeData)
	return callGraph, nil
}
