package callgraph

import (
	"github.com/safedep/code/core"
	"github.com/safedep/dry/log"
	sitter "github.com/smacker/go-tree-sitter"
)

// goProfile has no instance keyword. A Go method names its receiver.
var goProfile = languageProfile{
	code:               core.LanguageCodeGo,
	separators:         []string{"/"},
	rootCallsFunctions: true,
	builtins: []string{
		"panic", "recover", "make", "new", "len", "cap", "append", "copy", "delete", "close",
		"complex", "real", "imag", "print", "println", "iota", "true", "false", "nil",
	},
	processors: map[string]nodeProcessor{
		"call_expression":      goCallExpressionProcessor,
		"composite_literal":    skipResultsProcessor,
		"function_declaration": functionDefinitionProcessor,
		"method_declaration":   goMethodDeclarationProcessor,
		"slice_expression":     skipResultsProcessor,
		"type_arguments":       skippedProcessor,
		"type_identifier":      skippedProcessor,
	},
	literals: map[string]bool{
		"false":                      true,
		"float_literal":              true,
		"imaginary_literal":          true,
		"int_literal":                true,
		"interpreted_string_literal": true,
		"raw_string_literal":         true,
		"rune_literal":               true,
		"true":                       true,
	},
	ignored: map[string]bool{"comment": true, "import_declaration": true},
	reachable: map[string]bool{
		"function_declaration": true,
		"method_declaration":   true,
	},
	transparent: map[string]childPosition{"parenthesized_expression": firstChild},
	members: map[string]memberShape{
		"selector_expression": {object: "operand", name: "field"},
	},
	argumentList:     "argument_list",
	functionLiterals: map[string]bool{"func_literal": true},
	valueRoots:       map[string]bool{"call_expression": true, "composite_literal": true},
}

// goCallExpressionProcessor adds the edges of a Go call, as os//WriteFile for
// os.WriteFile(name, data, 0644) and helper for helper(10).
func goCallExpressionProcessor(callNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if callNode == nil {
		return newProcessorResult()
	}

	result := newProcessorResult()

	functionNode := callNode.ChildByFieldName("function")
	if functionNode == nil {
		return result
	}

	argumentsNode := callNode.ChildByFieldName("arguments")
	callArguments := []CallArgument{}
	if argumentsNode != nil {
		callArguments = resolveCallArguments(argumentsNode, treeData, currentNamespace, callGraph, metadata)
	}

	function := callGraph.unwrap(functionNode)

	// A chained call, as in ecdh.X25519().GenerateKey() or
	// openai.NewClient(k).Chat.Completions.New(), calls the inner function too
	if root := callGraph.unwrap(chainRoot(function, "selector_expression", "operand")); root != function && root != nil && callGraph.profile.valueRoots[root.Type()] {
		processNode(root, treeData, currentNamespace, callGraph, metadata)
	}

	var callees []string
	var reason UnresolvedReason
	unbound := false

	switch function.Type() {
	case "selector_expression":
		callees, reason = resolveGoSelectorExpression(function, treeData, currentNamespace, callGraph)
	case "identifier":
		callee, bound := resolveIdentifier(function.Content(treeData), currentNamespace, callGraph)
		callees, unbound = []string{callee}, !bound
	case "unary_expression":
		// (*T)(p) converts p to a pointer type and calls nothing
	default:
		if callGraph.profile.functionLiterals[function.Type()] {
			// The body of an immediately called function runs in the caller, as
			// in go func() { work() }()
			processFunctionBody(function, treeData, currentNamespace, callGraph, metadata)
			break
		}
		processNode(function, treeData, currentNamespace, callGraph, metadata)
		reason = UnresolvedComputedName
	}

	if len(callees) == 0 && reason != "" {
		callGraph.addUnresolved(currentNamespace, functionNode, function, treeData, reason)
	}

	for _, callee := range callees {
		ref := callGraph.reference(callee, CallCall, functionNode, callArguments)
		ref.Unbound = unbound
		callGraph.addCall(currentNamespace, nil, ref)
		log.Debugf("Go call: %s -> %s", currentNamespace, callee)
	}

	return result
}

// resolveGoSelectorExpression resolves Go selector expressions like pkg.Func
// or obj.Method. A variable with more than one value gives one name for
// each value. It returns no name for a selector that it cannot resolve, and
// the reason.
func resolveGoSelectorExpression(selectorNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph) ([]string, UnresolvedReason) {
	operandNode := callGraph.unwrap(selectorNode.ChildByFieldName("operand"))
	fieldNode := selectorNode.ChildByFieldName("field")
	if operandNode == nil || fieldNode == nil {
		return nil, UnresolvedUnknownReceiver
	}
	fieldName := fieldNode.Content(treeData)

	// A call or a selector as the operand is the base of the name, as in
	// ecdh.X25519().GenerateKey or client.Chat.Completions.New
	switch operandNode.Type() {
	case "call_expression":
		bases, reason := goCalleeOf(operandNode, treeData, currentNamespace, callGraph)
		return qualify(bases, fieldName), reason
	case "selector_expression":
		bases, reason := resolveGoSelectorExpression(operandNode, treeData, currentNamespace, callGraph)
		return qualify(bases, fieldName), reason
	case "identifier":
	default:
		return nil, UnresolvedUnknownReceiver
	}

	// An imported package or a variable
	operandName := operandNode.Content(treeData)
	if operandAssignment, operandResolved := searchSymbolInScopeChain(operandName, currentNamespace, callGraph); operandResolved {
		if targets := callGraph.receiverTargets(operandAssignment.Namespace, currentNamespace, selectorNode); len(targets) > 0 {
			return qualify(namespacesOf(targets), fieldName), ""
		}
	}

	// The scope chain does not know the operand, as a parameter or a local
	// variable. The name keeps the operand as written.
	callGraph.addUnresolved(currentNamespace, selectorNode, selectorNode, treeData, UnresolvedUnknownReceiver)
	return []string{operandName + namespaceSeparator + fieldName}, ""
}

// goCalleeOf returns the namespaces of the function of a Go call. It adds no
// edge.
func goCalleeOf(callNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph) ([]string, UnresolvedReason) {
	function := callGraph.unwrap(callNode.ChildByFieldName("function"))
	if function == nil {
		return nil, UnresolvedUnknownReceiver
	}
	switch function.Type() {
	case "selector_expression":
		return resolveGoSelectorExpression(function, treeData, currentNamespace, callGraph)
	case "identifier":
		callee, _ := resolveIdentifier(function.Content(treeData), currentNamespace, callGraph)
		return []string{callee}, ""
	}
	return nil, UnresolvedUnknownReceiver
}

// goMethodDeclarationProcessor processes a method in the namespace of its
// receiver type, as file//ReceiverType//MethodName. A receiver with no type
// name puts the method in the current namespace.
func goMethodDeclarationProcessor(methodDefNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if methodDefNode == nil {
		return newProcessorResult()
	}

	methodNameNode := methodDefNode.ChildByFieldName("name")
	if methodNameNode == nil {
		log.Debugf("Go method declaration without name - %s", methodDefNode.Content(treeData))
		return newProcessorResult()
	}

	receiverNode := methodDefNode.ChildByFieldName("receiver")
	receiverType := extractGoReceiverType(receiverNode, treeData)

	methodName := methodNameNode.Content(treeData)

	methodNamespace := currentNamespace + namespaceSeparator + methodName
	if receiverType != "" {
		methodNamespace = currentNamespace + namespaceSeparator + receiverType + namespaceSeparator + methodName
	}

	if callGraph.addDefinition(methodNamespace, methodDefNode) {
		log.Debugf("Register Go method definition for %s on %s - %s", methodName, receiverType, methodNamespace)
	}
	return processFunctionBody(methodDefNode, treeData, methodNamespace, callGraph, metadata)
}

// extractGoReceiverType returns the type name of a receiver, as MyType for
// (m *MyType).
func extractGoReceiverType(receiverNode *sitter.Node, treeData []byte) string {
	if receiverNode == nil {
		return ""
	}

	for i := 0; uint32(i) < receiverNode.ChildCount(); i++ {
		child := receiverNode.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "parameter_declaration" {
			for j := 0; uint32(j) < child.ChildCount(); j++ {
				typeNode := child.Child(j)
				if typeNode == nil {
					continue
				}

				switch typeNode.Type() {
				case "type_identifier":
					return typeNode.Content(treeData)
				case "pointer_type":
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
