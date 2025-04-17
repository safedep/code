package callgraph

import (
	"fmt"
	"strings"

	"github.com/safedep/dry/log"
	sitter "github.com/smacker/go-tree-sitter"
)

type processorMetadata struct {
	insideClass    bool
	insideFunction bool
}

type processorResult struct {
	ImmediateCalls       []string // Will be needed to manage assignment-for-call-returned values
	ImmediateAssignments []string
}

func newProcessorResult() processorResult {
	return processorResult{
		ImmediateCalls:       []string{},
		ImmediateAssignments: []string{},
	}
}

// Create a ProcessorResult.addResults which takes variable number of ProcessorResult and adds them to the current ProcessorResult
func (pr *processorResult) addResults(results ...processorResult) {
	for _, result := range results {
		pr.ImmediateAssignments = append(pr.ImmediateAssignments, result.ImmediateAssignments...)
		// @TODO - add some entries in assignment graph basis the pr.immediateCalls
	}
}

type nodeProcessor func(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult

var nodeProcessors map[string]nodeProcessor

func init() {
	nodeProcessors = map[string]nodeProcessor{
		"module":               emptyProcessor,
		"program":              emptyProcessor,
		"expression_statement": emptyProcessor,
		"binary_operator":      binaryOperatorProcessor,
		"identifier":           identifierProcessor,
		"class_definition":     classDefinitionProcessor,
		"function_definition":  functionDefinitionProcessor,
		"call":                 callProcessor,
		"return":               emptyProcessor,
		"return_statement":     functionReturnProcessor,
		"arguments":            emptyProcessor,
		"argument_list":        emptyProcessor,
		"attribute":            attributeProcessor,
		"assignment":           assignmentProcessor,
	}

	// Literals
	for _, symbol := range []string{"string", "number", "integer", "float", "double", "boolean", "null", "undefined", "true", "false"} {
		nodeProcessors[symbol] = literalValueProcessor
	}

	skippedNodeTypes := []string{
		// Imports
		"import_statement", "import", "import_from_statement",
		// Operators
		"+", "-", "*", "/", "%", "**", "//", "=", "+=", "-=", "*=", "/=", "%=",
		// Symbols
		",", ":", ";", ".", "(", ")", "{", "}", "[", "]",
		// Comments and fillers
		"comment", "whitespace", "newline",
		// Other
	}
	for _, symbol := range skippedNodeTypes {
		nodeProcessors[symbol] = skippedProcessor
	}
}

func emptyProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if node == nil {
		return newProcessorResult()
	}

	fmt.Printf("Process children for %s\n", node.Type())

	return processChildren(node, treeData, currentNamespace, callGraph, metadata)
}

func skippedProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if node == nil {
		return newProcessorResult()
	}

	// fmt.Printf("Skipped '%s' - %s under namespace %s\n", node.Type(), node.Content(treeData), currentNamespace)

	return newProcessorResult()
}

func literalValueProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if node == nil {
		return newProcessorResult()
	}

	// fmt.Printf("Assignment of Literal value '%s' - %s under namespace %s\n", node.Type(), node.Content(treeData), currentNamespace)

	result := newProcessorResult()
	result.ImmediateAssignments = append(result.ImmediateAssignments, node.Content(treeData))
	return result
}

func classDefinitionProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if node == nil {
		return newProcessorResult()
	}

	classNameNode := node.ChildByFieldName("name")
	if classNameNode == nil {
		log.Errorf("Class definition without name - %s", node.Content(treeData))
		return newProcessorResult()
	}

	// Class definition has its own scope, hence its own namespace
	classNamespace := currentNamespace + namespaceSeparator + classNameNode.Content(treeData)
	callGraph.AddNode(classNamespace)

	// Assignment is added so that we can resolve class constructor when a function with same name as classname is called
	callGraph.assignments.AddIdentifier(classNamespace)
	callGraph.classConstructors[classNamespace] = true

	instanceKeyword, exists := callGraph.GetInstanceKeyword()
	if exists {
		instanceNamespace := classNamespace + namespaceSeparator + instanceKeyword
		callGraph.AddNode(instanceNamespace)
		callGraph.assignments.AddIdentifier(instanceNamespace)
		fmt.Println("Added instance keyword to call & assignment graph -", instanceNamespace)
	}

	classBody := node.ChildByFieldName("body")
	if classBody == nil {
		log.Errorf("Class definition without body - %s", node.Content(treeData))
		return newProcessorResult()
	}

	metadata.insideClass = true
	processChildren(classBody, treeData, classNamespace, callGraph, metadata)
	metadata.insideClass = false

	fmt.Println("Processed class definition and noted namespace -", classNamespace)

	return newProcessorResult()
}

func functionDefinitionProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if node == nil {
		return newProcessorResult()
	}

	functionNameNode := node.ChildByFieldName("name")
	if functionNameNode == nil {
		log.Errorf("Function definition without name - %s", node.Content(treeData))
		return newProcessorResult()
	}

	funcName := functionNameNode.Content(treeData)

	// Function definition has its own scope, hence its own namespace
	functionNamespace := currentNamespace + namespaceSeparator + funcName

	// Add function to the call graph
	if _, exists := callGraph.Nodes[functionNamespace]; !exists {
		callGraph.AddNode(functionNamespace)
		callGraph.assignments.AddIdentifier(functionNamespace)
		fmt.Println("Added function to call & assignment graph -", functionNamespace)

		// Add virtual fn call from class => classConstructor
		if metadata.insideClass {
			instanceKeyword, exists := callGraph.GetInstanceKeyword()
			if exists {
				instanceNamespace := currentNamespace + namespaceSeparator + instanceKeyword + namespaceSeparator + funcName
				callGraph.AddEdge(instanceNamespace, functionNamespace)
				fmt.Printf("Resolved member function %s => %s\n", instanceNamespace, functionNamespace)
			}
			if funcName == "__init__" {
				callGraph.AddEdge(currentNamespace, functionNamespace)
				fmt.Printf(("Resolved constructor %s => %s\n"), funcName, functionNamespace)
			}
		}
	}

	results := newProcessorResult()

	functionBody := node.ChildByFieldName("body")
	if functionBody != nil {
		metadata.insideFunction = true
		result := processChildren(functionBody, treeData, functionNamespace, callGraph, metadata)
		metadata.insideFunction = false
		results.addResults(result)
	}

	fmt.Println("Processed function definition and noted namespace -", functionNamespace)

	return results
}

func functionReturnProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if node == nil {
		return newProcessorResult()
	}

	// @TODO - Improve this to handle assignments for return values
	// How to handle cross assignment-call
	// eg. def main(): x = y()
	// here, we know, main calls=> y,
	// handle, x assigned=> return values of y

	return processChildren(node, treeData, currentNamespace, callGraph, metadata)
}

func assignmentProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if node == nil {
		return newProcessorResult()
	}

	leftNode := node.ChildByFieldName("left")
	if leftNode == nil {
		log.Errorf("Assignment without left node - %s", node.Content(treeData))
		return newProcessorResult()
	}

	rightNode := node.ChildByFieldName("right")
	if rightNode == nil {
		log.Errorf("Assignment without right node - %s", node.Content(treeData))
		return newProcessorResult()
	}

	// @TODO - Handle multi variate assignments, eg. a, b = 1, 2

	assigneeNamespaces := []string{currentNamespace + namespaceSeparator + leftNode.Content(treeData)}
	if leftNode.Type() == "attribute" {
		// eg. xyz.attr = 1
		// must be resolved to xyz//attr (assigned)=> 1
		attributeResult := attributeProcessor(leftNode, treeData, currentNamespace, callGraph, metadata)
		assigneeNamespaces = attributeResult.ImmediateAssignments
		fmt.Println("Resolved for attr assignment of left -", assigneeNamespaces)
	}

	fmt.Println("Process right node children", rightNode.Type(), rightNode.Content(treeData))
	result := processNode(rightNode, treeData, currentNamespace, callGraph, metadata)

	// Process & note direct calls of processChildren(right,...), and assign returned values in assignment graph

	for _, assigneeNamespace := range assigneeNamespaces {
		for _, immediateCall := range result.ImmediateCalls {
			callGraph.AddEdge(assigneeNamespace, immediateCall)
		}
		for _, immediateAssignment := range result.ImmediateAssignments {
			callGraph.assignments.AddAssignment(assigneeNamespace, immediateAssignment)
		}

		fmt.Printf("Resolved assignment for '%s' = ...\n", assigneeNamespace)
		if callGraph.assignments.Assignments[assigneeNamespace] != nil {
			fmt.Printf("\tAssignment edges -> %v\n", callGraph.assignments.Assignments[assigneeNamespace])
		}
		if callGraph.Nodes[assigneeNamespace] != nil {
			fmt.Printf("\tGraph edges -> %v\n", callGraph.Nodes[assigneeNamespace].CallsTo)
		}
	}
	return newProcessorResult()
}

func attributeProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if node == nil {
		return newProcessorResult()
	}
	fmt.Println("Attribute processor -", node.Type(), node.Content(treeData))

	objectSymbol, attributeQualifierNamespace, err := attributeResolver(node, treeData, currentNamespace, callGraph, metadata)
	if err != nil {
		log.Errorf("Error resolving attribute - %v", err)
		return newProcessorResult()
	}

	targetObjectNamespace, objectResolved := resolveSymbolNamespace(objectSymbol, currentNamespace, callGraph)
	if !objectResolved {
		log.Errorf("Object not found in namespace for attribute - %s (Obj - %s, Attr - %s)", node.Content(treeData), objectSymbol, attributeQualifierNamespace)
		return newProcessorResult()
	}

	resolvedObjectNamespaces := callGraph.assignments.Resolve(targetObjectNamespace)

	fmt.Printf("Resolved attribute for `%s` => %v // %s\n", node.Content(treeData), resolvedObjectNamespaces, attributeQualifierNamespace)

	// We only handle assignments for attributes here eg. xyz.attr
	// 'called' attributes eg. xyz.attr(), are handled in callProcessor directly
	result := newProcessorResult()
	for _, resolvedObjectNamespace := range resolvedObjectNamespaces {
		finalAttributeNamespace := resolvedObjectNamespace + namespaceSeparator + attributeQualifierNamespace
		result.ImmediateAssignments = append(result.ImmediateAssignments, finalAttributeNamespace)
	}

	return result
}

func binaryOperatorProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if node == nil {
		return newProcessorResult()
	}

	leftNode := node.ChildByFieldName("left")
	if leftNode == nil {
		log.Errorf("Binary operator without left node - %s", node.Content(treeData))
		return newProcessorResult()
	}
	rightNode := node.ChildByFieldName("right")
	if rightNode == nil {
		log.Errorf("Binary operator without right node - %s", node.Content(treeData))
		return newProcessorResult()
	}

	results := newProcessorResult()

	leftResult := processNode(leftNode, treeData, currentNamespace, callGraph, metadata)
	rightResult := processNode(rightNode, treeData, currentNamespace, callGraph, metadata)
	results.addResults(leftResult, rightResult)

	// fmt.Printf("Binary operator - '%s', left='%s', right='%s'\n", node.Content(treeData), leftNode.Content(treeData), rightNode.Content(treeData))

	return processChildren(node, treeData, currentNamespace, callGraph, metadata)
}

func identifierProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if node == nil {
		return newProcessorResult()
	}

	// fmt.Printf("Identifier - '%s' under namespace %s\n", node.Content(treeData), currentNamespace)

	result := newProcessorResult()

	// @TODO - Handle identifier during accessing properties
	// eg. xyz.pqr, instance.property.subproperty, etc

	// @TODO - Handle identifier during function calls
	// eg. xyz(), instance.method(), etc

	identifierNamespace, namespaceResolved := resolveSymbolNamespace(node.Content(treeData), currentNamespace, callGraph)

	if namespaceResolved {
		result.ImmediateAssignments = append(result.ImmediateAssignments, identifierNamespace)
		return result
	}

	// If not found in search namespace, we can assume it is a new identifier
	identifierNamespace = currentNamespace + namespaceSeparator + node.Content(treeData)
	result.ImmediateAssignments = append(result.ImmediateAssignments, identifierNamespace)

	return result
}

func callProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if node == nil {
		return newProcessorResult()
	}

	fmt.Printf("Call - '%s' under namespace %s\n", node.Content(treeData), currentNamespace)

	functionNode := node.ChildByFieldName("function")
	argumentsNode := node.ChildByFieldName("arguments")
	if functionNode != nil {
		return functionCallProcessor(functionNode, argumentsNode, treeData, currentNamespace, callGraph, metadata)
	}

	return newProcessorResult()
}

func functionCallProcessor(functionNode *sitter.Node, argumentsNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	result := newProcessorResult()

	functionName := functionNode.Content(treeData)

	markClassAssignment := func(namespace string) {
		if callGraph.classConstructors[namespace] {
			fmt.Printf("Class constructed - %s in fncall for %s\n", namespace, functionName)
			// Include class namespace in assignments for constructors
			result.ImmediateAssignments = append(result.ImmediateAssignments, namespace)
		}
	}

	fmt.Println("Fcp Fn - ", functionNode.Type(), functionNode.Content(treeData))

	// Process function arguments
	if argumentsNode != nil {
		fmt.Println("Fcp ARgs - ", argumentsNode.Type(), argumentsNode.Content(treeData))
		// @TODO - Ideally, the result.ImmediateAssignments should be associated with called function
		// but, we don't have parameter and their positional information, which is a complex task
		// Hence, we're not processing argument results here
		processNode(argumentsNode, treeData, currentNamespace, callGraph, metadata)
	}

	functionNamespace, functionResolvedBySearch := resolveSymbolNamespace(functionName, currentNamespace, callGraph)
	if functionResolvedBySearch {
		fmt.Printf("Call %s searched (direct) & resolved to %s\n", functionName, functionNamespace)
		fmt.Printf("\t%s (calls)=> %s\n", currentNamespace, functionNamespace)
		callGraph.AddEdge(currentNamespace, functionNamespace)
		// result.ImmediateCalls = append(result.ImmediateCalls, functionNamespace)
		markClassAssignment(functionNamespace)
		return result
	}

	// @TODO - Handle class qualified builtins eg. console.log, console.warn etc
	// @TODO - Handle function calls with multiple qualifiers eg. abc.xyz.attr()
	// Resolve qualified function calls, eg. xyz.attr()

	// Process attributes
	functionObjectNode := functionNode.ChildByFieldName("object")
	functionAttributeNode := functionNode.ChildByFieldName("attribute")
	if functionAttributeNode != nil && functionObjectNode != nil {
		// fmt.Println("\tFunction object -", functionObjectNode.Type(), functionObjectNode.Content(treeData))
		// fmt.Println("\tFunction attribute -", functionAttributeNode.Type(), functionAttributeNode.Content(treeData))

		objectSymbol, attributeQualifierNamespace, err := attributeResolver(functionObjectNode, treeData, currentNamespace, callGraph, metadata)
		if err != nil {
			log.Errorf("Error resolving function attribute - %v", err)
			return newProcessorResult()
		}
		finalAttributeNamespace := functionAttributeNode.Content(treeData)
		if attributeQualifierNamespace != "" {
			finalAttributeNamespace = attributeQualifierNamespace + namespaceSeparator + finalAttributeNamespace
		}
		// fmt.Printf("\tResolved fn call attribute for `%s` => %s // %s\n", functionNode.Content(treeData), objectSymbol, finalAttributeNamespace)

		objectNamespace, functionResolvedByObjectQualifiedSearch := resolveSymbolNamespace(objectSymbol, currentNamespace, callGraph)

		if functionResolvedByObjectQualifiedSearch {
			resolvedObjectNamespaces := callGraph.assignments.Resolve(objectNamespace)
			// fmt.Printf("\tObj %s traversed (object qualified) & resolved to %v\n", objectNamespace, resolvedObjectNamespaces)
			for _, resolvedObjectNamespace := range resolvedObjectNamespaces {
				functionNamespace := resolvedObjectNamespace + namespaceSeparator + finalAttributeNamespace
				fmt.Printf("Call %s searched (attr qualified) & resolved to %s\n", functionName, functionNamespace)
				callGraph.AddEdge(currentNamespace, functionNamespace)
				// result.ImmediateCalls = append(result.ImmediateCalls, functionNamespace)
				markClassAssignment(functionNamespace)
			}
			return result
		}
	}

	// @TODO - Rethink on this
	// if not found, possibility of hoisting (declared later)

	// @TODO - In order to handle function assignment to a variable, modify below code to search assignment graph also for scoped namespaces

	// @TODO - Handle argument assignment
	// eg. for def add(a, b)
	// if used as, add(x,y), we must assign add//a => x, add//b => y
	// argumentNode := node.ChildByFieldName("arguments")

	// Builtin assignment already available
	// @TODO - Handle class qualified builtins eg. console.log, console.warn etc

	log.Errorf("Couldn't process function call - %s", functionName)
	return newProcessorResult()
}

// Search symbol in parent namespaces (from self to parent to  grandparent ...)
// eg. namespace - nestNestedFn.py//nestParent//nestChild, callTarget - outerfn1
// try searching for outerfn1 in graph with all scope levels
// eg. search nestNestedFn.py//nestParent//nestChild//outerfn1
// then nestNestedFn.py//nestParent//outerfn1 then nestNestedFn.py//outerfn1 and so on
func resolveSymbolNamespace(symbol string, currentNamespace string, callGraph *CallGraph) (string, bool) {
	if symbol == "" {
		return "", false
	}

	for i := strings.Count(currentNamespace, namespaceSeparator) + 1; i >= 0; i-- {
		searchNamespace := strings.Join(strings.Split(currentNamespace, namespaceSeparator)[:i], namespaceSeparator) + namespaceSeparator + symbol
		if i == 0 {
			searchNamespace = symbol
		}

		// Note - We're searching in assignment graph currently, since callgraph includes only nodes from defined functions, however assignment graph also has imported function items
		if _, exists := callGraph.assignments.Assignments[searchNamespace]; exists {
			return searchNamespace, true
		}
	}

	return "", false
}

// Resolves a attribute eg. xyz.attr.subattr -> xyz, attr//subattr
// Returns objectSymbol, attributeQualifierNamespace, err
// This can be used to identify correct objNamespace for objectSymbol, finally resulting
// objNamespace//attributeQualifierNamespace
func attributeResolver(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) (string, string, error) {
	if node == nil {
		return "", "", fmt.Errorf("fnAttributeResolver - node is nil")
	}

	if node.Type() == "identifier" {
		return node.Content(treeData), "", nil
	}

	// @TODO - In cases of immediate attribution on constructors, we must resolve the objectNode of types - "call" also
	// eg. result = someClass().attr.attr

	if node.Type() != "attribute" {
		return "", "", fmt.Errorf("invalid node type for attribute resolver - %s", node.Type())
	}

	// fmt.Printf("Try resolving attribute - '%s' \n", node.Content(treeData))

	objectNode := node.ChildByFieldName("object")
	subAttributeNode := node.ChildByFieldName("attribute")

	if objectNode == nil {
		return "", "", fmt.Errorf("object node not found for attribute - %s", node.Content(treeData))
	}
	if subAttributeNode == nil {
		return "", "", fmt.Errorf("sub-attribute node not found for attribute - %s", node.Content(treeData))
	}

	// if objectNode.ChildCount() == 1 && objectNode.Child(0).Type() == "identifier" &&
	// 	subAttributeNode.ChildCount() == 1 && subAttributeNode.Child(0).Type() == "identifier" {
	// 	fmt.Printf("Root object - '%s' \n", objectNode.Content(treeData))
	// 	return objectNode.Content(treeData), subAttributeNode.Content(treeData), nil
	// }

	objectSymbol, objectSubAttributeNamespace, err := attributeResolver(objectNode, treeData, currentNamespace, callGraph, metadata)

	if err != nil {
		return "", "", err
	}

	attributeQualifierNamespace := subAttributeNode.Content(treeData)
	if objectSubAttributeNamespace != "" {
		attributeQualifierNamespace = objectSubAttributeNamespace + namespaceSeparator + attributeQualifierNamespace
	}

	return objectSymbol, attributeQualifierNamespace, nil
}
