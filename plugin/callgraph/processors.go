package callgraph

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/safedep/code/core"
	"github.com/safedep/dry/log"
	sitter "github.com/smacker/go-tree-sitter"
)

type processorMetadata struct {
	insideClass    bool
	insideFunction bool
}

type processorResult struct {
	ImmediateCalls       []*CallGraphNode // Will be needed to manage assignment-for-call-returned values
	ImmediateAssignments []*assignmentNode
}

func newProcessorResult() processorResult {
	return processorResult{
		ImmediateCalls:       []*CallGraphNode{},
		ImmediateAssignments: []*assignmentNode{},
	}
}
func (pr *processorResult) ToString() string {
	result := "Immediate Calls:\n"
	for _, call := range pr.ImmediateCalls {
		result += fmt.Sprintf("\t%s\n", call.Namespace)
	}
	result += "Immediate Assignments:\n"
	for _, assignment := range pr.ImmediateAssignments {
		result += fmt.Sprintf("\t%s\n", assignment.Namespace)
	}
	return result
}

// addResults adds the results of the provided processorResults to the current (callee) processorResult
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
		"block":                emptyProcessor,
		"try_statement":        emptyProcessor,
		"catch_clause":         emptyProcessor,
		"class_body":           emptyProcessor,
		"return":               emptyProcessor,
		"return_statement":     functionReturnProcessor,
		"arguments":            emptyProcessor,
		"argument_list":        emptyProcessor,
		"attribute":            attributeProcessor,
		"assignment":           assignmentProcessor,
		"subscript":            skippedProcessor,

		// Java-specific
		"method_invocation":          methodInvocationProcessor,
		"class_declaration":          classDefinitionProcessor,
		"scoped_type_identifier":     scopedIdentifierProcessor,
		"variable_declarator":        variableDeclaratorProcessor,
		"local_variable_declaration": localVariableDeclarationProcessor,
		"object_creation_expression": objectCreationExpressionProcessor,
		"method_declaration":         functionDefinitionProcessor,
		"assignment_expression":      assignmentProcessor,
	}

	// Literals
	for _, symbol := range []string{"string", "number", "integer", "float", "double", "boolean", "null", "undefined", "true", "false"} {
		nodeProcessors[symbol] = literalValueProcessor
	}

	skippedNodeTypes := []string{
		// Imports
		"import_statement", "import", "import_from_statement", "import_declaration",
		// Comments and fillers
		"comment", "whitespace", "newline", "line_comment",
		// Operators
		"+", "-", "*", "/", "%", "**", "//", "=", "+=", "-=", "*=", "/=", "%=",
		// Symbols
		",", ":", ";", ".", "(", ")", "{", "}", "[", "]",
		// Other
	}
	for _, symbol := range skippedNodeTypes {
		nodeProcessors[symbol] = skippedProcessor
	}
}

func emptyProcessor(emptyNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if emptyNode == nil {
		return newProcessorResult()
	}

	return processChildren(emptyNode, treeData, currentNamespace, callGraph, metadata)
}

func skippedProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if node == nil {
		return newProcessorResult()
	}

	return newProcessorResult()
}

func literalValueProcessor(literalNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if literalNode == nil {
		return newProcessorResult()
	}

	result := newProcessorResult()
	literalNamespace := currentNamespace + namespaceSeparator + literalNode.Content(treeData)
	literalAssignmentNode := callGraph.assignmentGraph.AddIdentifier(literalNamespace, literalNode)
	result.ImmediateAssignments = append(result.ImmediateAssignments, literalAssignmentNode)
	return result
}

func classDefinitionProcessor(classDefNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if classDefNode == nil {
		return newProcessorResult()
	}

	classNameNode := classDefNode.ChildByFieldName("name")
	if classNameNode == nil {
		log.Errorf("Class definition without name - %s", classDefNode.Content(treeData))
		return newProcessorResult()
	}

	// Class definition has its own scope, hence its own namespace
	classNamespace := currentNamespace + namespaceSeparator + classNameNode.Content(treeData)
	callGraph.AddNode(classNamespace, classDefNode)

	// Assignment is added so that we can resolve class constructor when a function with same name as classname is called
	callGraph.assignmentGraph.AddIdentifier(classNamespace, classDefNode)
	callGraph.classConstructors[classNamespace] = true

	instanceKeyword, exists := callGraph.GetInstanceKeyword()
	if exists {
		instanceNamespace := classNamespace + namespaceSeparator + instanceKeyword
		callGraph.AddNode(instanceNamespace, nil) // @TODO - Can't create sitter node for instance keyword
		callGraph.assignmentGraph.AddIdentifier(instanceNamespace, nil)
	}

	classBody := classDefNode.ChildByFieldName("body")
	if classBody == nil {
		log.Errorf("Class definition without body - %s", classDefNode.Content(treeData))
		return newProcessorResult()
	}

	metadata.insideClass = true
	processChildren(classBody, treeData, classNamespace, callGraph, metadata)
	metadata.insideClass = false

	log.Debugf("Register class definition for %s - %s", classNameNode.Content(treeData), classNamespace)

	return newProcessorResult()
}

func functionDefinitionProcessor(functionDefNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if functionDefNode == nil {
		return newProcessorResult()
	}

	functionNameNode := functionDefNode.ChildByFieldName("name")
	if functionNameNode == nil {
		log.Errorf("Function definition without name - %s", functionDefNode.Content(treeData))
		return newProcessorResult()
	}

	treeLanguage, err := callGraph.Tree.Language()
	if err != nil {
		log.Errorf("Error resolving tree language - %v", err)
		return newProcessorResult()
	}

	// Function definition has its own scope, hence its own namespace
	funcName := functionNameNode.Content(treeData)
	functionNamespace := currentNamespace + namespaceSeparator + funcName

	// Java - Register direct call from root namespace to main function
	if treeLanguage.Meta().Code == core.LanguageCodeJava && funcName == "main" {
		rootNamespace := resolveRootNamespaceQualifier(currentNamespace)
		callGraph.AddEdge(rootNamespace, nil, functionNamespace, functionDefNode)
	}

	// Add function to the call graph
	if _, exists := callGraph.Nodes[functionNamespace]; !exists {
		callGraph.AddNode(functionNamespace, functionDefNode)
		callGraph.assignmentGraph.AddIdentifier(functionNamespace, functionDefNode)
		log.Debugf("Register function definition for %s - %s", funcName, functionNamespace)

		// Add virtual fn call from class => classConstructor
		if metadata.insideClass {
			instanceKeyword, exists := callGraph.GetInstanceKeyword()
			if exists {
				instanceNamespace := currentNamespace + namespaceSeparator + instanceKeyword + namespaceSeparator + funcName
				callGraph.AddEdge(instanceNamespace, nil, functionNamespace, functionDefNode) // @TODO - Can't create sitter node for instance keyword
				log.Debugf("Register instance member function definition for %s - %s\n", funcName, instanceNamespace)
			}
			if funcName == "__init__" {
				callGraph.AddEdge(currentNamespace, nil, functionNamespace, functionDefNode) // @TODO - Can't create sitter node for instance keyword
				log.Debugf("Register class constructor for %s", currentNamespace)
			}
		}
	}

	results := newProcessorResult()

	functionBody := functionDefNode.ChildByFieldName("body")
	if functionBody != nil {
		metadata.insideFunction = true
		result := processChildren(functionBody, treeData, functionNamespace, callGraph, metadata)
		metadata.insideFunction = false
		results.addResults(result)
	}

	return results
}

func functionReturnProcessor(fnReturnNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if fnReturnNode == nil {
		return newProcessorResult()
	}

	// @TODO - Improve this to handle assignments for return values
	// How to handle cross assignment-call
	// eg. def main(): x = y()
	// here, we know, main calls=> y,
	// handle, x assigned=> return values of y

	return processChildren(fnReturnNode, treeData, currentNamespace, callGraph, metadata)
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

	assigneeNodes := []*assignmentNode{}

	if leftNode.Type() == "attribute" {
		// eg. xyz.attr = 1
		// must be resolved to xyz//attr (assigned)=> 1
		attributeResult := attributeProcessor(leftNode, treeData, currentNamespace, callGraph, metadata)
		assigneeNodes = attributeResult.ImmediateAssignments
	}

	// Create new fallback assignment node for leftNode if not found
	if len(assigneeNodes) == 0 {
		assigneeNodes = []*assignmentNode{
			callGraph.assignmentGraph.AddIdentifier(currentNamespace+namespaceSeparator+leftNode.Content(treeData), leftNode),
		}
	}

	result := processNode(rightNode, treeData, currentNamespace, callGraph, metadata)

	// Process & note direct calls of processChildren(right,...), and assign returned values in assignment graph

	for _, assigneeNode := range assigneeNodes {
		for _, immediateCall := range result.ImmediateCalls {
			callGraph.AddEdge(assigneeNode.Namespace, assigneeNode.TreeNode, immediateCall.Namespace, immediateCall.TreeNode)
		}
		for _, immediateAssignment := range result.ImmediateAssignments {
			callGraph.assignmentGraph.AddAssignment(assigneeNode.Namespace, assigneeNode.TreeNode, immediateAssignment.Namespace, immediateAssignment.TreeNode)
		}

		// log.Debugf("Resolved assignment for '%s' => %v\n", assigneeNode.Namespace, assigneeNode.AssignedTo)
		// if callGraph.Nodes[assigneeNode.Namespace] != nil {
		// 	log.Debugf("\tGraph edges -> %v\n", callGraph.Nodes[assigneeNode.Namespace].CallsTo)
		// }
	}
	return newProcessorResult()
}

func attributeProcessor(attributeNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if attributeNode == nil {
		return newProcessorResult()
	}

	objectSymbol, attributeQualifierNamespace, err := dissectAttributeQualifier(attributeNode, treeData, currentNamespace, callGraph, metadata)
	if err != nil {
		// log.Debugf("Error resolving attribute - %v", err)
		return newProcessorResult()
	}

	targetObject, objectResolved := searchSymbolInScopeChain(objectSymbol, currentNamespace, callGraph)
	if !objectResolved {
		log.Errorf("Object not found in namespace for attribute - %s (Obj - %s, Attr - %s)", attributeNode.Content(treeData), objectSymbol, attributeQualifierNamespace)
		return newProcessorResult()
	}

	resolvedObjects := callGraph.assignmentGraph.Resolve(targetObject.Namespace)

	// log.Debugf("Resolved attribute for `%s` => %v // %s\n", node.Content(treeData), resolvedObjectNamespaces, attributeQualifierNamespace)

	// We only handle assignments for attributes here eg. xyz.attr
	// 'called' attributes eg. xyz.attr(), are handled in callProcessor directly
	result := newProcessorResult()
	for _, resolvedObject := range resolvedObjects {
		finalAttributeNamespace := resolvedObject.Namespace + namespaceSeparator + attributeQualifierNamespace
		finalAttributeNode := callGraph.assignmentGraph.AddIdentifier(finalAttributeNamespace, attributeNode)
		result.ImmediateAssignments = append(result.ImmediateAssignments, finalAttributeNode)
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

	return results
}

func identifierProcessor(identifierNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if identifierNode == nil {
		return newProcessorResult()
	}

	result := newProcessorResult()

	identifierAssignmentNode, identifierResolved := searchSymbolInScopeChain(identifierNode.Content(treeData), currentNamespace, callGraph)

	if identifierResolved {
		result.ImmediateAssignments = append(result.ImmediateAssignments, identifierAssignmentNode)
		return result
	}

	// If not found iby search, we can assume it is a new identifier
	identifierAssignmentNode = callGraph.assignmentGraph.AddIdentifier(
		currentNamespace+namespaceSeparator+identifierNode.Content(treeData),
		identifierNode,
	)

	result.ImmediateAssignments = append(result.ImmediateAssignments, identifierAssignmentNode)

	return result
}

func callProcessor(callNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if callNode == nil {
		return newProcessorResult()
	}

	functionNode := callNode.ChildByFieldName("function")
	argumentsNode := callNode.ChildByFieldName("arguments")
	if functionNode != nil {
		return functionCallProcessor(functionNode, argumentsNode, treeData, currentNamespace, callGraph, metadata)
	}

	return newProcessorResult()
}

func functionCallProcessor(functionCallNode *sitter.Node, argumentsNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	result := newProcessorResult()

	functionName := functionCallNode.Content(treeData)

	markClassAssignment := func(classAssignmentNode *assignmentNode) {
		if classAssignmentNode != nil && callGraph.classConstructors[classAssignmentNode.Namespace] {
			// Include class namespace in assignments for constructors
			result.ImmediateAssignments = append(result.ImmediateAssignments, classAssignmentNode)
			log.Debugf("Class constructed - %s in fncall for %s\n", classAssignmentNode, functionName)
		}
	}

	// Process function arguments
	if argumentsNode != nil {
		// @TODO - Ideally, the result.ImmediateAssignments should be associated with called function
		// but, we don't have parameter and their positional information, which is a complex task
		// Hence, we're not processing argument results here
		processNode(argumentsNode, treeData, currentNamespace, callGraph, metadata)
	}

	functionAssignmentNode, functionResolvedBySearch := searchSymbolInScopeChain(functionName, currentNamespace, callGraph)
	if functionResolvedBySearch {
		log.Debugf("Call %s searched (direct) & resolved to %s\n", functionName, functionAssignmentNode.Namespace)
		callGraph.AddEdge(currentNamespace, nil, functionAssignmentNode.Namespace, functionAssignmentNode.TreeNode) // Assumption - current namespace exists in the graph
		markClassAssignment(functionAssignmentNode)
		return result
	}

	// @TODO - Handle class qualified builtins eg. console.log, console.warn etc
	// @TODO - Handle function calls with multiple qualifiers eg. abc.xyz.attr()
	// Resolve qualified function calls, eg. xyz.attr()

	// Process attributes
	functionObjectNode := functionCallNode.ChildByFieldName("object")
	functionAttributeNode := functionCallNode.ChildByFieldName("attribute")
	if functionAttributeNode != nil && functionObjectNode != nil {
		log.Debugf("Call %s searched (attr qualified) & resolved to object - %s (%s), attribute - %s (%s) \n", functionName, functionObjectNode.Content(treeData), functionObjectNode.Type(), functionAttributeNode.Content(treeData), functionAttributeNode.Type())

		objectSymbol, attributeQualifierNamespace, err := dissectAttributeQualifier(functionObjectNode, treeData, currentNamespace, callGraph, metadata)
		if err != nil {
			// log.Debugf("Error resolving function attribute - %v", err)
			return newProcessorResult()
		}
		finalAttributeNamespace := functionAttributeNode.Content(treeData)
		if attributeQualifierNamespace != "" {
			finalAttributeNamespace = attributeQualifierNamespace + namespaceSeparator + finalAttributeNamespace
		}

		objectAssignmentNode, functionResolvedByObjectQualifiedSearch := searchSymbolInScopeChain(objectSymbol, currentNamespace, callGraph)

		if functionResolvedByObjectQualifiedSearch {
			resolvedObjectNodes := callGraph.assignmentGraph.Resolve(objectAssignmentNode.Namespace)
			for _, resolvedObjectNode := range resolvedObjectNodes {
				functionNamespace := resolvedObjectNode.Namespace + namespaceSeparator + finalAttributeNamespace

				// log.Debugf("Call %s searched (attr qualified) & resolved to %s\n", functionName, functionNamespace)
				callGraph.AddEdge(currentNamespace, nil, functionNamespace, nil) // @TODO - Assumed current namespace & functionNamespace to be pre-existing

				markClassAssignment(callGraph.assignmentGraph.Assignments[functionNamespace])
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

	// log.Debug("Couldn't process function call - %s", functionName)
	return newProcessorResult()
}

// Search symbol in parent namespaces (from self to parent to  grandparent ...)
// eg. namespace - nestNestedFn.py//nestParent//nestChild, callTarget - outerfn1
// try searching for outerfn1 in graph with all scope levels
// eg. search nestNestedFn.py//nestParent//nestChild//outerfn1
// then nestNestedFn.py//nestParent//outerfn1 then nestNestedFn.py//outerfn1 and so on
func searchSymbolInScopeChain(symbol string, currentNamespace string, callGraph *CallGraph) (*assignmentNode, bool) {
	if symbol == "" {
		return nil, false
	}

	for i := strings.Count(currentNamespace, namespaceSeparator) + 1; i >= 0; i-- {
		searchNamespace := strings.Join(strings.Split(currentNamespace, namespaceSeparator)[:i], namespaceSeparator) + namespaceSeparator + symbol
		if i == 0 {
			searchNamespace = symbol
		}
		// Note - We're searching in assignment graph currently, since callgraph includes only nodes from defined functions, however assignment graph also has imported function items
		searchedAssignmentNode, exists := callGraph.assignmentGraph.Assignments[searchNamespace]
		if exists {
			return searchedAssignmentNode, true
		}
	}

	return nil, false
}

// dissectAttributeQualifier splits provided qualified atribute into qualifiers and returns -
// - objectIdentifier (First qualifier object name)
// - objectQualifierNamespace (remaining qualifiers as namespace)
// - error
// eg. xyz.attr.subattr -> xyz, attr//subattr
//
// This can be used to identify correct objNamespace for objectSymbol, finally resulting
// objNamespace//attributeQualifierNamespace
func dissectAttributeQualifier(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) (string, string, error) {
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

	objectNode := node.ChildByFieldName("object")
	subAttributeNode := node.ChildByFieldName("attribute")

	if objectNode == nil {
		return "", "", fmt.Errorf("object node not found for attribute - %s", node.Content(treeData))
	}
	if subAttributeNode == nil {
		return "", "", fmt.Errorf("sub-attribute node not found for attribute - %s", node.Content(treeData))
	}

	objectSymbol, objectSubAttributeNamespace, err := dissectAttributeQualifier(objectNode, treeData, currentNamespace, callGraph, metadata)

	if err != nil {
		return "", "", err
	}

	attributeQualifierNamespace := subAttributeNode.Content(treeData)
	if objectSubAttributeNamespace != "" {
		attributeQualifierNamespace = objectSubAttributeNamespace + namespaceSeparator + attributeQualifierNamespace
	}

	return objectSymbol, attributeQualifierNamespace, nil
}

// Java-specific ------
func scopedIdentifierProcessor(scopedIdentifierNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if scopedIdentifierNode == nil {
		return newProcessorResult()
	}

	targetObjectIdentifier, objectQualifierNamespace, err := disectScopedIdentifierQualifier(scopedIdentifierNode, treeData, currentNamespace, callGraph)
	if err != nil {
		log.Errorf("error resolving scoped identifier %s - %v", scopedIdentifierNode.Content(treeData), err)
	}

	targetObject, objectResolved := searchSymbolInScopeChain(targetObjectIdentifier, currentNamespace, callGraph)
	if objectResolved {
		resolvedObjects := callGraph.assignmentGraph.Resolve(targetObject.Namespace)

		result := newProcessorResult()
		for _, resolvedObject := range resolvedObjects {
			finalIdentifierNamespace := resolvedObject.Namespace + namespaceSeparator + objectQualifierNamespace
			finalAttributeNode := callGraph.assignmentGraph.AddIdentifier(finalIdentifierNamespace, scopedIdentifierNode)
			result.ImmediateAssignments = append(result.ImmediateAssignments, finalAttributeNode)
		}
		return result
	}

	// Consider this as fully qualified usage of a type without importing it
	// eg. directly using - "java.awt.event.MouseEvent"
	importNamespace := targetObjectIdentifier + namespaceSeparator + objectQualifierNamespace
	callGraph.assignmentGraph.AddIdentifier(importNamespace, scopedIdentifierNode)
	log.Debugf("Scoped identifier %s - fallback to fully qualified import - %s", scopedIdentifierNode.Content(treeData), importNamespace)

	result := newProcessorResult()
	result.ImmediateAssignments = append(result.ImmediateAssignments, callGraph.assignmentGraph.Assignments[importNamespace])

	return result
}

// disectScopedIdentifierQualifier splits provided qualified name into qualifiers and returns -
// - objectIdentifier (First qualifier object name)
// - objectQualifierNamespace (remaining qualifiers as namespace)
// - error
// eg. java.awt.event.MouseEvent => java, awt//event//MouseEvent, nil
func disectScopedIdentifierQualifier(scopedTypeIdentifierNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph) (string, string, error) {
	if scopedTypeIdentifierNode == nil {
		return "", "", fmt.Errorf("scopedIdentifierResolver - node is nil")
	}

	if scopedTypeIdentifierNode.Type() != "scoped_type_identifier" {
		return "", "", fmt.Errorf("invalid node type for scoped identifier resolver - %s", scopedTypeIdentifierNode.Type())
	}

	qualifierList := []string{}
	disectScopedIdentifierQualifierUtil(scopedTypeIdentifierNode, treeData, currentNamespace, callGraph, &qualifierList)

	if len(qualifierList) == 0 {
		return "", "", fmt.Errorf("could not resolve qualifiers for scoped identifier - %s", scopedTypeIdentifierNode.Content(treeData))
	}

	return qualifierList[0], strings.Join(qualifierList[1:], namespaceSeparator), nil
}

func disectScopedIdentifierQualifierUtil(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, qualifierList *[]string) {
	if node.Type() != "scoped_type_identifier" {
		return
	}
	for i := 0; uint32(i) < node.ChildCount(); i++ {
		childNode := node.Child(i)
		switch childNode.Type() {
		case "scoped_type_identifier":
			disectScopedIdentifierQualifierUtil(childNode, treeData, currentNamespace, callGraph, qualifierList)
		case "type_identifier":
			*qualifierList = append(*qualifierList, childNode.Content(treeData))
		}
	}
}

// @TODO - Handle object creation expression in Java eg. new SomeClass()
func objectCreationExpressionProcessor(objectCreationNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if objectCreationNode == nil {
		return newProcessorResult()
	}

	result := newProcessorResult()

	markClassAssignment := func(classAssignmentNode *assignmentNode) {
		if classAssignmentNode != nil {
			// Include class namespace in assignments for constructors
			result.ImmediateAssignments = append(result.ImmediateAssignments, classAssignmentNode)
		}
	}

	constructedClassNode := objectCreationNode.ChildByFieldName("type")
	if constructedClassNode != nil {
		constructedClassName := constructedClassNode.Content(treeData)

		// Resolve symbol - targetObjClassNode ?
		classNode, classResolvedBySearch := searchSymbolInScopeChain(constructedClassName, currentNamespace, callGraph)
		if classResolvedBySearch {
			log.Debugf("Constructor %s searched (direct) & resolved to %s\n", constructedClassName, classNode.Namespace)

			callGraph.AddEdge(currentNamespace, nil, classNode.Namespace, classNode.TreeNode) // Assumption - current namespace exists in the graph

			markClassAssignment(classNode)
		} else if constructedClassNode.Type() == "scoped_type_identifier" {
			// Try resolving scoped identifiers
			scopedIdentifierResult := scopedIdentifierProcessor(constructedClassNode, treeData, currentNamespace, callGraph, metadata)

			for _, scopedIdentifierAssignment := range scopedIdentifierResult.ImmediateAssignments {
				// Register class constructor as Function call in callgraph
				callGraph.AddEdge(currentNamespace, nil, scopedIdentifierAssignment.Namespace, scopedIdentifierAssignment.TreeNode) // Assumption - current namespace exists in the graph

				// Mark assignments for parent
				markClassAssignment(scopedIdentifierAssignment)
			}
		}
	}

	argumentsNode := objectCreationNode.ChildByFieldName("arguments")
	if argumentsNode != nil {
		// Need to handle assignment results from arguments ?
		processNode(argumentsNode, treeData, currentNamespace, callGraph, metadata)
	}

	return result
}

func variableDeclaratorProcessor(declaratorNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if declaratorNode == nil {
		return newProcessorResult()
	}

	accumulatedResult := newProcessorResult()

	declaredValueNode := declaratorNode.ChildByFieldName("value")
	if declaredValueNode != nil {
		accumulatedResult.addResults(
			processNode(declaredValueNode, treeData, currentNamespace, callGraph, metadata),
		)
	}

	declaredVariableNode := declaratorNode.ChildByFieldName("name")
	if declaredVariableNode != nil {
		declaredVariableNamespace := currentNamespace + namespaceSeparator + declaredVariableNode.Content(treeData)

		for _, immediateAssignment := range accumulatedResult.ImmediateAssignments {
			callGraph.assignmentGraph.AddAssignment(declaredVariableNamespace, declaredVariableNode, immediateAssignment.Namespace, immediateAssignment.TreeNode)
		}
	}

	return newProcessorResult()
}

func localVariableDeclarationProcessor(declarationNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if declarationNode == nil {
		return newProcessorResult()
	}

	// Child named "type" defines type of left side of assignment eg. "int" for "int a = 1;"

	declaratorNode := declarationNode.ChildByFieldName("declarator")
	if declaratorNode != nil {
		processNode(declaratorNode, treeData, currentNamespace, callGraph, metadata)
	}

	return newProcessorResult()
}

func methodInvocationProcessor(methodInvocationNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if methodInvocationNode == nil {
		return newProcessorResult()
	}

	argumentsNode := methodInvocationNode.ChildByFieldName("arguments")
	// Process function arguments
	if argumentsNode != nil {
		// @TODO - Ideally, the result.ImmediateAssignments should be associated with called function
		// but, we don't have parameter and their positional information, which is a complex task
		// Hence, we're not processing argument results here
		processNode(argumentsNode, treeData, currentNamespace, callGraph, metadata)
	}

	methodNameNode := methodInvocationNode.ChildByFieldName("name")
	if methodNameNode == nil {
		log.Errorf("Method invocation without name - %s", methodInvocationNode.Content(treeData))
		return newProcessorResult()
	}

	methodName := methodNameNode.Content(treeData)

	// For chained method invocations eg. xyz.method1().method2().method3()
	// we will ignore methodName ie. method3 in this case, and only resolve the first method in the chain
	hasChainedMethodInvocations := false

	methodQualifierObjectNode := methodInvocationNode.ChildByFieldName("object")
	if methodQualifierObjectNode != nil {
		// Extract first called method from chain of method invocations
		// eg. xyz.method1().method2().method3() => xyz.method1
		for methodQualifierObjectNode.Type() == "method_invocation" {
			hasChainedMethodInvocations = true
			nextObjNode := methodQualifierObjectNode.ChildByFieldName("object")
			if nextObjNode == nil {
				break
			}

			// For a method_invocation over new objects, perform object creation expression processing
			// eg. new xyz().method1().method2() => perform only new xyz()
			// @TODO - Immediate members of constructed class can be handled here
			// eg. in new xyz().method1().method2() => xyz//method1 can be resolved
			if nextObjNode.Type() == "object_creation_expression" {
				return objectCreationExpressionProcessor(nextObjNode, treeData, currentNamespace, callGraph, metadata)
			}

			if nextObjNode.Type() != "method_invocation" {
				break
			}
			methodQualifierObjectNode = nextObjNode
		}

		methodObjectQualifierNamespace := resolveQualifierObjectFieldaccess(methodQualifierObjectNode, treeData)
		qualifiers := strings.Split(methodObjectQualifierNamespace, namespaceSeparator)
		callerObjectNamespaces := []string{}

		if len(qualifiers) > 0 {
			rootObjKeyword := qualifiers[0]
			rootObjNode, rootObjNodeExists := searchSymbolInScopeChain(rootObjKeyword, currentNamespace, callGraph)

			var rootCallerObjAssignments []*assignmentNode
			if rootObjNodeExists {
				rootCallerObjAssignments = callGraph.assignmentGraph.Resolve(rootObjNode.Namespace)
			} else {
				// If root object is not found, we can assume it is a fully qualified object
				// eg. sun.reflect.Method here, sun couldn't be identified, so assume its a library root keyword
				callGraph.AddNode(rootObjKeyword, nil)                       // @TODO - Can't create sitter node for fully qualified object
				callGraph.assignmentGraph.AddIdentifier(rootObjKeyword, nil) // @TODO - Can't create sitter node for fully qualified object
				rootCallerObjAssignments = callGraph.assignmentGraph.Resolve(rootObjKeyword)
			}

			// len(qualifiers)>1 means methodObjectQualifierNamespace includes a separator eg. "xyz//attr"
			if len(qualifiers) > 1 {
				remainingQualifiersNamespaceSuffix := strings.Join(qualifiers[1:], namespaceSeparator)
				for _, rootCallerObjAssignment := range rootCallerObjAssignments {
					callerObjectNamespaces = append(callerObjectNamespaces, rootCallerObjAssignment.Namespace+namespaceSeparator+remainingQualifiersNamespaceSuffix)
				}
			} else {
				if rootObjNodeExists {
					for _, rootCallerObjAssignment := range rootCallerObjAssignments {
						callerObjectNamespaces = append(callerObjectNamespaces, rootCallerObjAssignment.Namespace)
					}
				} else {
					callerObjectNamespaces = append(callerObjectNamespaces, rootObjKeyword)
				}
			}
		}

		for _, callerObjectNamespace := range callerObjectNamespaces {
			calledNamespace := callerObjectNamespace
			if !hasChainedMethodInvocations {
				calledNamespace = calledNamespace + namespaceSeparator + methodName
			}
			log.Debugf("Method invocation %s searched (object qualified) & resolved to - %s\n", methodName, calledNamespace)
			callGraph.AddEdge(currentNamespace, nil, calledNamespace, methodInvocationNode) // Assumption - current namespace exists in the graph
		}

		return newProcessorResult()
	}

	// Simple function call lookup without any qualifiers
	functionAssignmentNode, functionResolvedBySearch := searchSymbolInScopeChain(methodName, currentNamespace, callGraph)
	if functionResolvedBySearch {
		log.Debugf("Method invocation %s searched (direct) & resolved to %s\n", methodName, functionAssignmentNode.Namespace)
		callGraph.AddEdge(currentNamespace, nil, functionAssignmentNode.Namespace, functionAssignmentNode.TreeNode) // Assumption - current namespace exists in the graph
		return newProcessorResult()
	}

	log.Debugf("Method invocation %s couldn't be processed", methodName)

	return newProcessorResult()
}

var methodInvocationNormaliserRegexp = regexp.MustCompile(`[-()\n ]`)

// resolveQualifierObjectFieldaccess resolves the namespace for object and qualified field
// and removes if any call Parentheses symbols present; intended to be used by methodInvocationProcessor
// eg. for "xyz.attr.subattr.fncall", it returns xyz//attr//subattr//fncall"]
func resolveQualifierObjectFieldaccess(invokedObjNode *sitter.Node, treeData []byte) string {
	if invokedObjNode == nil {
		return ""
	}

	normalisedString := methodInvocationNormaliserRegexp.ReplaceAllString(invokedObjNode.Content(treeData), "")

	return strings.ReplaceAll(normalisedString, ".", namespaceSeparator)
}
