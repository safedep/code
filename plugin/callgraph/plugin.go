package callgraph

import (
	"context"
	"fmt"
	"sync"

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

var loadBuiltinOnce sync.Once

func NewCallGraphPlugin(callgraphCallback CallgraphCallback) *callgraphPlugin {
	// Load builtin keywords
	loadBuiltinOnce.Do(initBuiltins)

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

	cg, err := buildCallGraph(tree, lang, file.Name())
	if err != nil {
		return fmt.Errorf("failed to build call graph: %w", err)
	}

	return p.callgraphCallback(ctx, cg)
}

// buildCallGraph builds a call graph from the syntax tree
func buildCallGraph(tree core.ParseTree, lang core.Language, filePath string) (*CallGraph, error) {
	astRootNode := tree.Tree().RootNode()

	treeData, err := tree.Data()
	if err != nil {
		return nil, fmt.Errorf("failed to get tree data: %w", err)
	}

	imports, err := lang.Resolvers().ResolveImports(tree)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve imports: %w", err)
	}

	// log.Debugf("Imported identifier => namespace:")
	// for identifier, parsedImport := range importedIdentifiers {
	// 	log.Debugf("  %s => %s\n", identifier, parsedImport.Namespace)
	// }

	callGraph, err := newCallGraph(filePath, astRootNode, imports, tree)
	if err != nil {
		return nil, fmt.Errorf("failed to create call graph: %w", err)
	}

	processChildren(astRootNode, *treeData, filePath, callGraph, processorMetadata{})

	return callGraph, nil
}

func processNode(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if node == nil {
		return newProcessorResult()
	}

	nodeProcessor, exists := nodeProcessors[node.Type()]
	if exists {
		return nodeProcessor(node, treeData, currentNamespace, callGraph, metadata)
	}

	// log.Debugf("Can't process %s with namespace: %s => %s", node.Type(), currentNamespace, node.Content(treeData))
	return emptyProcessor(node, treeData, currentNamespace, callGraph, metadata)
}

func processChildren(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if node == nil {
		return newProcessorResult()
	}

	childrenResults := newProcessorResult()

	for i := 0; i < int(node.ChildCount()); i++ {
		result := processNode(node.Child(i), treeData, currentNamespace, callGraph, metadata)
		childrenResults.addResults(result)
	}

	return childrenResults
}
