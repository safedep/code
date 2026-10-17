package callgraph

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/safedep/code/core"
	"github.com/safedep/dry/log"
	sitter "github.com/smacker/go-tree-sitter"
)

var javaProfile = languageProfile{
	code:            core.LanguageCodeJava,
	separators:      []string{"."},
	instanceKeyword: "this",
	entryFunction:   "main",
	builtins: []string{
		"Object", "Class", "Enum", "String", "StringBuilder", "StringBuffer", "System", "Runtime",
		"Thread", "ThreadGroup", "Throwable", "Exception", "Error", "Math", "StrictMath", "Number",
		"Void", "Package", "Process", "ProcessBuilder", "SecurityManager", "Boolean", "Byte",
		"Character", "Short", "Integer", "Long", "Float", "Double",
	},
	processors: map[string]nodeProcessor{
		"assignment_expression":      assignmentProcessor,
		"class_declaration":          classDefinitionProcessor,
		"enum_declaration":           skipResultsProcessor,
		"interface_body":             skippedProcessor,
		"interface_declaration":      skipResultsProcessor,
		"local_variable_declaration": localVariableDeclarationProcessor,
		"method_declaration":         functionDefinitionProcessor,
		"method_invocation":          methodInvocationProcessor,
		"object_creation_expression": objectCreationExpressionProcessor,
		"scoped_type_identifier":     scopedIdentifierProcessor,
		"ternary_expression":         ternaryExpressionProcessor,
		"type_arguments":             skippedProcessor,
		"type_identifier":            skippedProcessor,
		"variable_declarator":        variableDeclaratorProcessor,
	},
	reachable: map[string]bool{
		"program":            true,
		"class_declaration":  true,
		"method_declaration": true,
	},
}

// scopedIdentifierProcessor gives the namespace of a qualified type name, as
// java//awt//event//MouseEvent. The first segment resolves through the scope
// chain. A first segment with no binding names a type with no import.
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

	importNamespace := targetObjectIdentifier + namespaceSeparator + objectQualifierNamespace
	log.Debugf("Scoped identifier %s - fallback to fully qualified import - %s", scopedIdentifierNode.Content(treeData), importNamespace)

	result := newProcessorResult()
	result.addAssignments(callGraph.assignmentGraph.addNode(importNamespace, scopedIdentifierNode))

	return result
}

// disectScopedIdentifierQualifier splits a qualified type name into its first
// segment and the namespace of the rest, as java and awt//event//MouseEvent.
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

// objectCreationExpressionProcessor adds the edge of new SomeClass() to the
// class. The value of the creation is the class.
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
			result.ImmediateAssignments = append(result.ImmediateAssignments, classAssignmentNode)
		}
	}

	constructedClassNode := objectCreationNode.ChildByFieldName("type")
	if constructedClassNode != nil {
		constructedClassName := constructedClassNode.Content(treeData)

		classNode, classResolvedBySearch := searchSymbolInScopeChain(constructedClassName, currentNamespace, callGraph)
		if classResolvedBySearch {
			log.Debugf("Constructor %s searched (direct) & resolved to %s\n", constructedClassName, classNode.Namespace)

			callGraph.addEdge(currentNamespace, nil, objectCreationNode, classNode.Namespace, nil, argsResult)
			markClassAssignment(classNode)
		} else if constructedClassNode.Type() == "scoped_type_identifier" {
			scopedIdentifierResult := scopedIdentifierProcessor(constructedClassNode, treeData, currentNamespace, callGraph, metadata)
			for _, scopedIdentifierAssignment := range scopedIdentifierResult.ImmediateAssignments {
				callGraph.addEdge(currentNamespace, nil, objectCreationNode, scopedIdentifierAssignment.Namespace, nil, argsResult)
				markClassAssignment(scopedIdentifierAssignment)
			}
		}
	}

	return result
}

// localVariableDeclarationProcessor processes the declarator. The type, as
// int in int a = 1, gives no value.
func localVariableDeclarationProcessor(declarationNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if declarationNode == nil {
		return newProcessorResult()
	}

	declaratorNode := declarationNode.ChildByFieldName("declarator")
	if declaratorNode != nil {
		processNode(declaratorNode, treeData, currentNamespace, callGraph, metadata)
	}

	return newProcessorResult()
}

// processMethodArgs gives the arguments of a method invocation.
func processMethodArgs(methodInvocationNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) []CallArgument {
	if methodInvocationNode == nil || methodInvocationNode.Type() != "method_invocation" {
		log.Warnf("Incorrect method invocation node processed for args - %s", methodInvocationNode.Content(treeData))
		return []CallArgument{}
	}

	argumentsNode := methodInvocationNode.ChildByFieldName("arguments")
	if argumentsNode == nil {
		return []CallArgument{}
	}

	return resolveCallArguments(argumentsNode, treeData, currentNamespace, callGraph, metadata)
}

// methodInvocationProcessor adds the edges of a Java method call. In a chain,
// as in xyz.method1().method2(), it resolves only the first call of the chain,
// xyz//method1, because the value of a call is out of reach.
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

	hasChainedMethodInvocations := false

	methodQualifierObjectNode := methodInvocationNode.ChildByFieldName("object")

	if methodQualifierObjectNode != nil {
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
				// A root with no binding is the root of a qualified name, as sun
				// in sun.reflect.Method.
				callGraph.addNode(rootObjKeyword, nil)
				rootCallerObjAssignments = callGraph.assignmentGraph.resolve(rootObjKeyword)
			}

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
			callGraph.addEdge(currentNamespace, nil, methodInvocationNode, calledNamespace, methodInvocationNode, argsResult)
		}

		return newProcessorResult()
	}

	functionAssignmentNode, functionResolvedBySearch := searchSymbolInScopeChain(methodName, currentNamespace, callGraph)
	if functionResolvedBySearch {
		log.Debugf("Method invocation %s searched (direct) & resolved to %s\n", methodName, functionAssignmentNode.Namespace)
		callGraph.addEdge(
			currentNamespace, nil, methodInvocationNode,
			functionAssignmentNode.Namespace, nil,
			argsResult,
		)
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

// resolveQualifierObjectFieldaccess gives the namespace of the object of a
// method invocation with no parentheses, as xyz//attr//fncall for
// xyz.attr.fncall().
func resolveQualifierObjectFieldaccess(invokedObjNode *sitter.Node, treeData []byte) string {
	if invokedObjNode == nil {
		return ""
	}

	normalisedString := methodInvocationNormaliserRegexp.ReplaceAllString(invokedObjNode.Content(treeData), "")

	return strings.ReplaceAll(normalisedString, ".", namespaceSeparator)
}
