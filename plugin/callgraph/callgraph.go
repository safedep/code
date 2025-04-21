package callgraph

import (
	"context"
	"fmt"

	"github.com/safedep/code/core"
	"github.com/safedep/dry/log"
	sitter "github.com/smacker/go-tree-sitter"
)

type CallgraphCallback func(*CallGraph) error

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

var supportedLanguages = []core.LanguageCode{core.LanguageCodePython}

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

	log.Debugf("callgraph - Analyzing tree for language: %s, file: %s\n",
		lang.Meta().Code, file.Name())

	cg, err := buildCallGraph(tree, lang, file.Name())

	if err != nil {
		return fmt.Errorf("failed to build call graph: %w", err)
	}

	return p.callgraphCallback(cg)
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

	// Required to map identifiers to imported modules as assignments
	importedIdentifierNamespaces := parseImportedIdentifierNamespaces(imports, lang)

	fmt.Println()
	fmt.Println("Imported identifier => namespace:")
	for identifier, namespace := range importedIdentifierNamespaces {
		fmt.Printf("  %s => %s\n", identifier, namespace)
	}
	fmt.Println()

	callGraph, err := NewCallGraph(filePath, importedIdentifierNamespaces, tree)
	if err != nil {
		return nil, fmt.Errorf("failed to create call graph: %w", err)
	}

	// Add root node to the call graph
	callGraph.AddNode(filePath)

	// traverseTree(astRootNode, treeData, callGraph, filePath, filePath, "", false)
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

	fmt.Println("Can't process", node.Type(), "with namespace:", currentNamespace, " =>", node.Content(treeData))
	// fmt.Println("Content - ", node.Content(treeData))
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
