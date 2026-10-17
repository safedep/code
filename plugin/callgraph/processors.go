package callgraph

import (
	"fmt"
	"strings"

	"github.com/safedep/dry/log"
	sitter "github.com/smacker/go-tree-sitter"
)

const (
	// The depth limits stop the recursion on deep chains, as in generated
	// JavaScript.
	maxMemberExpressionDepth = 10
	maxAttributeDepth        = 10
	maxProcessingDepth       = 50

	// maxPolymorphicResolutions limits the values of one variable that a
	// member call resolves, so that the names do not multiply.
	maxPolymorphicResolutions = 20
)

type processorMetadata struct {
	insideClass     bool
	insideFunction  bool
	processingDepth int
}

type processorResult struct {
	ImmediateAssignments []*assignmentNode
}

func newProcessorResult() processorResult {
	return processorResult{
		ImmediateAssignments: []*assignmentNode{},
	}
}

func (pr *processorResult) ToString() string {
	result := "Immediate Assignments:\n"
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

func (pr *processorResult) addResults(results ...processorResult) {
	for _, result := range results {
		pr.ImmediateAssignments = append(pr.ImmediateAssignments, result.ImmediateAssignments...)
	}
}

type nodeProcessor func(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult

// processNode processes node with the processor of its type in the profile
// of the language, or else with a grammar-neutral processor.
func processNode(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if node == nil {
		return newProcessorResult()
	}

	processor, ok := callGraph.profile.processors[node.Type()]
	if !ok {
		processor = neutralProcessor(node.Type())
	}
	return processor(node, treeData, currentNamespace, callGraph, metadata)
}

// neutralProcessor returns the processor of a node type that has one meaning
// in every grammar, such as a literal, a comment or a statement.
func neutralProcessor(nodeType string) nodeProcessor {
	switch {
	case literalNodeTypes[nodeType]:
		return literalValueProcessor
	case initialisedDataStructures[nodeType]:
		return skipResultsProcessor
	case ignoredNodeTypes[nodeType]:
		return skippedProcessor
	}
	switch nodeType {
	case "identifier":
		return identifierProcessor
	case "expression_statement":
		return expressionStatementProcessor
	}
	return emptyProcessor
}

func processChildren(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if node == nil || callGraph.ctxErr != nil {
		return newProcessorResult()
	}

	// The depth limit stops a stack overflow on deep nesting.
	if metadata.processingDepth > maxProcessingDepth {
		callGraph.truncate(TruncationDepth, currentNamespace, node)
		return newProcessorResult()
	}

	childrenResults := newProcessorResult()
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

// skipResultsProcessor processes the children and gives no result to the
// parent.
func skipResultsProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if node == nil {
		return newProcessorResult()
	}

	processChildren(node, treeData, currentNamespace, callGraph, metadata)
	return newProcessorResult()
}

// expressionStatementProcessor processes a statement and gives no result to
// the parent. At module level it processes each child with no depth step,
// so the calls of a long script stay in the graph.
func expressionStatementProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if node == nil {
		return newProcessorResult()
	}

	if !metadata.insideFunction && !metadata.insideClass {
		for i := 0; i < int(node.ChildCount()); i++ {
			if child := node.Child(i); child != nil {
				processNode(child, treeData, currentNamespace, callGraph, metadata)
			}
		}
	} else {
		processChildren(node, treeData, currentNamespace, callGraph, metadata)
	}

	return newProcessorResult()
}

// emptyProcessor processes the children and gives their results to the
// parent.
func emptyProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if node == nil {
		return newProcessorResult()
	}

	return processChildren(node, treeData, currentNamespace, callGraph, metadata)
}

// skippedProcessor is the processor of a node with no calls below it.
func skippedProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	return newProcessorResult()
}

// literalValueProcessor gives the literal as its value. A literal belongs to
// no scope, so its namespace is its text.
func literalValueProcessor(literalNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if literalNode == nil {
		return newProcessorResult()
	}

	result := newProcessorResult()
	result.addAssignments(callGraph.assignmentGraph.addNode(literalNode.Content(treeData), literalNode))
	return result
}

// classDefinitionProcessor processes a class in its own namespace. The class
// is in the assignment graph too, so a call of the class name resolves to
// its constructor.
func classDefinitionProcessor(classDefNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if classDefNode == nil {
		return newProcessorResult()
	}

	classNameNode := classDefNode.ChildByFieldName("name")
	if classNameNode == nil {
		log.Debugf("Class definition without name - %s", classDefNode.Content(treeData))
		return newProcessorResult()
	}

	classNamespace := currentNamespace + namespaceSeparator + classNameNode.Content(treeData)
	callGraph.addNode(classNamespace, classDefNode)
	callGraph.assignmentGraph.addNode(classNamespace, classDefNode)
	callGraph.classConstructors[classNamespace] = true

	if keyword := callGraph.profile.instanceKeyword; keyword != "" {
		callGraph.addNode(classNamespace+namespaceSeparator+keyword, nil)
	}

	classBody := classDefNode.ChildByFieldName("body")
	if classBody == nil {
		log.Debugf("Class definition without body - %s", classDefNode.Content(treeData))
		return newProcessorResult()
	}

	metadata.insideClass = true
	processChildren(classBody, treeData, classNamespace, callGraph, metadata)

	log.Debugf("Register class definition for %s - %s", classNameNode.Content(treeData), classNamespace)
	return newProcessorResult()
}

// functionDefinitionProcessor processes a function in its own namespace. The
// instance calls a method, the class calls its constructor method and the
// root namespace calls the entry function of the language.
func functionDefinitionProcessor(functionDefNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if functionDefNode == nil {
		return newProcessorResult()
	}

	functionNameNode := functionDefNode.ChildByFieldName("name")
	if functionNameNode == nil {
		log.Debugf("Function definition without name - %s", functionDefNode.Content(treeData))
		return newProcessorResult()
	}

	funcName := functionNameNode.Content(treeData)
	functionNamespace := currentNamespace + namespaceSeparator + funcName
	profile := callGraph.profile

	// The edge from the root comes first, so the entry function gets no
	// edge from the instance.
	if profile.entryFunction != "" && funcName == profile.entryFunction {
		callGraph.addEdge(resolveRootNamespaceQualifier(currentNamespace), nil, nil, functionNamespace, functionDefNode, []CallArgument{})
	}

	if _, exists := callGraph.Nodes[functionNamespace]; !exists {
		callGraph.addNode(functionNamespace, functionDefNode)
		log.Debugf("Register function definition for %s - %s", funcName, functionNamespace)

		if metadata.insideClass {
			if keyword := profile.instanceKeyword; keyword != "" {
				instanceNamespace := currentNamespace + namespaceSeparator + keyword + namespaceSeparator + funcName
				callGraph.addEdge(instanceNamespace, nil, nil, functionNamespace, functionDefNode, []CallArgument{})
			}
			if profile.constructorMethod != "" && funcName == profile.constructorMethod {
				callGraph.addEdge(currentNamespace, nil, nil, functionNamespace, functionDefNode, []CallArgument{})
			}
		}
	}

	results := newProcessorResult()
	if functionBody := functionDefNode.ChildByFieldName("body"); functionBody != nil {
		metadata.insideFunction = true
		results.addResults(processChildren(functionBody, treeData, functionNamespace, callGraph, metadata))
	}
	return results
}

// assignmentProcessor assigns the values of the right side to the left
// side. An attribute on the left, as in xyz.attr = 1, resolves to the
// attribute of each value of its object, xyz//attr.
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

	// A tuple on the left, as in a, b = 1, 2, is one name. The values do not
	// go to their names.
	assigneeNodes := []*assignmentNode{}
	if leftNode.Type() == "attribute" {
		assigneeNodes = attributeProcessor(leftNode, treeData, currentNamespace, callGraph, metadata).ImmediateAssignments
	}

	if len(assigneeNodes) == 0 {
		fallback := callGraph.assignmentGraph.addNode(currentNamespace+namespaceSeparator+leftNode.Content(treeData), leftNode)
		if fallback == nil {
			return newProcessorResult()
		}
		assigneeNodes = []*assignmentNode{fallback}
	}

	result := processNode(rightNode, treeData, currentNamespace, callGraph, metadata)
	for _, assigneeNode := range assigneeNodes {
		for _, immediateAssignment := range result.ImmediateAssignments {
			callGraph.assignmentGraph.addAssignment(assigneeNode.Namespace, nil, immediateAssignment.Namespace, nil)
		}
	}

	return newProcessorResult()
}

// ternaryExpressionProcessor gives the values of both branches. The
// condition gives no value.
func ternaryExpressionProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if node == nil {
		return newProcessorResult()
	}

	processNode(node.ChildByFieldName("condition"), treeData, currentNamespace, callGraph, metadata)

	result := newProcessorResult()
	result.addResults(
		processNode(node.ChildByFieldName("consequence"), treeData, currentNamespace, callGraph, metadata),
		processNode(node.ChildByFieldName("alternative"), treeData, currentNamespace, callGraph, metadata),
	)
	return result
}

// identifierProcessor gives the binding of a name in the scope chain. A name
// with no binding becomes a new name in the current namespace.
func identifierProcessor(identifierNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if identifierNode == nil {
		return newProcessorResult()
	}

	result := newProcessorResult()
	if assignment, found := searchSymbolInScopeChain(identifierNode.Content(treeData), currentNamespace, callGraph); found {
		result.ImmediateAssignments = append(result.ImmediateAssignments, assignment)
		return result
	}

	result.addAssignments(callGraph.assignmentGraph.addNode(currentNamespace+namespaceSeparator+identifierNode.Content(treeData), identifierNode))

	return result
}

// resolveCallArguments gives one CallArgument for each argument, in the
// order of the call, with the terminal values of the argument. An argument
// with no value keeps its place as an empty CallArgument.
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

		resolvedTerminalAssignmentNodes := []*assignmentNode{}
		for _, assignmentNode := range childProcessorResult.ImmediateAssignments {
			resolvedNodes := callGraph.assignmentGraph.resolve(assignmentNode.Namespace)
			resolvedTerminalAssignmentNodes = append(resolvedTerminalAssignmentNodes, resolvedNodes...)
		}

		result[i] = argumentOf(resolvedTerminalAssignmentNodes)
	}

	return result
}

// searchSymbolInScopeChain finds symbol in the current namespace and then in
// each parent namespace, as a//b//c//x, then a//b//x, a//x and x. It looks
// in the assignment graph, because the assignment graph also holds the
// imported names.
func searchSymbolInScopeChain(symbol string, currentNamespace string, callGraph *CallGraph) (*assignmentNode, bool) {
	if symbol == "" {
		return nil, false
	}

	for i := strings.Count(currentNamespace, namespaceSeparator) + 1; i >= 0; i-- {
		searchNamespace := strings.Join(strings.Split(currentNamespace, namespaceSeparator)[:i], namespaceSeparator) + namespaceSeparator + symbol
		if i == 0 {
			searchNamespace = symbol
		}
		searchedAssignmentNode, exists := callGraph.assignmentGraph.Assignments[searchNamespace]
		if exists {
			return searchedAssignmentNode, true
		}
	}

	return nil, false
}

// variableDeclaratorProcessor assigns the values of the initializer to the
// declared name.
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
				immediateAssignment.Namespace, nil,
			)
		}
	}

	return newProcessorResult()
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

// processFunctionBody processes the body of a function literal in
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
