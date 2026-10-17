package callgraph

import (
	"context"
	"fmt"

	"github.com/safedep/code/core"
	"github.com/safedep/dry/log"
	sitter "github.com/smacker/go-tree-sitter"
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

var supportedLanguages = []core.LanguageCode{
	core.LanguageCodePython,
	core.LanguageCodeJava,
	core.LanguageCodeGo,
	core.LanguageCodeJavascript,
	core.LanguageCodeTypescript,
	core.LanguageCodeCSharp,
	core.LanguageCodeRust,
	core.LanguageCodePHP,
	core.LanguageCodeRuby,
}

func (p *callgraphPlugin) SupportedLanguages() []core.LanguageCode {
	return supportedLanguages
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
// truncation.
func buildCallGraph(ctx context.Context, tree core.ParseTree, lang core.Language, filePath string) (*CallGraph, error) {
	astRootNode := tree.Tree().RootNode()

	treeData, err := tree.Data()
	if err != nil {
		return nil, fmt.Errorf("failed to get tree data: %w", err)
	}

	imports, err := lang.Resolvers().ResolveImports(ctx, tree)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve imports: %w", err)
	}

	callGraph, err := newCallGraph(filePath, astRootNode, imports, tree)
	if err != nil {
		return nil, fmt.Errorf("failed to create call graph: %w", err)
	}

	callGraph.walk(ctx, astRootNode, *treeData)
	return callGraph, nil
}

// walk processes the tree from root. It stops when ctx is done, and the
// graph does not keep ctx after it.
func (cg *CallGraph) walk(ctx context.Context, root *sitter.Node, treeData []byte) {
	cg.ctx = ctx
	defer func() { cg.ctx = nil }()
	processChildren(root, treeData, cg.FileName, cg, processorMetadata{})
}

func processNode(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if node == nil {
		return newProcessorResult()
	}

	if nodeProcessor, exists := languageNodeProcessors[callGraph.languageCode][node.Type()]; exists {
		return nodeProcessor(node, treeData, currentNamespace, callGraph, metadata)
	}

	nodeProcessor, exists := nodeProcessors[node.Type()]
	if exists {
		return nodeProcessor(node, treeData, currentNamespace, callGraph, metadata)
	}

	// log.Debugf("Can't process %s with namespace: %s => %s", node.Type(), currentNamespace, node.Content(treeData))
	return emptyProcessor(node, treeData, currentNamespace, callGraph, metadata)
}

func processChildren(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if node == nil || callGraph.ctxErr != nil {
		return newProcessorResult()
	}

	// Check processing depth to prevent stack overflow in deeply nested structures
	if metadata.processingDepth > maxProcessingDepth {
		callGraph.truncate(TruncationDepth, currentNamespace, node)
		return newProcessorResult()
	}

	childrenResults := newProcessorResult()

	// Increment depth for child processing
	childMetadata := metadata
	childMetadata.processingDepth++

	for i := 0; i < int(node.ChildCount()); i++ {
		if callGraph.stopped() {
			break
		}
		result := processNode(node.Child(i), treeData, currentNamespace, callGraph, childMetadata)
		childrenResults.addResults(result)
	}

	return childrenResults
}
