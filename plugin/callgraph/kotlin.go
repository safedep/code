package callgraph

import (
	"github.com/safedep/code/core"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/pkg/ts"
	sitter "github.com/smacker/go-tree-sitter"
)

// The Kotlin grammar names no fields, so each shape names the types of the
// children. The functions of a companion object are members of the class,
// as a call names them. A getter, a setter, an init block and a property
// initializer run on an instance, so their calls belong to the class.
var kotlinProfile = shapedProfile(languageProfile{
	code:            core.LanguageCodeKotlin,
	separators:      []string{"."},
	instanceKeyword: "this",
	builtins:        langcode.Builtins(langcode.Kotlin),
	literals: map[string]bool{
		"integer_literal": true, "long_literal": true, "hex_literal": true, "bin_literal": true,
		"unsigned_literal": true, "real_literal": true, "boolean_literal": true,
		"character_literal": true, "string_literal": true,
	},
	collections: map[string]bool{"collection_literal": true},
	ignored: map[string]bool{
		"line_comment": true, "multiline_comment": true, "shebang_line": true,
		"package_header": true, "import_list": true, "file_annotation": true,
	},
	transparent: map[string]childPosition{
		"parenthesized_expression": firstChild,
		"postfix_expression":       firstChild,
		"value_argument":           lastChild,
	},
	strings:      stringShape{interpolations: map[string]bool{"interpolated_expression": true}},
	argumentList: "value_arguments",
	processors: map[string]nodeProcessor{
		"simple_identifier":           identifierProcessor,
		"object_literal":              kotlinObjectLiteralProcessor,
		"companion_object":            kotlinCompanionProcessor,
		"secondary_constructor":       kotlinConstructorProcessor,
		"constructor_delegation_call": kotlinDelegationCallProcessor,
		"callable_reference":          skippedProcessor,
		"modifiers":                   skippedProcessor,
		"annotation":                  skippedProcessor,
		"user_type":                   skippedProcessor,
		"nullable_type":               skippedProcessor,
		"function_type":               skippedProcessor,
		"type_arguments":              skippedProcessor,
		"type_parameters":             skippedProcessor,
		"type_constraints":            skippedProcessor,
		"type_alias":                  skippedProcessor,
		"parameter":                   kotlinParameterProcessor,
		"class_parameter":             kotlinParameterProcessor,
		"constructor_invocation":      kotlinConstructorInvocationProcessor,
	},
}, syntaxShapes{
	calls: map[string]callShape{
		"call_expression": {function: typed(""), arguments: typed("call_suffix", "value_arguments")},
		// An infix call, as a to b, calls the function of its name on the
		// left side.
		"infix_expression": {receiver: childAt(0), name: childAt(1)},
	},
	members: map[string]memberShape{
		"navigation_expression": {object: typed(""), name: typed("navigation_suffix", "simple_identifier")},
	},
	assignments: map[string]assignmentShape{
		"property_declaration": {left: typed("variable_declaration", "simple_identifier")},
		"assignment":           {left: typed("directly_assignable_expression")},
	},
	scopes: map[string]scopeShape{
		"function_declaration": {name: typed("simple_identifier"), body: typed("function_body"), reachable: true, header: true, bodyRequired: true},
		"class_declaration":    {name: typed("type_identifier"), body: typed("class_body|enum_class_body"), reachable: true, class: true, header: true},
		"object_declaration":   {name: typed("type_identifier"), body: typed("class_body"), reachable: true, class: true, header: true},
		"enum_entry":           {name: typed("simple_identifier"), body: typed("class_body"), class: true, bodyRequired: true},
	},
	values: map[string]childRef{"as_expression": typed("")},
	computedCallees: map[string]bool{
		"indexing_expression": true, "lambda_literal": true, "anonymous_function": true,
	},
	superKeywords: []string{"super"},
	classCalls:    true,
	trailing:      typed("call_suffix", "annotated_lambda"),
})

// kotlinConstructorName names a secondary constructor, as Java names a
// constructor.
const kotlinConstructorName = javaConstructorName

// kotlinConstructorProcessor processes a secondary constructor as a method
// of its class with the name <init>, as a Java constructor. The class calls
// it.
func kotlinConstructorProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if node == nil {
		return newProcessorResult()
	}
	namespace := currentNamespace + namespaceSeparator + kotlinConstructorName
	callGraph.addSyntheticEdge(currentNamespace, namespace, node)
	callGraph.addDefinition(namespace, node)
	processChildren(node, treeData, namespace, callGraph, metadata)
	return newProcessorResult()
}

// kotlinDelegationCallProcessor adds the call of super(...) in a secondary
// constructor, which runs the constructor of the base class. this(...) runs
// another constructor of the class, which has the same key.
func kotlinDelegationCallProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if node == nil {
		return newProcessorResult()
	}
	args := resolveCallArguments(childOfType(node, "value_arguments"), treeData, currentNamespace, callGraph, metadata)
	if node.ChildCount() > 0 && node.Child(0).Type() == superKeyword {
		ref := callGraph.reference(superKeyword, CallCall, node.Child(0), args)
		ref.Unbound = true
		callGraph.addCall(currentNamespace, nil, ref)
	}
	return newProcessorResult()
}

// kotlinParameterProcessor names a parameter in the scope of its function,
// or a property of the primary constructor in the scope of its class, so a
// call on it does not bind a name of an outer scope. Its default value
// runs in the scope.
func kotlinParameterProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if node == nil {
		return newProcessorResult()
	}
	name := childOfType(node, "simple_identifier")
	if name != nil {
		callGraph.assignmentGraph.addNode(currentNamespace+namespaceSeparator+name.Content(treeData), name)
	}
	for _, child := range ts.NamedChildren(node) {
		if name == nil || !sameNode(child, name) {
			processNode(child, treeData, currentNamespace, callGraph, metadata)
		}
	}
	return newProcessorResult()
}

// kotlinConstructorInvocationProcessor adds the call of the constructor of
// a base class, as Base(x) in class A : Base(x). The class calls it, as
// each instance of the class runs it.
func kotlinConstructorInvocationProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if node == nil {
		return newProcessorResult()
	}
	args := resolveCallArguments(childOfType(node, "value_arguments"), treeData, currentNamespace, callGraph, metadata)
	typeNode := childOfType(node, "user_type")
	if typeNode == nil {
		return newProcessorResult()
	}
	shaped := shapedProcessors{separators: callGraph.profile.separators}
	if base := shaped.resolvePath(typeNode.Content(treeData), currentNamespace, callGraph); base != "" {
		callGraph.addEdge(currentNamespace, CallNew, typeNode, base, nil, args)
	}
	return newProcessorResult()
}

// kotlinStaticInit names the code of a companion object that runs when the
// class loads, as Java names a static initializer.
const kotlinStaticInit = "<clinit>"

// kotlinCompanionProcessor processes a companion object. Its functions are
// members of the class. Its property initializers and init blocks run once
// when the class loads, so they are the static initializer <clinit> of the
// class, as in Java.
func kotlinCompanionProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	body := childOfType(node, "class_body")
	if body == nil {
		return newProcessorResult()
	}
	init := currentNamespace + namespaceSeparator + kotlinStaticInit
	for _, member := range ts.NamedChildren(body) {
		if member.Type() == "function_declaration" {
			processNode(member, treeData, currentNamespace, callGraph, metadata)
			continue
		}
		if callGraph.Nodes[init] == nil {
			callGraph.addDefinition(init, node)
		}
		processNode(member, treeData, init, callGraph, metadata)
	}
	return newProcessorResult()
}

// kotlinObjectLiteralProcessor processes an object expression, as
// object : Runnable { override fun run() {} }. The graph has no name for
// its class, so the body of each method runs in the scope that makes the
// object, as an anonymous class of Java.
func kotlinObjectLiteralProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if node == nil {
		return newProcessorResult()
	}
	for _, child := range ts.NamedChildren(node) {
		switch child.Type() {
		case "delegation_specifier":
			processNode(child, treeData, currentNamespace, callGraph, metadata)
		case "class_body":
			for _, member := range ts.NamedChildren(child) {
				if member.Type() == "function_declaration" {
					processNode(childOfType(member, "function_body"), treeData, currentNamespace, callGraph, metadata)
					continue
				}
				processNode(member, treeData, currentNamespace, callGraph, metadata)
			}
		}
	}
	return newProcessorResult()
}
