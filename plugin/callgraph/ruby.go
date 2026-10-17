package callgraph

import (
	"strings"

	"github.com/safedep/code/core"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/pkg/ts"
	sitter "github.com/smacker/go-tree-sitter"
)

// Ruby calls a method with no receiver and no arguments by its bare name,
// as load_data in data = load_data. So an identifier that no local
// variable and no parameter binds is a call.
var rubyProfile = shapedProfile(languageProfile{
	code:            core.LanguageCodeRuby,
	separators:      []string{"::", "."},
	instanceKeyword: "self",
	builtins:        langcode.Builtins(langcode.Ruby),
	reachable:       map[string]bool{"program": true},
	literals: map[string]bool{
		"character": true, "false": true, "float": true, "integer": true, "string": true, "true": true,
		"nil": true, "simple_symbol": true, "delimited_symbol": true, "heredoc_body": true,
	},
	collections:  map[string]bool{"array": true},
	ignored:      map[string]bool{"comment": true, "alias": true},
	transparent:  map[string]childPosition{"parenthesized_statements": firstChild},
	argumentList: "argument_list",
	strings:      stringShape{interpolations: map[string]bool{"interpolation": true}},
	processors: map[string]nodeProcessor{
		"identifier":         rubyIdentifierProcessor,
		"method_parameters":  rubyParametersProcessor,
		"block_parameters":   rubyParametersProcessor,
		"lambda_parameters":  rubyParametersProcessor,
		"exception_variable": rubyParametersProcessor,
		"super":              rubySuperProcessor,
		"block_argument":     rubyBlockArgumentProcessor,
		"for":                rubyForProcessor,
	},
}, syntaxShapes{
	calls: map[string]callShape{
		"call": {receiver: named("receiver"), name: named("method"), arguments: named("arguments")},
	},
	assignments: map[string]assignmentShape{
		"assignment": {left: named("left"), right: named("right")},
	},
	scopes: map[string]scopeShape{
		"class":            {name: named("name"), body: named("body"), reachable: true, class: true},
		"module":           {name: named("name"), body: named("body"), reachable: true, class: true},
		"method":           {name: named("name"), body: named("body"), params: named("parameters"), reachable: true},
		"singleton_method": {name: named("name"), body: named("body"), params: named("parameters"), reachable: true, prefix: "self."},
	},
	constructorMethods: []string{"new"},
	computedCallees:    map[string]bool{"element_reference": true},
	callables: map[string]int{
		"send": 0, "public_send": 0, "__send__": 0, "method": 0, "instance_method": 0, "const_get": 0,
	},
	staticCallables: map[string]bool{"simple_symbol": true, "string": true},
	destructures:    map[string]bool{"left_assignment_list": true},
})

// rubyKeywords are the identifiers of the grammar that name a value of the
// interpreter and call nothing.
var rubyKeywords = map[string]bool{"__FILE__": true, "__LINE__": true, "__dir__": true, "__method__": true, "__ENCODING__": true}

// rubyIdentifierProcessor gives the binding of a local name. A name that no
// local binds is a call of a method of self or of a top-level method, so
// it adds an edge, and its value is the result of the call.
func rubyIdentifierProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	name := node.Content(treeData)
	if rubyKeywords[name] {
		return newProcessorResult()
	}
	// A local binds a name under a namespace. A builtin method, as exit,
	// binds the bare name, and its name is a call too.
	if assignment, found := searchSymbolInScopeChain(name, currentNamespace, callGraph); found && assignment.Namespace != name && callGraph.Nodes[assignment.Namespace] == nil {
		result := newProcessorResult()
		result.addAssignments(assignment)
		return result
	}
	shaped := shapedProcessors{separators: callGraph.profile.separators}
	callee := shaped.resolvePath(name, currentNamespace, callGraph)
	callGraph.addEdge(currentNamespace, CallCall, node, callee, nil, []CallArgument{})
	callGraph.results[callee] = true
	return shaped.valueResult(callee, callGraph)
}

// rubyParametersProcessor binds each name of a list of parameters, or the
// variable of a rescue, in the current namespace, so a read of it is no
// call. A default value runs there.
func rubyParametersProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	bind := func(name *sitter.Node) {
		if name != nil && name.Type() == "identifier" {
			callGraph.assignmentGraph.addNode(currentNamespace+namespaceSeparator+name.Content(treeData), name)
		}
	}
	for _, param := range ts.NamedChildren(node) {
		switch param.Type() {
		case "identifier":
			bind(param)
		case "destructured_parameter":
			rubyParametersProcessor(param, treeData, currentNamespace, callGraph, metadata)
		default:
			bind(param.ChildByFieldName("name"))
			processNode(param.ChildByFieldName("value"), treeData, currentNamespace, callGraph, metadata)
		}
	}
	return newProcessorResult()
}

// rubyForProcessor binds the variable of a for loop and processes the
// loop.
func rubyForProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if pattern := node.ChildByFieldName("pattern"); pattern != nil {
		if pattern.Type() == "identifier" {
			callGraph.assignmentGraph.addNode(currentNamespace+namespaceSeparator+pattern.Content(treeData), pattern)
		} else {
			rubyParametersProcessor(pattern, treeData, currentNamespace, callGraph, metadata)
		}
	}
	processNode(node.ChildByFieldName("value"), treeData, currentNamespace, callGraph, metadata)
	processNode(node.ChildByFieldName("body"), treeData, currentNamespace, callGraph, metadata)
	return newProcessorResult()
}

// rubySuperProcessor adds the call of super, which runs the method of the
// same name of the base class.
func rubySuperProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	callGraph.addEdge(currentNamespace, CallCall, node, superKeyword+namespaceSeparator+lastOf(currentNamespace), nil, []CallArgument{})
	return newProcessorResult()
}

// rubyBlockArgumentProcessor records the call that a symbol block makes, as
// process in items.map(&:process). It calls the method on each item, whose
// value the graph does not know.
func rubyBlockArgumentProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if symbol := childOfType(node, "simple_symbol"); symbol != nil {
		callGraph.addRecord(currentNamespace, symbol, "", strings.TrimPrefix(symbol.Content(treeData), ":"), treeData, UnresolvedUnknownReceiver)
		return newProcessorResult()
	}
	return processChildren(node, treeData, currentNamespace, callGraph, metadata)
}
