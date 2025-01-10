package callgraph

import (
	"context"
	"fmt"
	"strings"

	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
	"github.com/safedep/code/pkg/helpers"
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

	treeData, err := tree.Data()
	if err != nil {
		return fmt.Errorf("failed to get tree data: %w", err)
	}

	cg, err := buildCallGraph(tree, lang, treeData, file.Name())

	if err != nil {
		return fmt.Errorf("failed to build call graph: %w", err)
	}

	return p.callgraphCallback(cg)
}

// buildCallGraph builds a call graph from the syntax tree
func buildCallGraph(tree core.ParseTree, lang core.Language, treeData *[]byte, filePath string) (*CallGraph, error) {
	astRootNode := tree.Tree().RootNode()

	imports, err := lang.Resolvers().ResolveImports(tree)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve imports: %w", err)
	}

	// Required to map identifiers to imported modules as assignments
	importedIdentifierNamespaces := parseImportedIdentifierNamespaces(imports)

	fmt.Println()
	fmt.Println("Imported identifier => namespace:")
	for identifier, namespace := range importedIdentifierNamespaces {
		fmt.Printf("  %s => %s\n", identifier, namespace)
	}
	fmt.Println()

	callGraph := NewCallGraph(filePath, importedIdentifierNamespaces)

	// Add root node to the call graph
	callGraph.Nodes[filePath] = newGraphNode(filePath)

	traverseTree(astRootNode, treeData, callGraph, filePath, filePath, "", false)
	fmt.Println()

	return callGraph, nil
}

func traverseTree(node *sitter.Node, treeData *[]byte, callGraph *CallGraph, filePath string, currentNamespace string, classNamespace string, insideClass bool) {
	if node == nil {
		return
	}
	fmt.Println("Traverse ", node.Type(), "with content:", node.Content(*treeData), "with namespace:", currentNamespace)

	switch node.Type() {
	case "function_definition":
		nameNode := node.ChildByFieldName("name")
		if nameNode != nil {
			funcName := nameNode.Content(*treeData)
			if insideClass {
				currentNamespace = classNamespace + namespaceSeparator + funcName
			} else {
				currentNamespace = currentNamespace + namespaceSeparator + funcName
			}

			if _, exists := callGraph.Nodes[currentNamespace]; !exists {
				callGraph.Nodes[currentNamespace] = newGraphNode(currentNamespace)
				//@TODO - Class Constructor edge must be language agnostic, (__init__ ) must be obtained from lang. For java, it would be classname itself, etc
				if insideClass && funcName == "__init__" {
					callGraph.AddEdge(classNamespace, currentNamespace)
				}
			}
		}
	case "class_definition":
		// Handle class definitions
		className := node.ChildByFieldName("name").Content(*treeData)
		classNamespace = currentNamespace + namespaceSeparator + className
		insideClass = true
	case "assignment":
		leftNode := node.ChildByFieldName("left")
		rightNode := node.ChildByFieldName("right")
		if leftNode != nil && rightNode != nil {
			leftVar := leftNode.Content(*treeData)
			rightTargets := resolveTargets(rightNode, *treeData, currentNamespace, callGraph)
			for _, rightTarget := range rightTargets {
				callGraph.assignments.AddAssignment(leftVar, rightTarget)
			}
		}
	case "attribute":
		// processing a xyz.attr for xyz.attr() call
		if node.Parent().Type() == "call" {
			baseNode := node.ChildByFieldName("object")
			attributeNode := node.ChildByFieldName("attribute")
			if baseNode != nil && attributeNode != nil {
				// Resolve base object using the assignment graph at different scopes
				fmt.Printf("Try resolving target call %s.%s at %s\n", baseNode.Content(*treeData), attributeNode.Content(*treeData), currentNamespace)
				baseTargets := resolveTargets(baseNode, *treeData, currentNamespace, callGraph)
				fmt.Printf("Processing fn call as %s.%s() on base targets : %v \n", baseNode.Content(*treeData), attributeNode.Content(*treeData), baseTargets)

				for _, baseTarget := range baseTargets {
					fmt.Printf("Processing fn call as %s.%s() on base as %s \n", baseNode.Content(*treeData), attributeNode.Content(*treeData), baseTarget)
					attributeName := attributeNode.Content(*treeData)
					targetNamespace := baseTarget + namespaceSeparator + attributeName
					_, existed := callGraph.Nodes[targetNamespace]
					fmt.Printf("Attr %s resolved to %s, exists: %t\n", attributeName, targetNamespace, existed)

					// Check if resolved target exists in the call graph
					if _, exists := callGraph.Nodes[targetNamespace]; exists && targetNamespace != "" {
						fmt.Println("Add attr edge from", currentNamespace, "to", targetNamespace)
						callGraph.AddEdge(currentNamespace, targetNamespace)
					} else if _, exists := callGraph.importedIdentifierNamespaces[baseTarget]; exists {
						fmt.Println("Add attr edge from", currentNamespace, "to module namespace", baseTarget+namespaceSeparator+attributeName)
						callGraph.AddEdge(currentNamespace, baseTarget+namespaceSeparator+attributeName)
					}
				}
			}
		}
	case "call":
		fmt.Println("Traverse 'call' with content:", node.Content(*treeData), "with namespace:", currentNamespace, "insideClass:", insideClass, "node type:", node.Type())

		// Extract call target
		targetNode := node.ChildByFieldName("function")
		if targetNode != nil {
			callTarget := targetNode.Content(*treeData)

			// Search for the call target node at different scopes in the graph
			// eg. namespace - nestNestedFn.py//nestParent//nestChild, callTarget - outerfn1
			// try searching for outerfn1 in graph with all scope levels
			// eg. search nestNestedFn.py//nestParent//nestChild//outerfn1
			// then nestNestedFn.py//nestParent//outerfn1 then nestNestedFn.py//outerfn1 and so on
			// if not found, then use currentNamespace to build it
			// like, nestNestedFn.py//nestParent//nestChild//outerfn1

			targetNamespaces := []string{}
			for i := strings.Count(currentNamespace, namespaceSeparator) + 1; i >= 0; i-- {
				searchNamespace := strings.Join(strings.Split(currentNamespace, namespaceSeparator)[:i], namespaceSeparator) + namespaceSeparator + callTarget
				if i == 0 {
					searchNamespace = callTarget
				}
				fmt.Printf("searching %s in scoped - %s\n", callTarget, searchNamespace)
				// check in graph
				if _, exists := callGraph.Nodes[searchNamespace]; exists {
					targetNamespaces = append(targetNamespaces, searchNamespace)
					break
				}
			}
			if len(targetNamespaces) == 0 {
				// If namespace not found in available scopes in the graph, try to resolve it from imported namespaces
				if assignedNamespaces := callGraph.assignments.Resolve(callTarget); len(assignedNamespaces) > 0 {
					fmt.Println("Resolve imported target for", callTarget, ":", assignedNamespaces)
					targetNamespaces = assignedNamespaces
				}
				// else {
				// // @TODO - rethink this
				// 	if insideClass {
				// 		targetNamespaces = []string{classNamespace + namespaceSeparator + callTarget}
				// 	} else {
				// 		targetNamespaces = []string{currentNamespace + namespaceSeparator + callTarget}
				// 	}
				// }
			}

			// Add edge for function call
			for _, targetNamespace := range targetNamespaces {
				fmt.Println("Adding edge from", currentNamespace, "to", targetNamespace)
				callGraph.AddEdge(currentNamespace, targetNamespace)
			}
		}
	}

	// Recursively analyze children
	for i := 0; i < int(node.ChildCount()); i++ {
		traverseTree(node.Child(i), treeData, callGraph, filePath, currentNamespace, classNamespace, insideClass)
	}
}

func resolveTargets(
	node *sitter.Node,
	treeData []byte,
	currentNamespace string,
	callGraph *CallGraph,
) []string {
	if node == nil {
		return []string{}
	}
	fmt.Printf("Resolve targets for for %s type on %s with namespace %s\n", node.Type(), node.Content(treeData), currentNamespace)

	// Handle variable names directly
	if node.Type() == "identifier" {
		identifier := node.Content(treeData)
		// Check if the identifier maps to something in the assignment graph
		resolvedTargets := callGraph.assignments.Resolve(identifier)
		if len(resolvedTargets) > 0 {
			return resolvedTargets
		}
		// Fallback: return the identifier in the current namespace
		return []string{currentNamespace + namespaceSeparator + identifier}
	}

	// Handle calls, e.g., ClassA() -> resolve to ClassA namespace
	if node.Type() == "call" {
		functionNode := node.ChildByFieldName("function")
		if functionNode != nil {
			functionName := functionNode.Content(treeData)
			// Check if the function is a class in the current or parrent scopes in callgraph graph
			for i := strings.Count(currentNamespace, namespaceSeparator); i >= 0; i-- {
				searchNamespace := strings.Join(strings.Split(currentNamespace, namespaceSeparator)[:i], namespaceSeparator) + namespaceSeparator + functionName
				if i == 0 {
					searchNamespace = functionName
				}
				if _, exists := callGraph.Nodes[searchNamespace]; exists {
					return []string{searchNamespace}
				}
			}
			return []string{currentNamespace + namespaceSeparator + functionName}
		}
	}

	// Handle member access, e.g., obj.attr
	if node.Type() == "attribute" {
		baseNode := node.ChildByFieldName("object")
		attributeNode := node.ChildByFieldName("attribute")
		if baseNode != nil && attributeNode != nil {
			var baseTarget []string = resolveTargets(baseNode, treeData, currentNamespace, callGraph)
			attributeName := attributeNode.Content(treeData)
			var resolvedTargets []string
			for _, base := range baseTarget {
				resolvedTargets = append(resolvedTargets, base+namespaceSeparator+attributeName)
			}
			return resolvedTargets
		}
	}

	// Handle other expressions as fallbacks (e.g., literals, complex expressions)
	return []string{node.Content(treeData)}
}

// Fetches namespaces for imported identifiers
// eg. import pprint is parsed as:
// pprint -> pprint
// eg. from os import listdir as listdirfn, chmod is parsed as:
// listdirfn -> os//listdir
// chmod -> os//chmod
func parseImportedIdentifierNamespaces(imports []*ast.ImportNode) map[string]string {
	importedIdentifierNamespaces := make(map[string]string)
	for _, imp := range imports {
		if imp.IsWildcardImport() {
			continue
		}
		itemNamespace := imp.ModuleItem()
		if itemNamespace == "" {
			itemNamespace = imp.ModuleName()
		} else {
			itemNamespace = imp.ModuleName() + namespaceSeparator + itemNamespace
		}
		identifierKey := helpers.GetFirstNonEmptyString(imp.ModuleAlias(), imp.ModuleItem(), imp.ModuleName())
		importedIdentifierNamespaces[identifierKey] = itemNamespace
	}
	return importedIdentifierNamespaces
}
