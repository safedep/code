package callgraph

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/safedep/code/core"
	"github.com/safedep/dry/log"
	sitter "github.com/smacker/go-tree-sitter"
)

const (
	// Maximum recursion depth for nested member expressions and attributes
	// Prevents infinite recursion in pathological JavaScript files
	maxMemberExpressionDepth = 10
	maxAttributeDepth        = 10
	maxProcessingDepth       = 50

	// Maximum number of resolved objects to process for polymorphic variables
	// Prevents combinatorial explosion in complex type scenarios
	maxPolymorphicResolutions = 20
)

type processorMetadata struct {
	insideClass     bool
	insideFunction  bool
	processingDepth int // Track recursion depth for processChildren
}

type processorResult struct {
	ImmediateCallRefs    []CallReference
	ImmediateAssignments []*assignmentNode
}

func newProcessorResult() processorResult {
	return processorResult{
		ImmediateCallRefs:    []CallReference{},
		ImmediateAssignments: []*assignmentNode{},
	}
}

func (pr *processorResult) ToString() string {
	result := "Immediate Calls:\n"
	for _, call := range pr.ImmediateCallRefs {
		result += fmt.Sprintf("\t%s\n", call.CalleeNamespace)
	}
	result += "Immediate Assignments:\n"
	for _, assignment := range pr.ImmediateAssignments {
		result += fmt.Sprintf("\t%s\n", assignment.Namespace)
	}
	return result
}

// addAssignments adds the nodes that exist. The assignment graph returns nil
// past its node limit.
func (pr *processorResult) addAssignments(nodes ...*assignmentNode) {
	for _, node := range nodes {
		if node != nil {
			pr.ImmediateAssignments = append(pr.ImmediateAssignments, node)
		}
	}
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

// languageNodeProcessors holds the processors of a language for node types
// whose shape differs from the shared processors, or that another grammar
// uses with another meaning. processNode looks here first.
var languageNodeProcessors = map[core.LanguageCode]map[string]nodeProcessor{}

// registerLanguageProcessors adds the node processors of a language. A
// language file calls it from init.
func registerLanguageProcessors(code core.LanguageCode, processors map[string]nodeProcessor) {
	if languageNodeProcessors[code] == nil {
		languageNodeProcessors[code] = map[string]nodeProcessor{}
	}
	for nodeType, processor := range processors {
		languageNodeProcessors[code][nodeType] = processor
	}
}

func init() {
	nodeProcessors = map[string]nodeProcessor{
		"module":               emptyProcessor,
		"program":              emptyProcessor,
		"expression_statement": expressionStatementProcessorWrapper,
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
		"ternary_expression":   ternaryExpressionProcessor,

		// Java-specific
		"consequence":                skipResultsProcessor,
		"alternative":                skipResultsProcessor,
		"method_invocation":          methodInvocationProcessor,
		"class_declaration":          classDefinitionProcessor,
		"scoped_type_identifier":     scopedIdentifierProcessor,
		"variable_declarator":        variableDeclaratorProcessor,
		"local_variable_declaration": localVariableDeclarationProcessor,
		"object_creation_expression": objectCreationExpressionProcessor,
		"method_declaration":         goMethodDeclarationProcessorWrapper,
		"assignment_expression":      assignmentProcessor,

		// Go and JavaScript shared
		"call_expression":      callExpressionProcessorWrapper,
		"function_declaration": functionDeclarationProcessorWrapper,
		"source_file":          emptyProcessor,

		// JavaScript-specific (also used by TypeScript)
		"member_expression":   memberExpressionProcessor,
		"arrow_function":      arrowFunctionProcessor,
		"method_definition":   methodDefinitionProcessor,
		"new_expression":      jsNewExpressionProcessor,
		"lexical_declaration": lexicalDeclarationProcessor,

		// TypeScript-specific: skip type-level nodes to avoid false-positive call edges
		"type_annotation":            skippedProcessor,
		"type_alias_declaration":     skippedProcessor,
		"interface_declaration":      skipResultsProcessor,
		"enum_declaration":           skipResultsProcessor,
		"abstract_class_declaration": classDefinitionProcessor,
		"abstract_method_signature":  skippedProcessor,
		"as_expression":              skippedProcessor,
		"satisfies_expression":       skippedProcessor,
		"non_null_expression":        emptyProcessor,
		"type_arguments":             skippedProcessor,
		"accessibility_modifier":     skippedProcessor,
		"override_modifier":          skippedProcessor,
		"readonly":                   skippedProcessor,
		"required_parameter":         emptyProcessor,
		"optional_parameter":         emptyProcessor,
		"predefined_type":            skippedProcessor,
		"type_identifier":            skippedProcessor,
		"interface_body":             skippedProcessor,
		"abstract_method_definition": skippedProcessor,
		"public_field_definition":    skipResultsProcessor,
		"extends_clause":             skippedProcessor,
		"implements_clause":          skippedProcessor,
		"extends_type_clause":        skippedProcessor,
		"class_heritage":             skippedProcessor,
	}

	for literalNodeType := range literalNodeTypes {
		nodeProcessors[literalNodeType] = literalValueProcessor
	}

	// Only process item initialisers (possible calls or assignments in subexpressions)
	// without propagating results
	for nodeType := range initialisedDataStructures {
		nodeProcessors[nodeType] = skipResultsProcessor
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

// skipResultsProcessor processes its children but does not propagate any results to parent node.
func skipResultsProcessor(emptyNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if emptyNode == nil {
		return newProcessorResult()
	}

	processChildren(emptyNode, treeData, currentNamespace, callGraph, metadata)

	return newProcessorResult()
}

// expressionStatementProcessorWrapper handles expression_statement nodes
// For module-level statements in JavaScript, we want to track calls
// For statements inside functions/classes, we skip result propagation
func expressionStatementProcessorWrapper(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if node == nil {
		return newProcessorResult()
	}

	// Check if we're at module level (not inside a function or class method body)
	// For JavaScript at module level, we want to track the calls
	if !metadata.insideFunction && !metadata.insideClass {
		// Module-level: process children and track calls in the module namespace
		for i := 0; i < int(node.ChildCount()); i++ {
			childNode := node.Child(i)
			if childNode == nil {
				continue
			}

			childResult := processNode(childNode, treeData, currentNamespace, callGraph, metadata)

			// Register any immediate calls from module level
			for _, callRef := range childResult.ImmediateCallRefs {
				callGraph.addCall(currentNamespace, nil, callRef)
			}
		}
	} else {
		// Inside function/class: skip results propagation (original behavior)
		processChildren(node, treeData, currentNamespace, callGraph, metadata)
	}

	return newProcessorResult()
}

// emptyProcessor processes its children and propagates results to parent node,
func emptyProcessor(emptyNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if emptyNode == nil {
		return newProcessorResult()
	}

	return processChildren(emptyNode, treeData, currentNamespace, callGraph, metadata)
}

// skippedProcessor is placeholder for nodes that need not be processed
func skippedProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	return newProcessorResult()
}

// literalValueProcessor processes literal values (like strings, numbers, etc.)
// This is different from identifiers as it does not account for namespace,
// since literal values need not be associated to any scope.
func literalValueProcessor(literalNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if literalNode == nil {
		return newProcessorResult()
	}

	result := newProcessorResult()

	// Register literal values without namespace qualifier
	result.addAssignments(callGraph.assignmentGraph.addNode(literalNode.Content(treeData), literalNode))

	return result
}

func classDefinitionProcessor(classDefNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if classDefNode == nil {
		return newProcessorResult()
	}

	classNameNode := classDefNode.ChildByFieldName("name")
	if classNameNode == nil {
		log.Debugf("Class definition without name - %s", classDefNode.Content(treeData))
		return newProcessorResult()
	}

	// Class definition has its own scope, hence its own namespace
	classNamespace := currentNamespace + namespaceSeparator + classNameNode.Content(treeData)
	callGraph.addNode(classNamespace, classDefNode)

	// Assignment is added so that we can resolve class constructor when a function with same name as classname is called
	callGraph.assignmentGraph.addNode(classNamespace, classDefNode)
	callGraph.classConstructors[classNamespace] = true

	instanceKeyword, exists := callGraph.getInstanceKeyword()
	if exists {
		instanceNamespace := classNamespace + namespaceSeparator + instanceKeyword
		callGraph.addNode(instanceNamespace, nil) // @TODO - Can't create sitter node for instance keyword
	}

	classBody := classDefNode.ChildByFieldName("body")
	if classBody == nil {
		log.Debugf("Class definition without body - %s", classDefNode.Content(treeData))
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
		log.Debugf("Function definition without name - %s", functionDefNode.Content(treeData))
		return newProcessorResult()
	}

	// Function definition has its own scope, hence its own namespace
	funcName := functionNameNode.Content(treeData)
	functionNamespace := currentNamespace + namespaceSeparator + funcName

	// Java - Register direct call from root namespace to main function
	if callGraph.languageCode == core.LanguageCodeJava && funcName == "main" {
		rootNamespace := resolveRootNamespaceQualifier(currentNamespace)
		callGraph.addEdge(
			rootNamespace, nil, nil,
			functionNamespace, functionDefNode,
			[]CallArgument{}, // we don't know values for "String[] args" passed to main method
		)
	}

	// Add function to the call graph
	if _, exists := callGraph.Nodes[functionNamespace]; !exists {
		callGraph.addNode(functionNamespace, functionDefNode)
		log.Debugf("Register function definition for %s - %s", funcName, functionNamespace)

		// Add virtual fn call from class => classConstructor
		if metadata.insideClass {
			instanceKeyword, exists := callGraph.getInstanceKeyword()
			if exists {
				instanceNamespace := currentNamespace + namespaceSeparator + instanceKeyword + namespaceSeparator + funcName
				callGraph.addEdge(
					instanceNamespace, nil, nil,
					functionNamespace, functionDefNode,
					[]CallArgument{},
				) // @TODO - Can't create sitter node for instance keyword
				log.Debugf("Register instance member function definition for %s - %s\n", funcName, instanceNamespace)
			}

			// Python - Register direct call from current namespace to class constructor
			if callGraph.languageCode == core.LanguageCodePython && funcName == "__init__" {
				callGraph.addEdge(
					currentNamespace, nil, nil,
					functionNamespace, functionDefNode,
					[]CallArgument{},
				) // @TODO - Can't create sitter node for instance keyword
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
		log.Debugf("Assignment without left node - %s", node.Content(treeData))
		return newProcessorResult()
	}

	rightNode := node.ChildByFieldName("right")
	if rightNode == nil {
		log.Debugf("Assignment without right node - %s", node.Content(treeData))
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
		fallback := callGraph.assignmentGraph.addNode(currentNamespace+namespaceSeparator+leftNode.Content(treeData), leftNode)
		if fallback == nil {
			return newProcessorResult()
		}
		assigneeNodes = []*assignmentNode{fallback}
	}

	result := processNode(rightNode, treeData, currentNamespace, callGraph, metadata)

	// Assign/Call edge from all resolutions of left part => all resolutions of right part
	for _, assigneeNode := range assigneeNodes {
		for _, immediateCall := range result.ImmediateCallRefs {
			callGraph.addCall(assigneeNode.Namespace, assigneeNode.TreeNode, immediateCall)
		}

		for _, immediateAssignment := range result.ImmediateAssignments {
			callGraph.assignmentGraph.addAssignment(
				assigneeNode.Namespace, assigneeNode.TreeNode,
				immediateAssignment.Namespace, immediateAssignment.TreeNode,
			)
		}

		// log.Debugf("Resolved assignment for '%s' => %v\n", assigneeNode.Namespace, assigneeNode.AssignedTo)
		// if callGraph.Nodes[assigneeNode.Namespace] != nil {
		// 	log.Debugf("\tGraph edges -> %v\n", callGraph.Nodes[assigneeNode.Namespace].CallsTo)
		// }
	}

	return newProcessorResult()
}

func ternaryExpressionProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if node == nil {
		return newProcessorResult()
	}

	accumulatedResult := newProcessorResult()

	// Process condition part (without propagating assignments)
	conditionNode := node.ChildByFieldName("condition")
	processNode(conditionNode, treeData, currentNamespace, callGraph, metadata)

	// Accumulate and return results from consequence/alternative
	consequenceNode := node.ChildByFieldName("consequence")
	alternativeNode := node.ChildByFieldName("alternative")

	accumulatedResult.addResults(
		processNode(consequenceNode, treeData, currentNamespace, callGraph, metadata),
		processNode(alternativeNode, treeData, currentNamespace, callGraph, metadata),
	)

	return accumulatedResult
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
		log.Debugf("Object not found in namespace for attribute - %s (Obj - %s, Attr - %s)", attributeNode.Content(treeData), objectSymbol, attributeQualifierNamespace)
		return newProcessorResult()
	}

	resolvedObjects := callGraph.assignmentGraph.resolve(targetObject.Namespace)

	// log.Debugf("Resolved attribute for `%s` => %v // %s\n", node.Content(treeData), resolvedObjectNamespaces, attributeQualifierNamespace)

	// We only handle assignments for attributes here eg. xyz.attr
	// 'called' attributes eg. xyz.attr(), are handled in callProcessor directly
	result := newProcessorResult()
	for _, resolvedObject := range resolvedObjects {
		finalAttributeNamespace := resolvedObject.Namespace + namespaceSeparator + attributeQualifierNamespace
		result.addAssignments(callGraph.assignmentGraph.addNode(finalAttributeNamespace, attributeNode))
	}

	return result
}

func binaryOperatorProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if node == nil {
		return newProcessorResult()
	}

	leftNode := node.ChildByFieldName("left")
	if leftNode == nil {
		log.Debugf("Binary operator without left node - %s", node.Content(treeData))
		return newProcessorResult()
	}
	rightNode := node.ChildByFieldName("right")
	if rightNode == nil {
		log.Debugf("Binary operator without right node - %s", node.Content(treeData))
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

	// If not found by search, we can assume it is a new identifier
	result.addAssignments(callGraph.assignmentGraph.addNode(currentNamespace+namespaceSeparator+identifierNode.Content(treeData), identifierNode))

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

// resolveCallArguments processes the arguments of a function call as usual
// but records the immediate assignments of each argument separately ensuring
// positional information is not lost. This is required since some arguments
// may not be possible to be resolved to single value or any value at all,
// this would ensure at least an empty CallArgument is recorded.
func resolveCallArguments(argumentsListNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) []CallArgument {
	if argumentsListNode == nil {
		return []CallArgument{}
	}

	if argumentsListNode.Type() != "argument_list" && argumentsListNode.Type() != "arguments" {
		log.Debugf("Expected argument_list or arguments node, got %s for %s", argumentsListNode.Type(), argumentsListNode.Content(treeData))
		return []CallArgument{}
	}

	result := make([]CallArgument, argumentsListNode.NamedChildCount())

	for i := 0; uint32(i) < argumentsListNode.NamedChildCount(); i++ {
		childNode := argumentsListNode.NamedChild(i)
		if childNode == nil {
			continue
		}

		childProcessorResult := processNode(childNode, treeData, currentNamespace, callGraph, metadata)

		// Register ImmediateCallRefs (from holding current namespace)
		// eg. if we're processing args for "foo(a, b, bar(x))" in main function
		// this will register call from "main" to "bar"
		for _, callRef := range childProcessorResult.ImmediateCallRefs {
			callGraph.addCall(currentNamespace, nil, callRef)
		}

		resolvedTerminalAssignmentNodes := []*assignmentNode{}
		for _, assignmentNode := range childProcessorResult.ImmediateAssignments {
			// Resolve the assignment node to its terminal nodes
			resolvedNodes := callGraph.assignmentGraph.resolve(assignmentNode.Namespace)
			resolvedTerminalAssignmentNodes = append(resolvedTerminalAssignmentNodes, resolvedNodes...)
		}

		result[i] = CallArgument{
			Nodes: resolvedTerminalAssignmentNodes,
		}
	}

	return result
}

func functionCallProcessor(functionCallNode *sitter.Node, argumentsNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	result := newProcessorResult()

	functionName := functionCallNode.Content(treeData)

	markClassAssignment := func(classAssignmentNode *assignmentNode) {
		if classAssignmentNode != nil && callGraph.classConstructors[classAssignmentNode.Namespace] {
			// Include class namespace in assignments for constructors
			result.ImmediateAssignments = append(result.ImmediateAssignments, classAssignmentNode)
			log.Debugf("Class constructed - %s in fncall for %s\n", classAssignmentNode.Namespace, functionName)
		}
	}

	callArguments := []CallArgument{}

	// Process function arguments
	if argumentsNode != nil {
		callArguments = resolveCallArguments(argumentsNode, treeData, currentNamespace, callGraph, metadata)
	}

	addEdge := func(callee string, calleeTreeNode *sitter.Node) {
		callGraph.addEdge(currentNamespace, nil, functionCallNode, callee, calleeTreeNode, callArguments)
		if callGraph.isPythonDynamicImport(callee) && !staticString(firstArgument(argumentsNode)) {
			callGraph.addUnresolved(currentNamespace, functionCallNode.Parent(), treeData, UnresolvedDynamicImport)
		}
	}

	functionAssignmentNode, functionResolvedBySearch := searchSymbolInScopeChain(functionName, currentNamespace, callGraph)
	if functionResolvedBySearch {
		log.Debugf("Call %s searched (direct) & resolved to %s\n", functionName, functionAssignmentNode.Namespace)
		addEdge(functionAssignmentNode.Namespace, functionAssignmentNode.TreeNode)
		markClassAssignment(functionAssignmentNode)
		return result
	}

	switch functionCallNode.Type() {
	case "identifier":
		callGraph.addCall(currentNamespace, nil, CallReference{
			CalleeNamespace:  functionName,
			CallerIdentifier: functionCallNode,
			Arguments:        callArguments,
			Unbound:          true,
		})
		return result
	case "subscript":
		callGraph.addUnresolved(currentNamespace, functionCallNode, treeData, UnresolvedComputedName)
		return result
	case "call":
		// The inner call is a call of its own, as getattr in getattr(o, n)()
		processNode(functionCallNode, treeData, currentNamespace, callGraph, metadata)
		if inner := functionCallNode.ChildByFieldName("function"); inner != nil && inner.Content(treeData) == "getattr" {
			callGraph.addUnresolved(currentNamespace, functionCallNode, treeData, UnresolvedComputedName)
		}
		return result
	}

	// Process attributes
	functionObjectNode := functionCallNode.ChildByFieldName("object")
	functionAttributeNode := functionCallNode.ChildByFieldName("attribute")

	// A chained call, as in PasswordHasher().hash() or
	// OpenAI().chat.completions.create(), calls the inner function too, and
	// the inner function is the base of the name
	root := chainRoot(functionObjectNode, "attribute", "object")
	if functionAttributeNode != nil && root != nil && root.Type() == "call" {
		processNode(root, treeData, currentNamespace, callGraph, metadata)
		if callee, ok := pythonCalleeOf(functionCallNode, treeData, currentNamespace, callGraph, metadata, 0); ok {
			addEdge(callee, nil)
		}
		return result
	}

	if functionAttributeNode != nil && functionObjectNode != nil {
		log.Debugf("Call %s searched (attr qualified) & resolved to object - %s (%s), attribute - %s (%s) \n", functionName, functionObjectNode.Content(treeData), functionObjectNode.Type(), functionAttributeNode.Content(treeData), functionAttributeNode.Type())

		objectSymbol, attributeQualifierNamespace, err := dissectAttributeQualifier(functionObjectNode, treeData, currentNamespace, callGraph, metadata)
		if err != nil {
			if root != nil && root.Type() == "subscript" {
				callGraph.addUnresolved(currentNamespace, functionCallNode, treeData, UnresolvedComputedName)
			}
			return newProcessorResult()
		}

		finalAttributeNamespace := functionAttributeNode.Content(treeData)
		if attributeQualifierNamespace != "" {
			finalAttributeNamespace = attributeQualifierNamespace + namespaceSeparator + finalAttributeNamespace
		}

		objectAssignmentNode, functionResolvedByObjectQualifiedSearch := searchSymbolInScopeChain(objectSymbol, currentNamespace, callGraph)
		if !functionResolvedByObjectQualifiedSearch {
			callGraph.addUnresolved(currentNamespace, functionCallNode, treeData, UnresolvedUnknownReceiver)
			return result
		}

		for _, resolvedObjectNode := range callGraph.receiverTargets(objectAssignmentNode.Namespace, currentNamespace, functionCallNode) {
			functionNamespace := resolvedObjectNode.Namespace + namespaceSeparator + finalAttributeNamespace

			// The definition of the function is out of reach here
			addEdge(functionNamespace, nil)

			markClassAssignment(callGraph.assignmentGraph.Assignments[functionNamespace])
		}
	}

	return result
}

// pythonDynamicImports are the functions that import a module by a name
// that the call computes.
var pythonDynamicImports = map[string]bool{
	"importlib//import_module": true,
	"__import__":               true,
}

// isPythonDynamicImport reports that namespace, or a value of it, is a
// function that imports a module by name.
func (cg *CallGraph) isPythonDynamicImport(namespace string) bool {
	if cg.languageCode != core.LanguageCodePython {
		return false
	}
	if pythonDynamicImports[namespace] {
		return true
	}
	return slices.ContainsFunc(cg.assignmentGraph.resolve(namespace), func(target *assignmentNode) bool {
		return pythonDynamicImports[target.Namespace]
	})
}

// pythonCalleeOf returns the namespace of the function of a Python call,
// through the scope chain and the assignment graph. It adds no edge.
func pythonCalleeOf(function *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata, depth int) (string, bool) {
	if function == nil {
		return "", false
	}
	if depth > maxAttributeDepth {
		callGraph.depthLimit(TruncationAttributeDepth, currentNamespace, function, treeData)
		return "", false
	}

	terminal := func(symbol string) (string, bool) {
		assignment, found := searchSymbolInScopeChain(symbol, currentNamespace, callGraph)
		if !found {
			return "", false
		}
		if targets := callGraph.assignmentGraph.resolve(assignment.Namespace); len(targets) > 0 {
			return targets[0].Namespace, true
		}
		return assignment.Namespace, true
	}

	switch function.Type() {
	case "identifier":
		return terminal(function.Content(treeData))
	case "attribute":
		object := function.ChildByFieldName("object")
		attribute := function.ChildByFieldName("attribute")
		if object == nil || attribute == nil {
			return "", false
		}
		var base string
		var ok bool
		switch root := chainRoot(object, "attribute", "object"); {
		case object.Type() == "call":
			base, ok = pythonCalleeOf(object.ChildByFieldName("function"), treeData, currentNamespace, callGraph, metadata, depth+1)
		case root != nil && root.Type() == "call":
			base, ok = pythonCalleeOf(object, treeData, currentNamespace, callGraph, metadata, depth+1)
		}
		if ok {
			return base + namespaceSeparator + attribute.Content(treeData), true
		}
		if object.Type() == "call" {
			return "", false
		}
		symbol, qualifier, err := dissectAttributeQualifier(object, treeData, currentNamespace, callGraph, metadata)
		if err != nil {
			return "", false
		}
		base, ok = terminal(symbol)
		if !ok {
			return "", false
		}
		if qualifier != "" {
			base += namespaceSeparator + qualifier
		}
		return base + namespaceSeparator + attribute.Content(treeData), true
	}
	return "", false
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
	return dissectAttributeQualifierWithDepth(node, treeData, currentNamespace, callGraph, metadata, 0)
}

// dissectAttributeQualifierWithDepth is the internal implementation with depth tracking
func dissectAttributeQualifierWithDepth(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata, depth int) (string, string, error) {
	if node == nil {
		return "", "", fmt.Errorf("fnAttributeResolver - node is nil")
	}

	if depth > maxAttributeDepth {
		callGraph.depthLimit(TruncationAttributeDepth, currentNamespace, node, treeData)
		return "", "", fmt.Errorf("attribute deeper than %d levels", maxAttributeDepth)
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

	objectSymbol, objectSubAttributeNamespace, err := dissectAttributeQualifierWithDepth(objectNode, treeData, currentNamespace, callGraph, metadata, depth+1)
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
		log.Debugf("error resolving scoped identifier %s - %v", scopedIdentifierNode.Content(treeData), err)
	}

	targetObject, objectResolved := searchSymbolInScopeChain(targetObjectIdentifier, currentNamespace, callGraph)
	if objectResolved {
		resolvedObjects := callGraph.assignmentGraph.resolve(targetObject.Namespace)

		result := newProcessorResult()
		for _, resolvedObject := range resolvedObjects {
			finalIdentifierNamespace := resolvedObject.Namespace + namespaceSeparator + objectQualifierNamespace
			result.addAssignments(callGraph.assignmentGraph.addNode(finalIdentifierNamespace, scopedIdentifierNode))
		}
		return result
	}

	// Consider this as fully qualified usage of a type without importing it
	// eg. directly using - "java.awt.event.MouseEvent"
	importNamespace := targetObjectIdentifier + namespaceSeparator + objectQualifierNamespace
	log.Debugf("Scoped identifier %s - fallback to fully qualified import - %s", scopedIdentifierNode.Content(treeData), importNamespace)

	result := newProcessorResult()
	result.addAssignments(callGraph.assignmentGraph.addNode(importNamespace, scopedIdentifierNode))

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

	argsResult := []CallArgument{}

	argumentsNode := objectCreationNode.ChildByFieldName("arguments")
	if argumentsNode != nil {
		argsResult = resolveCallArguments(argumentsNode, treeData, currentNamespace, callGraph, metadata)
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

			callGraph.addEdge(
				currentNamespace, nil, objectCreationNode,
				classNode.Namespace, classNode.TreeNode,
				argsResult,
			) // Assumption - current namespace exists in the graph

			markClassAssignment(classNode)
		} else if constructedClassNode.Type() == "scoped_type_identifier" {
			// Try resolving scoped identifiers
			scopedIdentifierResult := scopedIdentifierProcessor(constructedClassNode, treeData, currentNamespace, callGraph, metadata)

			for _, scopedIdentifierAssignment := range scopedIdentifierResult.ImmediateAssignments {
				// Register class constructor as Function call in callgraph
				callGraph.addEdge(
					currentNamespace, nil, objectCreationNode,
					scopedIdentifierAssignment.Namespace, nil,
					argsResult,
				) // Assumption - current namespace exists in the graph

				// Mark assignments for parent
				markClassAssignment(scopedIdentifierAssignment)
			}
		}
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
			callGraph.assignmentGraph.addAssignment(
				declaredVariableNamespace, declaredVariableNode,
				immediateAssignment.Namespace, immediateAssignment.TreeNode,
			)
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

// Reused Utility wrapper over resolveCallArguments to process method invocation arguments
// Specific to Java method_invocation
func processMethodArgs(methodInvocationNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) []CallArgument {
	if methodInvocationNode == nil || methodInvocationNode.Type() != "method_invocation" {
		log.Warnf("Incorrect method invocation node processed for args - %s", methodInvocationNode.Content(treeData))
		return []CallArgument{}
	}

	argumentsNode := methodInvocationNode.ChildByFieldName("arguments")
	if argumentsNode == nil {
		return []CallArgument{}
	}

	// Process function arguments
	// @TODO - Ideally, the result.ImmediateAssignments should be associated with called function
	// but, we don't have parameter and their positional information, which is a complex task
	// Hence, we're not processing argument results here
	return resolveCallArguments(argumentsNode, treeData, currentNamespace, callGraph, metadata)
}

func methodInvocationProcessor(methodInvocationNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if methodInvocationNode == nil {
		return newProcessorResult()
	}

	argsResult := processMethodArgs(methodInvocationNode, treeData, currentNamespace, callGraph, metadata)

	methodNameNode := methodInvocationNode.ChildByFieldName("name")
	if methodNameNode == nil {
		log.Debugf("Method invocation without name - %s", methodInvocationNode.Content(treeData))
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
			processMethodArgs(methodQualifierObjectNode, treeData, currentNamespace, callGraph, metadata)

			hasChainedMethodInvocations = true
			nextObjNode := methodQualifierObjectNode.ChildByFieldName("object")
			if nextObjNode == nil {
				break
			}

			if nextObjNode.Type() != "method_invocation" {
				break
			}

			processMethodArgs(nextObjNode, treeData, currentNamespace, callGraph, metadata)

			methodQualifierObjectNode = nextObjNode
		}

		if creation, calledMethod := createdReceiver(methodQualifierObjectNode, methodName, hasChainedMethodInvocations, treeData); creation != nil {
			created := objectCreationExpressionProcessor(creation, treeData, currentNamespace, callGraph, metadata)
			var classes []string
			for _, class := range created.ImmediateAssignments {
				for _, target := range callGraph.assignmentGraph.resolve(class.Namespace) {
					classes = append(classes, target.Namespace)
				}
			}
			if typeNode := creation.ChildByFieldName("type"); len(classes) == 0 && typeNode != nil {
				// A class with no import, as one of the same package or of
				// java.lang, keeps its name.
				classes = append(classes, strings.ReplaceAll(withoutTypeArguments(typeNode.Content(treeData)), ".", namespaceSeparator))
			}
			for _, class := range classes {
				callGraph.addEdge(
					currentNamespace, nil, methodInvocationNode,
					class+namespaceSeparator+calledMethod, methodInvocationNode,
					argsResult,
				)
			}
			return newProcessorResult()
		}

		methodObjectQualifierNamespace := resolveQualifierObjectFieldaccess(methodQualifierObjectNode, treeData)
		qualifiers := strings.Split(methodObjectQualifierNamespace, namespaceSeparator)
		callerObjectNamespaces := []string{}

		if len(qualifiers) > 0 {
			rootObjKeyword := qualifiers[0]
			rootObjNode, rootObjNodeExists := searchSymbolInScopeChain(rootObjKeyword, currentNamespace, callGraph)

			var rootCallerObjAssignments []*assignmentNode
			if rootObjNodeExists {
				rootCallerObjAssignments = callGraph.assignmentGraph.resolve(rootObjNode.Namespace)
			} else {
				// If root object is not found, we can assume it is a fully qualified object
				// eg. sun.reflect.Method here, sun couldn't be identified, so assume its a library root keyword
				callGraph.addNode(rootObjKeyword, nil) // @TODO - Can't create sitter node for fully qualified object
				rootCallerObjAssignments = callGraph.assignmentGraph.resolve(rootObjKeyword)
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
			callGraph.addEdge(
				currentNamespace, nil, methodInvocationNode,
				calledNamespace, methodInvocationNode,
				argsResult,
			) // Assumption - current namespace exists in the graph
		}

		return newProcessorResult()
	}

	// Simple function call lookup without any qualifiers
	functionAssignmentNode, functionResolvedBySearch := searchSymbolInScopeChain(methodName, currentNamespace, callGraph)
	if functionResolvedBySearch {
		log.Debugf("Method invocation %s searched (direct) & resolved to %s\n", methodName, functionAssignmentNode.Namespace)
		callGraph.addEdge(
			currentNamespace, nil, methodInvocationNode,
			functionAssignmentNode.Namespace, functionAssignmentNode.TreeNode,
			argsResult,
		) // Assumption - current namespace exists in the graph
		return newProcessorResult()
	}

	log.Debugf("Method invocation %s couldn't be processed", methodName)

	return newProcessorResult()
}

// createdReceiver returns the object creation that receives the first call
// of a chain, and the name of that call. In new Builder("k").build(), the
// call is Builder//build. A chain with no object creation at its root
// returns nil.
func createdReceiver(qualifier *sitter.Node, methodName string, chained bool, treeData []byte) (*sitter.Node, string) {
	if chained && qualifier.Type() == "method_invocation" {
		name := qualifier.ChildByFieldName("name")
		if name == nil {
			return nil, ""
		}
		qualifier, methodName = qualifier.ChildByFieldName("object"), name.Content(treeData)
	}
	if qualifier == nil || qualifier.Type() != "object_creation_expression" {
		return nil, ""
	}
	return qualifier, methodName
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

// Go-specific ------

// Wrapper functions to check language before processing language-specific nodes

func callExpressionProcessorWrapper(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	switch callGraph.languageCode {
	case core.LanguageCodeGo:
		return goCallExpressionProcessor(node, treeData, currentNamespace, callGraph, metadata)
	case core.LanguageCodeJavascript, core.LanguageCodeTypescript:
		return jsCallExpressionProcessor(node, treeData, currentNamespace, callGraph, metadata)
	default:
		return newProcessorResult()
	}
}

func functionDeclarationProcessorWrapper(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	switch callGraph.languageCode {
	case core.LanguageCodeGo:
		return goFunctionDeclarationProcessor(node, treeData, currentNamespace, callGraph, metadata)
	case core.LanguageCodeJavascript, core.LanguageCodeTypescript:
		return jsFunctionDeclarationProcessor(node, treeData, currentNamespace, callGraph, metadata)
	default:
		// Fallback to default function definition processor for other languages
		return functionDefinitionProcessor(node, treeData, currentNamespace, callGraph, metadata)
	}
}

func goMethodDeclarationProcessorWrapper(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if callGraph.languageCode == core.LanguageCodeGo {
		return goMethodDeclarationProcessor(node, treeData, currentNamespace, callGraph, metadata)
	}

	// Fallback to default function definition processor for other languages (Java)
	return functionDefinitionProcessor(node, treeData, currentNamespace, callGraph, metadata)
}

// goCallExpressionProcessor handles Go call_expression nodes
// Examples:
// - os.WriteFile(filename, data, 0644) -> os//WriteFile
// - fmt.Println("hello") -> fmt//Println
// - helper(10) -> helper (unqualified function call)
func goCallExpressionProcessor(callNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if callNode == nil {
		return newProcessorResult()
	}

	result := newProcessorResult()

	// Get function node
	functionNode := callNode.ChildByFieldName("function")
	if functionNode == nil {
		return result
	}

	// Get arguments node
	argumentsNode := callNode.ChildByFieldName("arguments")
	callArguments := []CallArgument{}
	if argumentsNode != nil {
		callArguments = resolveGoCallArguments(argumentsNode, treeData, currentNamespace, callGraph, metadata)
	}

	// A chained call, as in ecdh.X25519().GenerateKey() or
	// openai.NewClient(k).Chat.Completions.New(), calls the inner function too
	if root := chainRoot(functionNode, "selector_expression", "operand"); root != nil && root.Type() == "call_expression" {
		processNode(root, treeData, currentNamespace, callGraph, metadata)
	}

	var callees []string
	unbound := false

	switch functionNode.Type() {
	case "selector_expression":
		// Package-qualified or method call: pkg.Func or obj.Method
		callees = resolveGoSelectorExpression(functionNode, treeData, currentNamespace, callGraph)
	case "identifier":
		callee, bound := resolveGoIdentifier(functionNode.Content(treeData), currentNamespace, callGraph)
		callees, unbound = []string{callee}, !bound
	default:
		// Other types (e.g., function literals, etc.) - try as identifier
		callees = []string{functionNode.Content(treeData)}
	}

	for _, callee := range callees {
		callGraph.addCall(currentNamespace, nil, CallReference{
			CalleeNamespace:  callee,
			CallerIdentifier: functionNode,
			Arguments:        callArguments,
			Unbound:          unbound,
		})
		log.Debugf("Go call: %s -> %s", currentNamespace, callee)
	}

	return result
}

// resolveGoCallArguments processes arguments for Go function calls
func resolveGoCallArguments(argumentsNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) []CallArgument {
	if argumentsNode == nil {
		return []CallArgument{}
	}

	if argumentsNode.Type() != "argument_list" {
		log.Debugf("Expected argument_list node, got %s for %s", argumentsNode.Type(), argumentsNode.Content(treeData))
		return []CallArgument{}
	}

	result := make([]CallArgument, 0, argumentsNode.NamedChildCount())

	for i := 0; uint32(i) < argumentsNode.NamedChildCount(); i++ {
		childNode := argumentsNode.NamedChild(i)
		if childNode == nil {
			continue
		}

		childProcessorResult := processNode(childNode, treeData, currentNamespace, callGraph, metadata)

		// Register ImmediateCallRefs from arguments
		for _, callRef := range childProcessorResult.ImmediateCallRefs {
			callGraph.addCall(currentNamespace, nil, callRef)
		}

		resolvedTerminalAssignmentNodes := []*assignmentNode{}
		for _, assignmentNode := range childProcessorResult.ImmediateAssignments {
			resolvedNodes := callGraph.assignmentGraph.resolve(assignmentNode.Namespace)
			resolvedTerminalAssignmentNodes = append(resolvedTerminalAssignmentNodes, resolvedNodes...)
		}

		result = append(result, CallArgument{
			Nodes: resolvedTerminalAssignmentNodes,
		})
	}

	return result
}

// resolveGoSelectorExpression resolves Go selector expressions like pkg.Func
// or obj.Method. A variable with more than one value gives one name for
// each value.
func resolveGoSelectorExpression(selectorNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph) []string {
	if selectorNode == nil || selectorNode.Type() != "selector_expression" {
		return nil
	}

	// Get operand (left side) and field (right side)
	operandNode := selectorNode.ChildByFieldName("operand")
	fieldNode := selectorNode.ChildByFieldName("field")

	if operandNode == nil || fieldNode == nil {
		return nil
	}

	operandName := operandNode.Content(treeData)
	fieldName := fieldNode.Content(treeData)

	// A call or a selector as the operand is the base of the name, as in
	// ecdh.X25519().GenerateKey or client.Chat.Completions.New
	switch operandNode.Type() {
	case "call_expression":
		if bases := goCalleeOf(operandNode, treeData, currentNamespace, callGraph); len(bases) > 0 {
			return qualify(bases, fieldName)
		}
	case "selector_expression":
		if bases := resolveGoSelectorExpression(operandNode, treeData, currentNamespace, callGraph); len(bases) > 0 {
			return qualify(bases, fieldName)
		}
	}

	// An imported package or a variable
	if operandAssignment, operandResolved := searchSymbolInScopeChain(operandName, currentNamespace, callGraph); operandResolved {
		if targets := callGraph.receiverTargets(operandAssignment.Namespace, currentNamespace, selectorNode); len(targets) > 0 {
			return qualify(namespacesOf(targets), fieldName)
		}
	}

	// The scope chain does not know the operand, as a parameter or a local
	// variable. The name keeps the operand as written.
	callGraph.addUnresolved(currentNamespace, selectorNode, treeData, UnresolvedUnknownReceiver)
	return []string{operandName + namespaceSeparator + fieldName}
}

// goCalleeOf returns the namespaces of the function of a Go call. It adds no
// edge.
func goCalleeOf(callNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph) []string {
	function := callNode.ChildByFieldName("function")
	if function == nil {
		return nil
	}
	switch function.Type() {
	case "selector_expression":
		return resolveGoSelectorExpression(function, treeData, currentNamespace, callGraph)
	case "identifier":
		callee, _ := resolveGoIdentifier(function.Content(treeData), currentNamespace, callGraph)
		return []string{callee}
	}
	return nil
}

// resolveGoIdentifier resolves an unqualified Go identifier. It reports
// false for a name that the scope chain does not bind, and returns the name
// in the current namespace.
func resolveGoIdentifier(identifier string, currentNamespace string, callGraph *CallGraph) (string, bool) {
	if assignmentNode, found := searchSymbolInScopeChain(identifier, currentNamespace, callGraph); found {
		return assignmentNode.Namespace, true
	}
	return currentNamespace + namespaceSeparator + identifier, false
}

// goFunctionDeclarationProcessor handles Go function and method declarations
func goFunctionDeclarationProcessor(funcDefNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if funcDefNode == nil {
		return newProcessorResult()
	}

	functionNameNode := funcDefNode.ChildByFieldName("name")
	if functionNameNode == nil {
		log.Debugf("Go function declaration without name - %s", funcDefNode.Content(treeData))
		return newProcessorResult()
	}

	funcName := functionNameNode.Content(treeData)
	functionNamespace := currentNamespace + namespaceSeparator + funcName

	// Go - Register package-level functions as callable from root namespace
	// This includes main() and all other top-level functions (library code, tests, etc.)
	// For library packages without main(), this makes exported functions discoverable
	callGraph.addEdge(
		currentNamespace, nil, nil,
		functionNamespace, funcDefNode,
		[]CallArgument{},
	)

	// Add function to call graph
	if _, exists := callGraph.Nodes[functionNamespace]; !exists {
		callGraph.addNode(functionNamespace, funcDefNode)
		log.Debugf("Register Go function definition for %s - %s", funcName, functionNamespace)
	}

	results := newProcessorResult()

	// Process function body
	functionBody := funcDefNode.ChildByFieldName("body")
	if functionBody != nil {
		metadata.insideFunction = true
		result := processChildren(functionBody, treeData, functionNamespace, callGraph, metadata)
		metadata.insideFunction = false
		results.addResults(result)
	}

	return results
}

// goMethodDeclarationProcessor handles Go method declarations (functions with receivers)
func goMethodDeclarationProcessor(methodDefNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if methodDefNode == nil {
		return newProcessorResult()
	}

	methodNameNode := methodDefNode.ChildByFieldName("name")
	if methodNameNode == nil {
		log.Debugf("Go method declaration without name - %s", methodDefNode.Content(treeData))
		return newProcessorResult()
	}

	// Extract receiver type
	receiverNode := methodDefNode.ChildByFieldName("receiver")
	receiverType := extractGoReceiverType(receiverNode, treeData)

	methodName := methodNameNode.Content(treeData)

	// Method namespace includes receiver type
	var methodNamespace string
	if receiverType != "" {
		// Namespace: file//ReceiverType//MethodName
		methodNamespace = currentNamespace + namespaceSeparator + receiverType + namespaceSeparator + methodName
	} else {
		// Fallback if receiver type can't be extracted
		methodNamespace = currentNamespace + namespaceSeparator + methodName
	}

	// Add method to call graph
	if _, exists := callGraph.Nodes[methodNamespace]; !exists {
		callGraph.addNode(methodNamespace, methodDefNode)
		log.Debugf("Register Go method definition for %s on %s - %s", methodName, receiverType, methodNamespace)
	}

	results := newProcessorResult()

	// Process method body
	methodBody := methodDefNode.ChildByFieldName("body")
	if methodBody != nil {
		metadata.insideFunction = true
		result := processChildren(methodBody, treeData, methodNamespace, callGraph, metadata)
		metadata.insideFunction = false
		results.addResults(result)
	}

	return results
}

// extractGoReceiverType extracts the receiver type name from a receiver parameter list
// Example: (m *MyType) -> MyType
func extractGoReceiverType(receiverNode *sitter.Node, treeData []byte) string {
	if receiverNode == nil {
		return ""
	}

	// Receiver is a parameter_list containing parameter_declaration
	for i := 0; uint32(i) < receiverNode.ChildCount(); i++ {
		child := receiverNode.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "parameter_declaration" {
			// Look for type_identifier or pointer_type
			for j := 0; uint32(j) < child.ChildCount(); j++ {
				typeNode := child.Child(j)
				if typeNode == nil {
					continue
				}

				switch typeNode.Type() {
				case "type_identifier":
					return typeNode.Content(treeData)
				case "pointer_type":
					// Extract the underlying type from pointer
					for k := 0; uint32(k) < typeNode.ChildCount(); k++ {
						pointerChild := typeNode.Child(k)
						if pointerChild != nil && pointerChild.Type() == "type_identifier" {
							return pointerChild.Content(treeData)
						}
					}
				}
			}
		}
	}

	return ""
}

// JavaScript-specific ------

// lexicalDeclarationProcessor handles JavaScript lexical_declaration nodes (const, let, var)
// Routes to variableDeclaratorProcessor which handles the actual assignments
func lexicalDeclarationProcessor(declarationNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if declarationNode == nil {
		return newProcessorResult()
	}

	// Process all variable_declarator children
	for i := 0; uint32(i) < declarationNode.NamedChildCount(); i++ {
		declaratorNode := declarationNode.NamedChild(i)
		if declaratorNode != nil && declaratorNode.Type() == "variable_declarator" {
			processNode(declaratorNode, treeData, currentNamespace, callGraph, metadata)
		}
	}

	return newProcessorResult()
}

// jsFunctionDeclarationProcessor handles JavaScript function declarations
// Similar to Go, we register top-level functions as callable from the module namespace
func jsFunctionDeclarationProcessor(funcDefNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if funcDefNode == nil {
		return newProcessorResult()
	}

	functionNameNode := funcDefNode.ChildByFieldName("name")
	if functionNameNode == nil {
		log.Debugf("JS function declaration without name - %s", funcDefNode.Content(treeData))
		return newProcessorResult()
	}

	funcName := functionNameNode.Content(treeData)
	functionNamespace := currentNamespace + namespaceSeparator + funcName

	// Add function to call graph
	if _, exists := callGraph.Nodes[functionNamespace]; !exists {
		callGraph.addNode(functionNamespace, funcDefNode)
		log.Debugf("Register JS function declaration for %s - %s", funcName, functionNamespace)
	}

	results := newProcessorResult()

	// Process function body
	functionBody := funcDefNode.ChildByFieldName("body")
	if functionBody != nil {
		metadata.insideFunction = true
		result := processChildren(functionBody, treeData, functionNamespace, callGraph, metadata)
		metadata.insideFunction = false
		results.addResults(result)
	}

	return results
}

// jsCallExpressionProcessor handles JavaScript call_expression nodes
// Examples:
// - console.log("hello") -> console//log
// - myFunc(10) -> myFunc (unqualified function call)
// - obj.method() -> obj//method (method call)
func jsCallExpressionProcessor(callNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if callNode == nil {
		return newProcessorResult()
	}

	result := newProcessorResult()

	// Get function node
	functionNode := callNode.ChildByFieldName("function")
	if functionNode == nil {
		return result
	}

	// Get arguments node
	argumentsNode := callNode.ChildByFieldName("arguments")
	callArguments := []CallArgument{}
	if argumentsNode != nil {
		callArguments = resolveCallArguments(argumentsNode, treeData, currentNamespace, callGraph, metadata)
	}

	// A chained call, as in crypto.createHash('md5').digest(), calls the
	// inner function too
	if functionNode.Type() == "member_expression" {
		if object := chainRoot(functionNode, "member_expression", "object"); object != nil && jsChainable[object.Type()] {
			processNode(object, treeData, currentNamespace, callGraph, metadata)
		}
	}

	var callees []string
	unbound := false

	switch functionNode.Type() {
	case "member_expression":
		// Method call: obj.method() or pkg.func()
		callees = resolveJSMemberExpression(functionNode, treeData, currentNamespace, callGraph)
	case "identifier":
		callee, bound := resolveJSIdentifier(functionNode.Content(treeData), currentNamespace, callGraph)
		callees, unbound = []string{callee}, !bound
		if functionNode.Content(treeData) == "require" && !staticString(firstArgument(argumentsNode)) {
			callGraph.addUnresolved(currentNamespace, callNode, treeData, UnresolvedDynamicImport)
		}
	case "import":
		if module, ok := jsDynamicImport(argumentsNode, treeData); ok {
			callees = []string{module}
		} else {
			callGraph.addUnresolved(currentNamespace, callNode, treeData, UnresolvedDynamicImport)
		}
	case "subscript_expression":
		callGraph.addUnresolved(currentNamespace, functionNode, treeData, UnresolvedComputedName)
	default:
		// Other types (e.g., function expressions) - try as identifier
		callees = []string{functionNode.Content(treeData)}
	}

	for _, callee := range callees {
		callGraph.addCall(currentNamespace, nil, CallReference{
			CalleeNamespace:  callee,
			CallerIdentifier: functionNode,
			Arguments:        callArguments,
			Unbound:          unbound,
		})
		log.Debugf("JS call: %s -> %s", currentNamespace, callee)
	}

	return result
}

// jsDynamicImport returns the module of a dynamic import, as in
// await import("x"), in the form of a Ruby require or a C# using: x//*.
// The import loads the module, but the names it binds are often out of
// reach, as in import("x").then(({ pipeline }) => pipeline()).
func jsDynamicImport(argumentsNode *sitter.Node, treeData []byte) (string, bool) {
	module := firstArgument(argumentsNode)
	if !staticString(module) {
		return "", false
	}
	name := strings.Trim(module.Content(treeData), "'\"`")
	if name == "" {
		return "", false
	}
	return strings.Join(splitQualifiedName(name, core.LanguageCodeJavascript), namespaceSeparator) + namespaceSeparator + "*", true
}

// resolveJSMemberExpression resolves JavaScript member expressions like
// obj.method or pkg.func. A variable with more than one value gives one name
// for each value. It returns nil for a member that it cannot resolve, and
// records why.
func resolveJSMemberExpression(memberNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph) []string {
	return resolveJSMemberExpressionWithDepth(memberNode, treeData, currentNamespace, callGraph, 0)
}

func resolveJSMemberExpressionWithDepth(memberNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, depth int) []string {
	if memberNode == nil || memberNode.Type() != "member_expression" {
		return nil
	}

	if depth > maxMemberExpressionDepth {
		callGraph.depthLimit(TruncationMemberDepth, currentNamespace, memberNode, treeData)
		return nil
	}

	// Get object (left side) and property (right side)
	objectNode := memberNode.ChildByFieldName("object")
	propertyNode := memberNode.ChildByFieldName("property")

	if objectNode == nil || propertyNode == nil {
		return nil
	}

	propertyName := propertyNode.Content(treeData)

	// The value of a call or of an object creation is the base of the
	// member, as in crypto.createHash('md5').digest or new OpenAI().chat
	if jsChainable[objectNode.Type()] {
		return qualify(jsValueOf(objectNode, treeData, currentNamespace, callGraph, depth+1), propertyName)
	}

	if objectNode.Type() == "member_expression" {
		return qualify(resolveJSMemberExpressionWithDepth(objectNode, treeData, currentNamespace, callGraph, depth+1), propertyName)
	}

	objectName := objectNode.Content(treeData)

	// An imported module or a variable
	if objectAssignment, objectResolved := searchSymbolInScopeChain(objectName, currentNamespace, callGraph); objectResolved {
		if targets := callGraph.receiverTargets(objectAssignment.Namespace, currentNamespace, memberNode); len(targets) > 0 {
			return qualify(namespacesOf(targets), propertyName)
		}
	}

	// The scope chain does not know the object, as a parameter or a global
	// of the runtime. The name keeps the object as written.
	callGraph.addUnresolved(currentNamespace, memberNode, treeData, UnresolvedUnknownReceiver)
	return []string{objectName + namespaceSeparator + propertyName}
}

// chainRoot follows the object field of a chain of member nodes to the
// first node that is not a member, as client for client.chat.completions or
// the call OpenAI() for OpenAI().chat.completions. A node that is not a
// member is its own root.
func chainRoot(node *sitter.Node, memberType, objectField string) *sitter.Node {
	for depth := 0; node != nil && node.Type() == memberType && depth <= maxMemberExpressionDepth; depth++ {
		node = node.ChildByFieldName(objectField)
	}
	return node
}

// jsChainable are the JavaScript nodes whose value can start a member chain.
var jsChainable = map[string]bool{
	"call_expression":  true,
	"new_expression":   true,
	"await_expression": true,
}

// jsValueOf returns the namespaces of the value of a call, an object
// creation or an await: the called function or the created class. It adds no
// edge. It returns nil for a value that it cannot resolve, and records why.
func jsValueOf(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, depth int) []string {
	if depth > maxMemberExpressionDepth {
		callGraph.depthLimit(TruncationMemberDepth, currentNamespace, node, treeData)
		return nil
	}

	var target *sitter.Node
	switch node.Type() {
	case "call_expression":
		target = node.ChildByFieldName("function")
	case "new_expression":
		target = node.ChildByFieldName("constructor")
	case "await_expression":
		if node.NamedChildCount() > 0 {
			return jsValueOf(node.NamedChild(0), treeData, currentNamespace, callGraph, depth+1)
		}
	}

	switch {
	case target == nil:
	case target.Type() == "member_expression":
		return resolveJSMemberExpressionWithDepth(target, treeData, currentNamespace, callGraph, depth+1)
	case target.Type() == "identifier":
		// An imported name resolves to its module, as OpenAI to openai
		if assignment, found := searchSymbolInScopeChain(target.Content(treeData), currentNamespace, callGraph); found {
			if targets := callGraph.receiverTargets(assignment.Namespace, currentNamespace, target); len(targets) > 0 {
				return namespacesOf(targets)
			}
		}
		callee, _ := resolveJSIdentifier(target.Content(treeData), currentNamespace, callGraph)
		return []string{callee}
	}

	callGraph.addUnresolved(currentNamespace, node, treeData, UnresolvedUnknownReceiver)
	return nil
}

// resolveJSIdentifier resolves an unqualified JavaScript identifier. It
// reports false for a name that the scope chain does not bind, and returns
// the name in the current namespace.
func resolveJSIdentifier(identifier string, currentNamespace string, callGraph *CallGraph) (string, bool) {
	if assignmentNode, found := searchSymbolInScopeChain(identifier, currentNamespace, callGraph); found {
		return assignmentNode.Namespace, true
	}
	return currentNamespace + namespaceSeparator + identifier, false
}

// memberExpressionProcessor handles JavaScript member_expression nodes
// This is used for property access, not method calls (which are handled by call_expression)
func memberExpressionProcessor(memberNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if memberNode == nil {
		return newProcessorResult()
	}

	objectNode := memberNode.ChildByFieldName("object")
	propertyNode := memberNode.ChildByFieldName("property")

	if objectNode == nil || propertyNode == nil {
		return newProcessorResult()
	}

	objectSymbol := objectNode.Content(treeData)
	propertyName := propertyNode.Content(treeData)

	targetObject, objectResolved := searchSymbolInScopeChain(objectSymbol, currentNamespace, callGraph)
	if !objectResolved {
		log.Debugf("Object not found in namespace for member expression - %s.%s", objectSymbol, propertyName)
		return newProcessorResult()
	}

	result := newProcessorResult()
	for _, resolvedObject := range callGraph.receiverTargets(targetObject.Namespace, currentNamespace, memberNode) {
		finalMemberNamespace := resolvedObject.Namespace + namespaceSeparator + propertyName
		result.addAssignments(callGraph.assignmentGraph.addNode(finalMemberNamespace, memberNode))
	}

	return result
}

// arrowFunctionProcessor handles JavaScript arrow function expressions
// Arrow functions are treated similarly to function declarations
func arrowFunctionProcessor(arrowNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if arrowNode == nil {
		return newProcessorResult()
	}

	results := newProcessorResult()

	// Try to find the function name from parent variable_declarator
	// e.g., const arrowFunc = (x) => { ... }
	functionName := ""
	parentNode := arrowNode.Parent()

	for parentNode != nil && functionName == "" {
		if parentNode.Type() == "variable_declarator" {
			nameNode := parentNode.ChildByFieldName("name")
			if nameNode != nil {
				functionName = nameNode.Content(treeData)
				break
			}
		}
		// Also check for assignment_expression: arrowFunc = (x) => { ... }
		if parentNode.Type() == "assignment_expression" {
			leftNode := parentNode.ChildByFieldName("left")
			if leftNode != nil && leftNode.Type() == "identifier" {
				functionName = leftNode.Content(treeData)
				break
			}
		}
		parentNode = parentNode.Parent()
	}

	// Determine the namespace for this arrow function
	arrowFunctionNamespace := currentNamespace
	if functionName != "" {
		arrowFunctionNamespace = currentNamespace + namespaceSeparator + functionName

		// Register arrow function as a callable node
		if _, exists := callGraph.Nodes[arrowFunctionNamespace]; !exists {
			callGraph.addNode(arrowFunctionNamespace, arrowNode)
			log.Debugf("Register arrow function definition for %s - %s", functionName, arrowFunctionNamespace)
		}

		// Mark this as an assignment so it can be resolved when called
		callGraph.assignmentGraph.addNode(arrowFunctionNamespace, arrowNode)
	}

	results.addResults(processFunctionBody(arrowNode, treeData, arrowFunctionNamespace, callGraph, metadata))
	return results
}

// processFunctionBody processes the body of a JavaScript function in
// namespace. The body of an arrow function can be one expression, as in
// (k) => parseInt(k), and the expression is then a call of its own.
func processFunctionBody(function *sitter.Node, treeData []byte, namespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	body := function.ChildByFieldName("body")
	if body == nil {
		return newProcessorResult()
	}

	metadata.insideFunction = true
	if body.Type() == "statement_block" {
		return processChildren(body, treeData, namespace, callGraph, metadata)
	}
	return processNode(body, treeData, namespace, callGraph, metadata)
}

// methodDefinitionProcessor handles JavaScript method_definition nodes in classes
// This is for class methods, similar to Java methods
func methodDefinitionProcessor(methodDefNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if methodDefNode == nil {
		return newProcessorResult()
	}

	methodNameNode := methodDefNode.ChildByFieldName("name")
	if methodNameNode == nil {
		log.Debugf("Method definition without name - %s", methodDefNode.Content(treeData))
		return newProcessorResult()
	}

	// Method definition has its own scope, hence its own namespace
	methodName := methodNameNode.Content(treeData)
	methodNamespace := currentNamespace + namespaceSeparator + methodName

	// Add method to the call graph
	if _, exists := callGraph.Nodes[methodNamespace]; !exists {
		callGraph.addNode(methodNamespace, methodDefNode)
		log.Debugf("Register JavaScript method definition for %s - %s", methodName, methodNamespace)

		// Add virtual method call from class instance => method
		if metadata.insideClass {
			instanceKeyword, exists := callGraph.getInstanceKeyword()
			if exists {
				instanceNamespace := currentNamespace + namespaceSeparator + instanceKeyword + namespaceSeparator + methodName
				callGraph.addEdge(
					instanceNamespace, nil, nil,
					methodNamespace, methodDefNode,
					[]CallArgument{},
				)
				log.Debugf("Register instance method definition for %s - %s\n", methodName, instanceNamespace)
			}

			// Register constructor
			if methodName == "constructor" {
				callGraph.addEdge(
					currentNamespace, nil, nil,
					methodNamespace, methodDefNode,
					[]CallArgument{},
				)
				log.Debugf("Register class constructor for %s", currentNamespace)
			}
		}
	}

	results := newProcessorResult()

	// Process method body
	methodBody := methodDefNode.ChildByFieldName("body")
	if methodBody != nil {
		metadata.insideFunction = true
		result := processChildren(methodBody, treeData, methodNamespace, callGraph, metadata)
		metadata.insideFunction = false
		results.addResults(result)
	}

	return results
}

// jsNewExpressionProcessor handles JavaScript new_expression nodes (constructor calls)
// Examples:
// - new TestClass("test", 42) - simple identifier constructor
// - new sqlite3.Database(':memory:') - member expression constructor
func jsNewExpressionProcessor(newNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if newNode == nil {
		return newProcessorResult()
	}

	result := newProcessorResult()

	// Get constructor name
	constructorNode := newNode.ChildByFieldName("constructor")
	if constructorNode == nil {
		return result
	}

	// Get arguments
	argumentsNode := newNode.ChildByFieldName("arguments")
	callArguments := []CallArgument{}
	if argumentsNode != nil {
		callArguments = resolveCallArguments(argumentsNode, treeData, currentNamespace, callGraph, metadata)
	}

	var constructors []string
	unbound := false

	switch constructorNode.Type() {
	case "member_expression":
		// Constructor like: new sqlite3.Database() or new pkg.ClassName()
		constructors = resolveJSMemberExpression(constructorNode, treeData, currentNamespace, callGraph)
	case "identifier":
		// Simple constructor: new TestClass()
		constructor, bound := resolveJSIdentifier(constructorNode.Content(treeData), currentNamespace, callGraph)
		constructors, unbound = []string{constructor}, !bound
	default:
		// Fallback: use content as-is
		constructors = []string{constructorNode.Content(treeData)}
	}

	for _, constructor := range constructors {
		log.Debugf("JS constructor resolved to %s", constructor)

		callGraph.addCall(currentNamespace, nil, CallReference{
			CalleeNamespace:  constructor,
			CallerIdentifier: newNode,
			Arguments:        callArguments,
			Unbound:          unbound,
		})

		// Try to find the class/constructor in the assignment graph for return value tracking
		if classAssignment, classResolved := searchSymbolInScopeChain(constructor, currentNamespace, callGraph); classResolved {
			result.addAssignments(classAssignment)
		}
	}

	return result
}
