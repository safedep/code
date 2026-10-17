package callgraph

import (
	"strings"

	"github.com/safedep/code/core"
	"github.com/safedep/dry/log"
	sitter "github.com/smacker/go-tree-sitter"
)

// JavaScript and TypeScript share their processors. A node type that only
// the TypeScript grammar has, as type_annotation, never comes from the
// JavaScript grammar.

var javascriptProfile = ecmascriptProfile(core.LanguageCodeJavascript, "")

var typescriptProfile = ecmascriptProfile(core.LanguageCodeTypescript, core.LanguageCodeJavascript)

func ecmascriptProfile(code, signatureFallback core.LanguageCode) languageProfile {
	return languageProfile{
		code:              code,
		separators:        []string{"/"},
		instanceKeyword:   "this",
		constructorMethod: "constructor",
		defaultExport:     "default",
		signatureFallback: signatureFallback,
		builtins:          ecmascriptBuiltins,
		processors:        ecmascriptProcessors,
		literals: map[string]bool{
			"false": true, "null": true, "number": true, "string": true, "true": true, "undefined": true,
		},
		collections: map[string]bool{"array": true, "object": true},
		ignored:     map[string]bool{"comment": true, "import": true, "import_statement": true},
		reachable: map[string]bool{
			"program":                        true,
			"class_declaration":              true,
			"abstract_class_declaration":     true,
			"function_declaration":           true,
			"generator_function_declaration": true,
			"function_expression":            true,
			"arrow_function":                 true,
			"method_definition":              true,
		},
		transparent: map[string]childPosition{
			"parenthesized_expression": firstChild,
			"await_expression":         firstChild,
			"sequence_expression":      lastChild,
			"non_null_expression":      firstChild,
			"as_expression":            firstChild,
			"satisfies_expression":     firstChild,
		},
		members: map[string]memberShape{
			"member_expression":    {object: "object", name: "property"},
			"subscript_expression": {object: "object", name: "index", index: true},
		},
		strings: stringShape{
			literals:       map[string]bool{"string": true, "template_string": true},
			fragments:      map[string]bool{"string_fragment": true},
			interpolations: map[string]bool{"template_substitution": true},
		},
		argumentList: "arguments",
		functionLiterals: map[string]bool{
			"function_expression": true,
			"function":            true,
			"arrow_function":      true,
			"generator_function":  true,
		},
		valueRoots: map[string]bool{
			"call_expression": true,
			"new_expression":  true,
			"array":           true,
			"object":          true,
			"template_string": true,
		},
		literalReceivers: map[string]string{
			"string":          "String",
			"template_string": "String",
			"number":          "Number",
			"array":           "Array",
			"object":          "Object",
			"regex":           "RegExp",
		},
	}
}

var ecmascriptBuiltins = []string{
	"console", "parseInt", "parseFloat", "isNaN", "isFinite", "setTimeout", "setInterval",
	"clearTimeout", "clearInterval", "eval", "encodeURI", "decodeURI", "encodeURIComponent",
	"decodeURIComponent", "Number", "String", "Boolean", "Array", "Object", "Function", "Date",
	"Math", "RegExp", "JSON", "Promise", "Symbol", "BigInt", "Map", "Set", "WeakMap", "WeakSet",
	"Reflect", "Proxy", "Intl", "undefined", "NaN", "Infinity", "process", "global",
	"globalThis", "window", "document", "navigator", "Buffer", "module", "exports", "require",
	"URL", "fetch", "Error", "TypeError", "AbortController", "TextEncoder", "TextDecoder",
	"crypto",
}

// ecmascriptProcessors skip the type-level nodes of TypeScript, so a type
// gives no call edge.
var ecmascriptProcessors = map[string]nodeProcessor{
	"arrow_function":             arrowFunctionProcessor,
	"assignment_expression":      assignmentProcessor,
	"call_expression":            jsCallExpressionProcessor,
	"class_declaration":          classDefinitionProcessor,
	"function_declaration":       functionDefinitionProcessor,
	"lexical_declaration":        lexicalDeclarationProcessor,
	"member_expression":          memberExpressionProcessor,
	"method_definition":          functionDefinitionProcessor,
	"new_expression":             jsNewExpressionProcessor,
	"ternary_expression":         ternaryExpressionProcessor,
	"variable_declarator":        variableDeclaratorProcessor,
	"abstract_class_declaration": classDefinitionProcessor,
	"enum_declaration":           skipResultsProcessor,
	"interface_declaration":      skipResultsProcessor,
	"public_field_definition":    skipResultsProcessor,
	"abstract_method_signature":  skippedProcessor,
	"accessibility_modifier":     skippedProcessor,
	"as_expression":              skippedProcessor,
	"class_heritage":             skippedProcessor,
	"extends_clause":             skippedProcessor,
	"extends_type_clause":        skippedProcessor,
	"implements_clause":          skippedProcessor,
	"interface_body":             skippedProcessor,
	"override_modifier":          skippedProcessor,
	"predefined_type":            skippedProcessor,
	"readonly":                   skippedProcessor,
	"satisfies_expression":       skippedProcessor,
	"type_alias_declaration":     skippedProcessor,
	"type_annotation":            skippedProcessor,
	"type_arguments":             skippedProcessor,
	"type_identifier":            skippedProcessor,
}

// lexicalDeclarationProcessor processes each declarator of const, let or
// var.
func lexicalDeclarationProcessor(declarationNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if declarationNode == nil {
		return newProcessorResult()
	}

	for i := 0; uint32(i) < declarationNode.NamedChildCount(); i++ {
		declaratorNode := declarationNode.NamedChild(i)
		if declaratorNode != nil && declaratorNode.Type() == "variable_declarator" {
			processNode(declaratorNode, treeData, currentNamespace, callGraph, metadata)
		}
	}

	return newProcessorResult()
}

// jsCallExpressionProcessor adds the edges of a JavaScript call, as
// console//log for console.log("hello") and myFunc for myFunc(10).
func jsCallExpressionProcessor(callNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
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
	processJSChainRoot(function, treeData, currentNamespace, callGraph, metadata)

	var callees []string
	var reason UnresolvedReason
	unbound := false

	switch function.Type() {
	case "member_expression":
		callees, reason = resolveJSMemberExpression(function, treeData, currentNamespace, callGraph)
	case "subscript_expression":
		callees, reason = resolveJSSubscript(function, treeData, currentNamespace, callGraph, 0)
	case "identifier":
		name := function.Content(treeData)
		callee, bound := resolveIdentifier(name, currentNamespace, callGraph)
		callees, unbound = []string{callee}, !bound
		if name == "require" && !callGraph.profile.staticString(firstArgument(argumentsNode)) {
			callGraph.addUnresolved(currentNamespace, callNode, function, treeData, UnresolvedDynamicImport)
		}
	case "import":
		if module, ok := jsDynamicImport(argumentsNode, treeData, callGraph.profile); ok {
			callees = []string{module}
		} else {
			callGraph.addUnresolved(currentNamespace, callNode, function, treeData, UnresolvedDynamicImport)
		}
	case "super":
		callees = []string{"super"}
	default:
		if callGraph.profile.functionLiterals[function.Type()] {
			// The body of an immediately invoked function runs in the caller
			processFunctionBody(function, treeData, currentNamespace, callGraph, metadata)
			break
		}
		// The inner call is a call of its own, as foo in foo()()
		processNode(function, treeData, currentNamespace, callGraph, metadata)
		reason = UnresolvedComputedName
	}

	if len(callees) == 0 && reason != "" {
		callGraph.addUnresolved(currentNamespace, functionNode, function, treeData, reason)
	}

	for _, callee := range callees {
		ref := callGraph.reference(callee, functionNode, callArguments)
		ref.Unbound = unbound
		callGraph.addCall(currentNamespace, nil, nil, ref)
		log.Debugf("JS call: %s -> %s", currentNamespace, callee)
	}

	return result
}

// jsDynamicImport returns the module of a dynamic import, as in
// await import("x"), in the form of a Ruby require or a C# using: x//*.
// The import loads the module, but the names it binds are often out of
// reach, as in import("x").then(({ pipeline }) => pipeline()).
func jsDynamicImport(argumentsNode *sitter.Node, treeData []byte, profile *languageProfile) (string, bool) {
	module := firstArgument(argumentsNode)
	if !profile.staticString(module) {
		return "", false
	}
	name := strings.Trim(module.Content(treeData), "'\"`")
	if name == "" {
		return "", false
	}
	return profile.namespaceOf(name) + namespaceSeparator + "*", true
}

// resolveJSMemberExpression resolves JavaScript member expressions like
// obj.method or pkg.func. A variable with more than one value gives one name
// for each value. It returns no name for a member that it cannot resolve,
// and the reason.
func resolveJSMemberExpression(memberNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph) ([]string, UnresolvedReason) {
	return resolveJSMemberExpressionWithDepth(memberNode, treeData, currentNamespace, callGraph, 0)
}

func resolveJSMemberExpressionWithDepth(memberNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, depth int) ([]string, UnresolvedReason) {
	if depth > maxMemberExpressionDepth {
		callGraph.truncate(TruncationMemberDepth, currentNamespace, memberNode)
		return nil, UnresolvedLimit
	}

	propertyNode := memberNode.ChildByFieldName("property")
	if propertyNode == nil {
		return nil, UnresolvedUnknownReceiver
	}

	bases, reason := jsReceiverOf(memberNode.ChildByFieldName("object"), memberNode, treeData, currentNamespace, callGraph, depth)
	return qualify(bases, propertyNode.Content(treeData)), reason
}

// resolveJSSubscript resolves a member that a string literal names, as
// obj['x'] to obj//x. An index that computes the name gives no name.
func resolveJSSubscript(subscriptNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, depth int) ([]string, UnresolvedReason) {
	if depth > maxMemberExpressionDepth {
		callGraph.truncate(TruncationMemberDepth, currentNamespace, subscriptNode)
		return nil, UnresolvedLimit
	}

	name, ok := callGraph.profile.memberName(subscriptNode.ChildByFieldName("index"), treeData)
	if !ok {
		return nil, UnresolvedComputedName
	}

	bases, reason := jsReceiverOf(subscriptNode.ChildByFieldName("object"), subscriptNode, treeData, currentNamespace, callGraph, depth)
	return qualify(bases, name), reason
}

// jsReceiverOf returns the namespaces of the object of member: the module
// of an import, the class of a value or the type of a literal. It returns
// no namespace for an object that it cannot resolve, and the reason.
func jsReceiverOf(object, member *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, depth int) ([]string, UnresolvedReason) {
	object = callGraph.unwrap(object)
	if object == nil {
		return nil, UnresolvedUnknownReceiver
	}
	if builtin, ok := callGraph.profile.literalReceivers[object.Type()]; ok {
		return []string{builtin}, ""
	}

	switch object.Type() {
	case "call_expression", "new_expression":
		// The value of a call or of an object creation is the base of the
		// member, as in crypto.createHash('md5').digest or new OpenAI().chat
		return jsValueOf(object, treeData, currentNamespace, callGraph, depth+1)
	case "member_expression":
		return resolveJSMemberExpressionWithDepth(object, treeData, currentNamespace, callGraph, depth+1)
	case "subscript_expression":
		return resolveJSSubscript(object, treeData, currentNamespace, callGraph, depth+1)
	case "identifier", "this", "super":
	default:
		return nil, UnresolvedUnknownReceiver
	}

	// An imported module or a variable
	objectName := object.Content(treeData)
	if objectAssignment, objectResolved := searchSymbolInScopeChain(objectName, currentNamespace, callGraph); objectResolved {
		if targets := callGraph.receiverTargets(objectAssignment.Namespace, currentNamespace, member); len(targets) > 0 {
			return namespacesOf(targets), ""
		}
	}

	// The scope chain does not know the object, as a parameter. The name
	// keeps the object as written.
	callGraph.addUnresolved(currentNamespace, member, member, treeData, UnresolvedUnknownReceiver)
	return []string{objectName}, ""
}

// processJSChainRoot processes the root of the member chain of a callee. A
// chained call, as in crypto.createHash('md5').digest(), calls the inner
// function too.
func processJSChainRoot(callee *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) {
	root := callee
	for depth := 0; root != nil && depth <= maxMemberExpressionDepth; depth++ {
		if root.Type() != "member_expression" && root.Type() != "subscript_expression" {
			break
		}
		root = callGraph.unwrap(root.ChildByFieldName("object"))
	}
	if root != callee && root != nil && callGraph.profile.valueRoots[root.Type()] {
		processNode(root, treeData, currentNamespace, callGraph, metadata)
	}
}

// jsValueOf returns the namespaces of the value of a call or an object
// creation: the called function or the created class. It adds no edge. It
// returns no namespace for a value that it cannot resolve, and the reason.
func jsValueOf(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, depth int) ([]string, UnresolvedReason) {
	if depth > maxMemberExpressionDepth {
		callGraph.truncate(TruncationMemberDepth, currentNamespace, node)
		return nil, UnresolvedLimit
	}

	field := "function"
	if node.Type() == "new_expression" {
		field = "constructor"
	}
	target := callGraph.unwrap(node.ChildByFieldName(field))
	if target == nil {
		return nil, UnresolvedUnknownReceiver
	}

	switch target.Type() {
	case "member_expression":
		return resolveJSMemberExpressionWithDepth(target, treeData, currentNamespace, callGraph, depth+1)
	case "subscript_expression":
		return resolveJSSubscript(target, treeData, currentNamespace, callGraph, depth+1)
	case "identifier":
		// An imported name resolves to its module, as OpenAI to openai
		if assignment, found := searchSymbolInScopeChain(target.Content(treeData), currentNamespace, callGraph); found {
			if targets := callGraph.receiverTargets(assignment.Namespace, currentNamespace, target); len(targets) > 0 {
				return namespacesOf(targets), ""
			}
		}
		callee, _ := resolveIdentifier(target.Content(treeData), currentNamespace, callGraph)
		return []string{callee}, ""
	}
	return nil, UnresolvedUnknownReceiver
}

// memberExpressionProcessor gives the property of each value of the object.
// jsCallExpressionProcessor handles a called member.
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

// arrowFunctionProcessor processes an arrow function in the namespace of the
// name it gets, as arrowFunc in const arrowFunc = (x) => x, or else in the
// current namespace. A named arrow function is a node and an assignment, so
// a call of the name resolves to it.
func arrowFunctionProcessor(arrowNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if arrowNode == nil {
		return newProcessorResult()
	}

	results := newProcessorResult()

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
		if parentNode.Type() == "assignment_expression" {
			leftNode := parentNode.ChildByFieldName("left")
			if leftNode != nil && leftNode.Type() == "identifier" {
				functionName = leftNode.Content(treeData)
				break
			}
		}
		parentNode = parentNode.Parent()
	}

	arrowFunctionNamespace := currentNamespace
	if functionName != "" {
		arrowFunctionNamespace = currentNamespace + namespaceSeparator + functionName

		if callGraph.addDefinition(arrowFunctionNamespace, arrowNode) {
			log.Debugf("Register arrow function definition for %s - %s", functionName, arrowFunctionNamespace)
		}
	}

	results.addResults(processFunctionBody(arrowNode, treeData, arrowFunctionNamespace, callGraph, metadata))
	return results
}

// jsNewExpressionProcessor adds the edge of an object creation to the
// class, as TestClass for new TestClass("test") and sqlite3//Database for
// new sqlite3.Database(':memory:'). The value of the creation is the class.
func jsNewExpressionProcessor(newNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if newNode == nil {
		return newProcessorResult()
	}

	result := newProcessorResult()

	constructorNode := newNode.ChildByFieldName("constructor")
	if constructorNode == nil {
		return result
	}

	argumentsNode := newNode.ChildByFieldName("arguments")
	callArguments := []CallArgument{}
	if argumentsNode != nil {
		callArguments = resolveCallArguments(argumentsNode, treeData, currentNamespace, callGraph, metadata)
	}

	var constructors []string
	var reason UnresolvedReason
	unbound := false

	callee := callGraph.unwrap(constructorNode)
	processJSChainRoot(callee, treeData, currentNamespace, callGraph, metadata)

	switch callee.Type() {
	case "member_expression":
		constructors, reason = resolveJSMemberExpression(callee, treeData, currentNamespace, callGraph)
	case "subscript_expression":
		constructors, reason = resolveJSSubscript(callee, treeData, currentNamespace, callGraph, 0)
	case "identifier":
		name, bound := resolveIdentifier(callee.Content(treeData), currentNamespace, callGraph)
		constructors, unbound = []string{name}, !bound
	default:
		processNode(callee, treeData, currentNamespace, callGraph, metadata)
		reason = UnresolvedComputedName
	}

	if len(constructors) == 0 && reason != "" {
		callGraph.addUnresolved(currentNamespace, constructorNode, callee, treeData, reason)
	}

	for _, constructor := range constructors {
		log.Debugf("JS constructor resolved to %s", constructor)

		ref := callGraph.reference(constructor, newNode, callArguments)
		ref.Unbound = unbound
		callGraph.addCall(currentNamespace, nil, nil, ref)

		if classAssignment, classResolved := searchSymbolInScopeChain(constructor, currentNamespace, callGraph); classResolved {
			result.addAssignments(classAssignment)
		}
	}

	return result
}
