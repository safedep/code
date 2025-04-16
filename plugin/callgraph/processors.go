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
		// @TODO - add some entreis in assignment graph basis the pr.immediateCalls
	}
}

type nodeProcessor func(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult

var nodeProcessors map[string]nodeProcessor

func init() {
	nodeProcessors = map[string]nodeProcessor{
		"module":                emptyProcessor,
		"program":               emptyProcessor,
		"expression_statement":  emptyProcessor,
		"binary_operator":       binaryOperatorProcessor,
		"identifier":            identifierProcessor,
		"class_definition":      classDefinitionProcessor,
		"function_definition":   functionDefinitionProcessor,
		"call":                  callProcessor,
		"return":                emptyProcessor,
		"return_statement":      functionReturnProcessor,
		"attribute":             attributeProcessor,
		"assignment":            assignmentProcessor,
		"string":                literalValueProcessor,
		"number":                literalValueProcessor,
		"integer":               literalValueProcessor,
		"float":                 literalValueProcessor,
		"double":                literalValueProcessor,
		"boolean":               literalValueProcessor,
		"null":                  literalValueProcessor,
		"comment":               skippedProcessor,
		"whitespace":            skippedProcessor,
		"newline":               skippedProcessor,
		"string_literal":        skippedProcessor,
		"import_statement":      skippedProcessor,
		"import":                skippedProcessor,
		"import_from_statement": skippedProcessor,
		"+":                     skippedProcessor,
		"-":                     skippedProcessor,
		"*":                     skippedProcessor,
		"/":                     skippedProcessor,
		"%":                     skippedProcessor,
		"**":                    skippedProcessor,
		"//":                    skippedProcessor,
		"=":                     skippedProcessor,
		"+=":                    skippedProcessor,
		"-=":                    skippedProcessor,
		"*=":                    skippedProcessor,
		"/=":                    skippedProcessor,
		"%=":                    skippedProcessor,
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
		callGraph.Nodes[functionNamespace] = newGraphNode(functionNamespace)
		fmt.Println("Added function to call graph -", functionNamespace)

		// Add virtual fn call from class => classConstructor
		if metadata.insideClass && funcName == "__init__" {
			callGraph.AddEdge(currentNamespace, functionNamespace)
			fmt.Printf(("Resolved constructor %s under %s\n"), funcName, currentNamespace)
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

	// @TODO - Improve this to handle assinments for return values
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

	leftVar := currentNamespace + namespaceSeparator + leftNode.Content(treeData)

	// @TODO - How to handle assignments with operators
	// eg. a = b + c
	fmt.Println("Process right node children", rightNode.Type(), rightNode.Content(treeData))
	result := processNode(rightNode, treeData, currentNamespace, callGraph, metadata)

	// @TODO - Process & note direct calls of processChildren(right,...), and assign returned values in assignment graph

	for _, immediateCall := range result.ImmediateCalls {
		callGraph.AddEdge(leftVar, immediateCall)
	}
	for _, immediateAssignment := range result.ImmediateAssignments {
		callGraph.assignments.AddAssignment(leftVar, immediateAssignment)
	}

	fmt.Printf("Resolved assignment for '%s' = ...\n", leftNode.Content(treeData))
	if callGraph.assignments.Assignments[leftVar] != nil {
		fmt.Printf("\tAssignment edges -> %v\n", callGraph.assignments.Assignments[leftVar])
	}
	if callGraph.Nodes[leftVar] != nil {
		fmt.Printf("\tGraph edges -> %v\n", callGraph.Nodes[leftVar].CallsTo)
	}
	return newProcessorResult()
}

func attributeProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if node == nil {
		return newProcessorResult()
	}

	// @TODO - Process & note direct calls, assign returned values in assignment graph
	if node.Parent() != nil && node.Parent().Type() == "call" {
		// processing a xyz.attr for xyz.attr() call
		// @TODO - handle multiple qualifiers eg. abc.xyz.attr
		baseNode := node.ChildByFieldName("object")
		attributeNode := node.ChildByFieldName("attribute")
		fmt.Println("Base node -", baseNode.Type(), baseNode.Content(treeData))
		fmt.Println("Attribute node -", attributeNode.Type(), attributeNode.Content(treeData))
		fmt.Printf("Resolved fn call attribute - `%s` = %v\n", node.Content(treeData), currentNamespace)
	}

	// @TODO - Handle attribute during accessing properties
	// eg. xyz.pqr, instance.property.subproperty, etc

	return newProcessorResult()
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

	fmt.Printf("Binary operator - '%s', left='%s', right='%s'\n", node.Content(treeData), leftNode.Content(treeData), rightNode.Content(treeData))

	return processChildren(node, treeData, currentNamespace, callGraph, metadata)
}

func identifierProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if node == nil {
		return newProcessorResult()
	}

	fmt.Printf("Identifier - '%s' under namespace %s\n", node.Content(treeData), currentNamespace)

	result := newProcessorResult()

	// @TODO - Handle identifier during accessing properties
	// eg. xyz.pqr, instance.property.subproperty, etc

	// @TODO - Handle identifier during function calls
	// eg. xyz(), instance.method(), etc

	identifierNamespace := currentNamespace + namespaceSeparator + node.Content(treeData)
	result.ImmediateAssignments = append(result.ImmediateAssignments, identifierNamespace)

	return result
}

func callProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if node == nil {
		return newProcessorResult()
	}

	fmt.Printf("Call - '%s' under namespace %s\n", node.Content(treeData), currentNamespace)

	result := newProcessorResult()

	functionNode := node.ChildByFieldName("function")
	if functionNode != nil {
		functionName := functionNode.Content(treeData)
		fmt.Printf("Handle call to function - '%s'\n", functionName)

		// @TODO - Handle argument assignment
		// eg. for def add(a, b)
		// if used as, add(x,y), we must assign add//a => x, add//b => y
		// argumentNode := node.ChildByFieldName("arguments")

		// Search function in parent namespaces (from self to parent to  grandparent ...)

		// Search for the call target node at different scopes in the graph
		// eg. namespace - nestNestedFn.py//nestParent//nestChild, callTarget - outerfn1
		// try searching for outerfn1 in graph with all scope levels
		// eg. search nestNestedFn.py//nestParent//nestChild//outerfn1
		// then nestNestedFn.py//nestParent//outerfn1 then nestNestedFn.py//outerfn1 and so on

		// @TODO - Rethink on this
		// if not found, then use currentNamespace to build it
		// like, nestNestedFn.py//nestParent//nestChild//outerfn1

		foundInSearchScopes := false
		for i := strings.Count(currentNamespace, namespaceSeparator) + 1; i >= 0; i-- {
			searchNamespace := strings.Join(strings.Split(currentNamespace, namespaceSeparator)[:i], namespaceSeparator) + namespaceSeparator + functionName
			if i == 0 {
				searchNamespace = functionName
			}
			if _, exists := callGraph.Nodes[searchNamespace]; exists {
				fmt.Printf("Call %s searched & resolved to %s\n", functionName, searchNamespace)
				callGraph.AddEdge(currentNamespace, searchNamespace)
				foundInSearchScopes = true
				break
			}
		}

		// Builtin assignment already available
		// @TODO - Hanlde class qualified builtins eg. console.log, console.warn etc
		// @TODO - In order to handle function assigned to a variable, modify below code to search assignment graph also for scoped namespaces
		if !foundInSearchScopes && callGraph.assignments.Assignments[functionName] != nil {
			callGraph.AddEdge(currentNamespace, functionName)
		}
	}

	return result
}
