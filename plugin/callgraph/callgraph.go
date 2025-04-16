package callgraph

import (
	"context"
	"fmt"

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
	importedIdentifierNamespaces := parseImportedIdentifierNamespaces(imports)

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
	callGraph.Nodes[filePath] = newGraphNode(filePath)

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

// func traverseTree(node *sitter.Node, treeData *[]byte, callGraph *CallGraph, filePath string, currentNamespace string, classNamespace string, insideClass bool, metadata processorMetadata) {
// 	if node == nil {
// 		return
// 	}

// 	nodeProcessor, exists := nodeProcessors[node.Type()]
// 	if exists {
// 		nodeProcessor(node, *treeData, currentNamespace, callGraph, metadata)
// 		return
// 	}

// 	// fmt.Println("Traverse ", node.Type(), "with content:", node.Content(*treeData), "with namespace:", currentNamespace)

// 	skipChildren := false

// 	switch node.Type() {
// 	case "function_definition":
// 		nameNode := node.ChildByFieldName("name")
// 		if nameNode != nil {
// 			funcName := nameNode.Content(*treeData)
// 			if insideClass {
// 				currentNamespace = classNamespace + namespaceSeparator + funcName
// 			} else {
// 				currentNamespace = currentNamespace + namespaceSeparator + funcName
// 			}

// 			if _, exists := callGraph.Nodes[currentNamespace]; !exists {
// 				callGraph.Nodes[currentNamespace] = newGraphNode(currentNamespace)
// 				// @TODO - Class Constructor edge must be language agnostic, (__init__ ) must be obtained from lang. For java, it would be classname itself, etc
// 				if insideClass && funcName == "__init__" {
// 					fmt.Printf(("Resolved constructor %s for class %s => %s\n"), funcName, classNamespace, currentNamespace)
// 					callGraph.AddEdge(classNamespace, currentNamespace)
// 				}
// 			}
// 		}
// 	case "class_definition":
// 		// Handle class definitions
// 		className := node.ChildByFieldName("name").Content(*treeData)
// 		classNamespace = currentNamespace + namespaceSeparator + className
// 		insideClass = true
// 		fmt.Println("Resolved class and noted namespace", classNamespace)
// 		skipChildren = true
// 	case "assignment":
// 		leftNode := node.ChildByFieldName("left")
// 		rightNode := node.ChildByFieldName("right")
// 		if leftNode != nil && rightNode != nil {
// 			// @TODO - Handle multi variate assignments, eg. a, b = 1, 2
// 			leftVar := currentNamespace + namespaceSeparator + leftNode.Content(*treeData)
// 			rightTargets := processNodeOld(rightNode, *treeData, currentNamespace, callGraph, metadata)
// 			for _, rightTarget := range rightTargets {
// 				callGraph.assignments.AddAssignment(leftVar, rightTarget)
// 			}
// 			fmt.Printf("Resolved assignment - `%s` = %v, \n\tAll edges -> %v \n", leftNode.Content(*treeData), rightTargets, callGraph.assignments.Assignments[leftVar])
// 		}
// 		skipChildren = true
// 	case "attribute":
// 		// processing a xyz.attr for xyz.attr() call
// 		// a.b.xyz().attr
// 		// a.ab
// 		// xyz()
// 		// a.b().xyz
// 		if node.Parent().Type() == "call" {
// 			baseNode := node.ChildByFieldName("object")
// 			attributeNode := node.ChildByFieldName("attribute")
// 			if baseNode != nil && attributeNode != nil {
// 				// Resolve base object using the assignment graph at different scopes
// 				fmt.Printf("Try resolving target call %s.%s at %s\n", baseNode.Content(*treeData), attributeNode.Content(*treeData), currentNamespace)

// 				// b = abc
// 				// b = xyz
// 				// b.attr()

// 				// b = SomeClass
// 				// filename//SomeClass//attr
// 				baseTargets := processNodeOld(baseNode, *treeData, currentNamespace, callGraph, metadata)
// 				fmt.Printf("Processing fn call as %s.%s() on base targets : %v \n", baseNode.Content(*treeData), attributeNode.Content(*treeData), baseTargets)

// 				for _, baseTarget := range baseTargets {
// 					fmt.Printf("Processing fn call as %s.%s() on base as %s \n", baseNode.Content(*treeData), attributeNode.Content(*treeData), baseTarget)
// 					attributeName := attributeNode.Content(*treeData)
// 					targetNamespace := baseTarget + namespaceSeparator + attributeName
// 					_, existed := callGraph.Nodes[targetNamespace]
// 					fmt.Printf("Attr %s resolved to %s, exists: %t\n", attributeName, targetNamespace, existed)

// 					// Check if resolved target exists in the call graph
// 					if _, exists := callGraph.Nodes[targetNamespace]; exists && targetNamespace != "" {
// 						fmt.Println("Add attr edge from", currentNamespace, "to", targetNamespace)
// 						callGraph.AddEdge(currentNamespace, targetNamespace)
// 					} else if _, exists := callGraph.importedIdentifierNamespaces[baseTarget]; exists {
// 						fmt.Println("Add attr edge from", currentNamespace, "to module namespace", baseTarget+namespaceSeparator+attributeName)
// 						callGraph.AddEdge(currentNamespace, baseTarget+namespaceSeparator+attributeName)
// 					}
// 				}
// 			}
// 		}
// 	case "call":
// 		// fmt.Println("Traverse 'call' with content:", node.Content(*treeData), "with namespace:", currentNamespace, "insideClass:", insideClass, "node type:", node.Type())

// 		// Extract call target
// 		targetNode := node.ChildByFieldName("function")
// 		if targetNode != nil {
// 			callTarget := targetNode.Content(*treeData)

// 			// if not found, then use currentNamespace to build it
// 			// like, nestNestedFn.py//nestParent//nestChild//outerfn1

// 			targetNamespaces := []string{}
// 			for i := strings.Count(currentNamespace, namespaceSeparator) + 1; i >= 0; i-- {
// 				searchNamespace := strings.Join(strings.Split(currentNamespace, namespaceSeparator)[:i], namespaceSeparator) + namespaceSeparator + callTarget
// 				if i == 0 {
// 					searchNamespace = callTarget
// 				}
// 				// check in graph
// 				if _, exists := callGraph.Nodes[searchNamespace]; exists {
// 					fmt.Printf("searched & found %s in scoped - %s\n", callTarget, searchNamespace)
// 					targetNamespaces = append(targetNamespaces, searchNamespace)
// 					break
// 				}
// 			}
// 			if len(targetNamespaces) == 0 {
// 				// If namespace not found in available scopes in the graph, try to resolve it from imported/builtin namespaces
// 				if assignedNamespaces := callGraph.assignments.Resolve(callTarget); len(assignedNamespaces) > 0 {
// 					fmt.Println("Resolve imported/builtin target for", callTarget, ":", assignedNamespaces)
// 					targetNamespaces = assignedNamespaces
// 				}
// 				// else {
// 				// // @TODO - rethink this
// 				// 	if insideClass {
// 				// 		targetNamespaces = []string{classNamespace + namespaceSeparator + callTarget}
// 				// 	} else {
// 				// 		targetNamespaces = []string{currentNamespace + namespaceSeparator + callTarget}
// 				// 	}
// 				// }
// 			}

// 			if skipChildren {
// 				fmt.Println("Skip children for", node.Type(), "with content:", node.Content(*treeData), "with namespace:", currentNamespace, "insideClass:", insideClass)
// 			}

// 			if !skipChildren {
// 				// Add edge for function call
// 				for _, targetNamespace := range targetNamespaces {
// 					fmt.Println("Adding edge from", currentNamespace, "to", targetNamespace)
// 					callGraph.AddEdge(currentNamespace, targetNamespace)
// 				}
// 			}
// 		}
// 	}

// 	// Recursively analyze children
// 	for i := 0; i < int(node.ChildCount()); i++ {
// 		traverseTree(node.Child(i), treeData, callGraph, filePath, currentNamespace, classNamespace, insideClass, metadata)
// 	}
// }

// func processNodeOld(
// 	node *sitter.Node,
// 	treeData []byte,
// 	currentNamespace string,
// 	callGraph *CallGraph,
// 	metadata processorMetadata,
// ) []string {
// 	if node == nil {
// 		return []string{}
// 	}

// 	fmt.Printf("Process '%s' - %s under namespace %s\n", node.Type(), node.Content(treeData), currentNamespace)

// 	nodeType := node.Type()

// 	switch nodeType {
// 	case "identifier", "string":
// 		return []string{currentNamespace + namespaceSeparator + node.Content(treeData)}
// 	case "binary_operator":
// 		// Handle binary operations, e.g., a + b, add(x,y) + sub(p,r), etc
// 		leftNode := node.ChildByFieldName("left")
// 		rightNode := node.ChildByFieldName("right")
// 		if leftNode != nil && rightNode != nil {
// 			leftTargets := processNodeOld(leftNode, treeData, currentNamespace, callGraph, metadata)
// 			rightTargets := processNodeOld(rightNode, treeData, currentNamespace, callGraph, metadata)
// 			resolvedTargets := slices.Concat(leftTargets, rightTargets)
// 			return resolvedTargets
// 		}
// 	case "call":
// 		// Handle calls, e.g., ClassA() -> resolve to ClassA namespace
// 		functionNode := node.ChildByFieldName("function")
// 		if functionNode != nil {
// 			functionName := functionNode.Content(treeData)
// 			// Check if the function is a class in the current or parrent scopes in callgraph graph
// 			for i := strings.Count(currentNamespace, namespaceSeparator); i >= 0; i-- {
// 				searchNamespace := strings.Join(strings.Split(currentNamespace, namespaceSeparator)[:i], namespaceSeparator) + namespaceSeparator + functionName
// 				if i == 0 {
// 					searchNamespace = functionName
// 				}
// 				if _, exists := callGraph.Nodes[searchNamespace]; exists {
// 					return []string{searchNamespace}
// 				}
// 			}
// 			return []string{currentNamespace + namespaceSeparator + functionName}
// 		}
// 	case "attribute":
// 		// Handle member access, e.g., obj.attr
// 		baseNode := node.ChildByFieldName("object")
// 		attributeNode := node.ChildByFieldName("attribute")
// 		if baseNode != nil && attributeNode != nil {
// 			var baseTarget []string = processNodeOld(baseNode, treeData, currentNamespace, callGraph, metadata)
// 			attributeName := attributeNode.Content(treeData)

// 			var resolvedTargets []string
// 			for _, base := range baseTarget {
// 				resolvedTargets = append(resolvedTargets, base+namespaceSeparator+attributeName)
// 			}
// 			return resolvedTargets
// 		}
// 	}

// 	// Handle other expressions as fallbacks (e.g., literals, complex expressions)
// 	return []string{node.Content(treeData)}
// }

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
