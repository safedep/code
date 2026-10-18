package callgraph

import (
	"github.com/safedep/code/core"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/pkg/ts"
	sitter "github.com/smacker/go-tree-sitter"
)

// A namespace of C# is no namespace of the graph: the package of the index
// holds it, so a class of a namespace has the key of a top-level class, as
// in Java. A static constructor and the static fields of a class run when
// the class loads, so they are its static initializer <clinit>, as in Java.
var csharpProfile = shapedProfile(languageProfile{
	code:            core.LanguageCodeCSharp,
	separators:      []string{"."},
	instanceKeyword: "this",
	builtins:        langcode.Builtins(langcode.CSharp),
	literals: map[string]bool{
		"character_literal": true, "raw_string_literal": true, "string_literal": true,
		"verbatim_string_literal": true, "integer_literal": true, "real_literal": true,
		"null_literal": true, "boolean_literal": true, "true": true, "false": true,
	},
	ignored:      map[string]bool{"comment": true},
	transparent:  map[string]childPosition{"parenthesized_expression": firstChild, "await_expression": firstChild},
	argumentList: "argument_list",
	processors: map[string]nodeProcessor{
		"enum_declaration":                    skipResultsProcessor,
		"as_expression":                       skippedProcessor,
		"predefined_type":                     skippedProcessor,
		"readonly":                            skippedProcessor,
		"namespace_declaration":               bodyProcessor,
		"file_scoped_namespace_declaration":   skippedProcessor,
		"constructor_declaration":             csharpConstructorProcessor,
		"constructor_initializer":             csharpConstructorInitializerProcessor,
		"field_declaration":                   csharpMemberProcessor,
		"property_declaration":                csharpMemberProcessor,
		"implicit_object_creation_expression": csharpImplicitCreationProcessor,
		"parameter":                           parameterProcessor,
	},
}, syntaxShapes{
	calls: map[string]callShape{
		"invocation_expression": {function: named("function"), arguments: named("arguments")},
	},
	creations: map[string]creationShape{
		"object_creation_expression": {typeField: named("type"), arguments: named("arguments")},
	},
	members: map[string]memberShape{
		"member_access_expression": {object: named("expression"), name: named("name")},
	},
	assignments: map[string]assignmentShape{
		"variable_declarator":   {left: named("name")},
		"assignment_expression": {left: named("left"), right: named("right")},
	},
	scopes: map[string]scopeShape{
		"class_declaration":        {name: named("name"), body: named("body"), reachable: true, class: true},
		"struct_declaration":       {name: named("name"), body: named("body"), reachable: true, class: true},
		"record_declaration":       {name: named("name"), body: named("body"), reachable: true, class: true},
		"interface_declaration":    {name: named("name"), body: named("body"), class: true},
		"method_declaration":       {name: named("name"), body: named("body"), params: named("parameters"), reachable: true, bodyRequired: true},
		"local_function_statement": {name: named("name"), body: named("body"), params: named("parameters"), reachable: true},
	},
	skipped:         []string{"using_directive", "attribute_list", "attribute"},
	values:          map[string]childRef{"cast_expression": named("value")},
	computedCallees: map[string]bool{"element_access_expression": true},
	superKeywords:   []string{"base"},
})

// csharpStaticInit names the static initializer of a C# class, as Java
// names it.
const csharpStaticInit = "<clinit>"

// csharpStatic reports whether a member has the modifier static.
func csharpStatic(node *sitter.Node) bool {
	for _, child := range ts.NamedChildren(node) {
		if child.Type() == "modifier" && child.ChildCount() > 0 && child.Child(0).Type() == "static" {
			return true
		}
	}
	return false
}

// csharpConstructorProcessor processes a constructor as a method of its
// class with the name of the class. A static constructor is the static
// initializer of the class.
func csharpConstructorProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	name := node.ChildByFieldName("name")
	if name == nil {
		return newProcessorResult()
	}
	namespace := currentNamespace + namespaceSeparator + name.Content(treeData)
	if csharpStatic(node) {
		namespace = currentNamespace + namespaceSeparator + csharpStaticInit
	}
	callGraph.addDefinition(namespace, node)
	body := node.ChildByFieldName("body")
	for _, child := range ts.NamedChildren(node) {
		if !sameNode(child, name) && (body == nil || !sameNode(child, body)) {
			processNode(child, treeData, namespace, callGraph, metadata)
		}
	}
	processChildren(body, treeData, namespace, callGraph, metadata)
	return newProcessorResult()
}

// csharpConstructorInitializerProcessor adds the call of base(...) in a
// constructor, which runs the constructor of the base class. this(...)
// runs another constructor of the class.
func csharpConstructorInitializerProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	args := resolveCallArguments(childOfType(node, "argument_list"), treeData, currentNamespace, callGraph, metadata)
	if hasChildOfType(node, "base") {
		callGraph.addUnboundCall(currentNamespace, superKeyword, CallCall, node, args)
	}
	return newProcessorResult()
}

// csharpMemberProcessor processes a field or a property. The initializer
// and the accessors of a static member run in the static initializer of
// the class.
func csharpMemberProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if csharpStatic(node) {
		init := currentNamespace + namespaceSeparator + csharpStaticInit
		if callGraph.Nodes[init] == nil {
			callGraph.addDefinition(init, node)
		}
		currentNamespace = init
	}
	return processChildren(node, treeData, currentNamespace, callGraph, metadata)
}

// csharpImplicitCreationProcessor adds the edge of a creation with no type,
// as new() in Store s = new(). The declaration gives the type. A creation
// whose type the code does not name is a record, because it can make an
// instance of any class.
func csharpImplicitCreationProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	args := resolveCallArguments(childOfType(node, "argument_list"), treeData, currentNamespace, callGraph, metadata)
	typ := csharpDeclaredType(node)
	if typ == nil {
		callGraph.addUnresolved(currentNamespace, node, nil, treeData, UnresolvedComputedName)
		return newProcessorResult()
	}
	shaped := shapedProcessors{separators: callGraph.profile.separators}
	created := shaped.resolvePath(typ.Content(treeData), currentNamespace, callGraph)
	if created == "" {
		return newProcessorResult()
	}
	callGraph.addEdge(currentNamespace, CallNew, node, created, nil, args)
	return shaped.valueResult(created, callGraph)
}

// csharpDeclaredType returns the type of the declaration that a creation
// initializes: a variable, a field or a property.
func csharpDeclaredType(node *sitter.Node) *sitter.Node {
	parent := node.Parent()
	if parent == nil {
		return nil
	}
	switch parent.Type() {
	case "variable_declarator":
		if declaration := parent.Parent(); declaration != nil && declaration.Type() == "variable_declaration" {
			if typ := declaration.ChildByFieldName("type"); typ != nil && typ.Type() != "implicit_type" {
				return typ
			}
		}
	case "property_declaration":
		return parent.ChildByFieldName("type")
	}
	return nil
}

func hasChildOfType(node *sitter.Node, nodeType string) bool {
	for i := 0; i < int(node.ChildCount()); i++ {
		if node.Child(i).Type() == nodeType {
			return true
		}
	}
	return false
}
