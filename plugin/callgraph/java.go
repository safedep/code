package callgraph

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/safedep/code/core"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/pkg/ts"
	"github.com/safedep/dry/log"
	sitter "github.com/smacker/go-tree-sitter"
)

var javaProfile = languageProfile{
	code:            core.LanguageCodeJava,
	separators:      []string{"."},
	instanceKeyword: "this",
	entryFunction:   "main",
	builtins:        langcode.Builtins(langcode.Java),
	processors: map[string]nodeProcessor{
		"assignment_expression":           assignmentProcessor,
		"class_declaration":               classDefinitionProcessor,
		"compact_constructor_declaration": javaConstructorProcessor,
		"constructor_declaration":         javaConstructorProcessor,
		"enum_constant":                   enumConstantProcessor,
		"enum_declaration":                classDefinitionProcessor,
		"explicit_constructor_invocation": explicitConstructorInvocationProcessor,
		"interface_declaration":           classDefinitionProcessor,
		"local_variable_declaration":      localVariableDeclarationProcessor,
		"method_reference":                skippedProcessor,
		"method_declaration":              javaMethodDeclarationProcessor,
		"method_invocation":               methodInvocationProcessor,
		"object_creation_expression":      objectCreationExpressionProcessor,
		"record_declaration":              classDefinitionProcessor,
		"scoped_type_identifier":          scopedIdentifierProcessor,
		"ternary_expression":              ternaryExpressionProcessor,
		"type_arguments":                  skippedProcessor,
		"type_identifier":                 skippedProcessor,
		"variable_declarator":             variableDeclaratorProcessor,
	},
	literals: map[string]bool{
		"binary_integer_literal":         true,
		"character_literal":              true,
		"decimal_floating_point_literal": true,
		"decimal_integer_literal":        true,
		"false":                          true,
		"hex_floating_point_literal":     true,
		"hex_integer_literal":            true,
		"octal_integer_literal":          true,
		"string_literal":                 true,
		"true":                           true,
	},
	ignored: map[string]bool{"import_declaration": true, "package_declaration": true, "line_comment": true, "block_comment": true},
	reachable: map[string]bool{
		"program":                 true,
		"class_declaration":       true,
		"method_declaration":      true,
		"constructor_declaration": true,
	},
	members: map[string]memberShape{
		"method_invocation": {object: named("object"), name: named("name")},
	},
	argumentList: "argument_list",
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

// javaMethodDeclarationProcessor processes a method with a body. An
// abstract method or a method of an interface with no body runs no code,
// so it is no definition. A call of it reaches the methods that override
// it.
func javaMethodDeclarationProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if node == nil || node.ChildByFieldName("body") == nil {
		return newProcessorResult()
	}
	return functionDefinitionProcessor(node, treeData, currentNamespace, callGraph, metadata)
}

// javaConstructorProcessor processes a constructor as the <init> method of
// its class, whose body runs in the key of the constructor.
func javaConstructorProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if node == nil {
		return newProcessorResult()
	}
	return processFunctionBody(node, treeData, callGraph.defineConstructor(currentNamespace, node), callGraph, metadata)
}

// enumConstantProcessor processes a constant of an enum. A constant with a
// class body is a class of its own under the enum, as Op//PLUS, so its
// methods override the methods of the enum. Its arguments run in the scope
// of the enum.
func enumConstantProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if node == nil {
		return newProcessorResult()
	}
	if args := node.ChildByFieldName("arguments"); args != nil {
		resolveCallArguments(args, treeData, currentNamespace, callGraph, metadata)
	}
	if node.ChildByFieldName("body") != nil {
		classDefinitionProcessor(node, treeData, currentNamespace, callGraph, metadata)
	}
	return newProcessorResult()
}

// explicitConstructorInvocationProcessor adds the call of super(...) in a
// constructor, which runs the constructor of the base class. this(...)
// runs another constructor of the class, which has the same key.
func explicitConstructorInvocationProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if node == nil {
		return newProcessorResult()
	}
	args := resolveCallArguments(node.ChildByFieldName("arguments"), treeData, currentNamespace, callGraph, metadata)
	if constructor := node.ChildByFieldName("constructor"); constructor != nil && constructor.Type() == superKeyword {
		callGraph.addUnboundCall(currentNamespace, superKeyword, CallCall, constructor, args)
	}
	return newProcessorResult()
}

// objectCreationExpressionProcessor adds the edge of new SomeClass() to the
// class. The value of the creation is the class. A class that the scope
// chain does not bind, as a class of the package, keeps its name. The
// methods of an anonymous class body run in the scope of the creation,
// because the graph has no name for the class.
func objectCreationExpressionProcessor(objectCreationNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if objectCreationNode == nil {
		return newProcessorResult()
	}
	for _, child := range ts.NamedChildren(objectCreationNode) {
		if child.Type() == "class_body" {
			processAnonymousClassBody(child, treeData, currentNamespace, callGraph, metadata)
		}
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

			callGraph.addEdge(currentNamespace, CallNew, constructedClassNode, classNode.Namespace, nil, argsResult)
			markClassAssignment(classNode)
		} else if constructedClassNode.Type() == "scoped_type_identifier" {
			scopedIdentifierResult := scopedIdentifierProcessor(constructedClassNode, treeData, currentNamespace, callGraph, metadata)
			for _, scopedIdentifierAssignment := range scopedIdentifierResult.ImmediateAssignments {
				callGraph.addEdge(currentNamespace, CallNew, constructedClassNode, scopedIdentifierAssignment.Namespace, nil, argsResult)
				markClassAssignment(scopedIdentifierAssignment)
			}
		} else if name := withoutTypeArguments(constructedClassName); name != "" {
			callGraph.addUnboundCall(currentNamespace, strings.ReplaceAll(name, ".", namespaceSeparator), CallNew, constructedClassNode, argsResult)
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
		recordChainedCalls(methodInvocationNode, treeData, currentNamespace, callGraph)
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
					currentNamespace, CallCall, methodInvocationNode,
					class+namespaceSeparator+calledMethod, nil,
					argsResult,
				)
			}
			return newProcessorResult()
		}

		methodObjectQualifierNamespace := resolveQualifierObjectFieldaccess(methodQualifierObjectNode, treeData)
		// The first call of a chain names its callee by its object and
		// its name. Its arguments are no part of the name.
		if methodQualifierObjectNode.Type() == "method_invocation" {
			methodObjectQualifierNamespace = invocationNamespace(methodQualifierObjectNode, treeData)
		}
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
			callGraph.addEdge(currentNamespace, CallCall, methodInvocationNode, calledNamespace, nil, argsResult)
		}

		return newProcessorResult()
	}

	functionAssignmentNode, functionResolvedBySearch := searchSymbolInScopeChain(methodName, currentNamespace, callGraph)
	if functionResolvedBySearch {
		log.Debugf("Method invocation %s searched (direct) & resolved to %s\n", methodName, functionAssignmentNode.Namespace)
		callGraph.addEdge(
			currentNamespace, CallCall, methodInvocationNode,
			functionAssignmentNode.Namespace, nil,
			argsResult,
		)
		return newProcessorResult()
	}

	// A name that the scope chain does not bind is a method of the class
	// or of a base class, or a method that a static import names. The link
	// step resolves it.
	callGraph.addUnboundCall(currentNamespace, methodName, CallCall, methodInvocationNode, argsResult)
	return newProcessorResult()
}

// processAnonymousClassBody processes the members of an anonymous class in
// the scope that creates it. The body of each method runs there too.
func processAnonymousClassBody(body *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) {
	for _, member := range ts.NamedChildren(body) {
		if member.Type() == "method_declaration" {
			processFunctionBody(member, treeData, currentNamespace, callGraph, metadata)
			continue
		}
		processNode(member, treeData, currentNamespace, callGraph, metadata)
	}
}

// recordChainedCalls records each call of a chain on the value of the call
// before it, as exec in Runtime.getRuntime().exec(), as a call on an
// unknown receiver. The graph keeps only the first call of the chain, and
// the value of a call is out of reach. The record sits at the name of the
// method, so it does not hide the first call at the start of the chain.
func recordChainedCalls(invocation *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph) {
	for node := invocation; node != nil && node.Type() == "method_invocation"; node = node.ChildByFieldName("object") {
		object := node.ChildByFieldName("object")
		if object == nil || object.Type() != "method_invocation" {
			return
		}
		if name := node.ChildByFieldName("name"); name != nil {
			callGraph.addUnresolved(currentNamespace, name, node, treeData, UnresolvedUnknownReceiver)
		}
	}
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

// invocationNamespace returns the namespace of the callee of a method
// invocation as written, as java//util//Logger//getLogger for
// java.util.Logger.getLogger("x").
func invocationNamespace(invocation *sitter.Node, treeData []byte) string {
	name := invocation.ChildByFieldName("name")
	if name == nil {
		return ""
	}
	object := invocation.ChildByFieldName("object")
	if object == nil {
		return name.Content(treeData)
	}
	return resolveQualifierObjectFieldaccess(object, treeData) + namespaceSeparator + name.Content(treeData)
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
