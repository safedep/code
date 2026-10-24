package callgraph

import (
	"fmt"
	"slices"
	"strings"

	"github.com/safedep/code/core"
	"github.com/safedep/code/langcode"
	"github.com/safedep/dry/log"
	sitter "github.com/smacker/go-tree-sitter"
)

var pythonProfile = languageProfile{
	code:              core.LanguageCodePython,
	separators:        []string{"."},
	instanceKeyword:   "self",
	constructorMethod: "__init__",
	importBindsRoot:   true,
	builtins:          langcode.Builtins(langcode.Python),
	processors: map[string]nodeProcessor{
		"assignment":          assignmentProcessor,
		"attribute":           attributeProcessor,
		"binary_operator":     binaryOperatorProcessor,
		"call":                callProcessor,
		"class_definition":    classDefinitionProcessor,
		"function_definition": functionDefinitionProcessor,
		"subscript":           skipResultsProcessor,
	},
	literals: map[string]bool{
		"false": true, "float": true, "integer": true, "none": true, "string": true, "true": true,
	},
	collections: map[string]bool{"dictionary": true, "list": true, "set": true, "tuple": true},
	ignored:     map[string]bool{"comment": true, "import_from_statement": true, "import_statement": true},
	reachable: map[string]bool{
		"module":              true,
		"class_definition":    true,
		"function_definition": true,
	},
	transparent: map[string]childPosition{"parenthesized_expression": firstChild},
	members: map[string]memberShape{
		"attribute": {object: named("object"), name: named("attribute")},
		"subscript": {object: named("value"), name: named("subscript"), index: true},
	},
	memberAssignees: map[string]bool{"attribute": true},
	strings: stringShape{
		literals:       map[string]bool{"string": true},
		fragments:      map[string]bool{"string_content": true},
		quotes:         map[string]bool{"string_start": true, "string_end": true},
		interpolations: map[string]bool{"interpolation": true},
	},
	argumentList:     "argument_list",
	functionLiterals: map[string]bool{"lambda": true},
	valueRoots: map[string]bool{
		"call":       true,
		"list":       true,
		"dictionary": true,
		"set":        true,
		"tuple":      true,
	},
	literalReceivers: map[string]string{
		"string":                   "str",
		"concatenated_string":      "str",
		"integer":                  "int",
		"float":                    "float",
		"list":                     "list",
		"list_comprehension":       "list",
		"dictionary":               "dict",
		"dictionary_comprehension": "dict",
		"set":                      "set",
		"set_comprehension":        "set",
		"tuple":                    "tuple",
	},
}

// attributeProcessor gives the attribute of each value of its object, as
// xyz//attr for xyz.attr. callProcessor handles a called attribute.
func attributeProcessor(attributeNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if attributeNode == nil {
		return newProcessorResult()
	}
	processPythonChainRoot(attributeNode, treeData, currentNamespace, callGraph, metadata)

	objectSymbol, attributeQualifierNamespace, err := dissectAttributeQualifier(attributeNode, treeData, currentNamespace, callGraph, metadata)
	if err != nil {
		return newProcessorResult()
	}

	targetObject, objectResolved := searchSymbolInScopeChain(objectSymbol, currentNamespace, callGraph)
	if !objectResolved {
		log.Debugf("Object not found in namespace for attribute - %s (Obj - %s, Attr - %s)", attributeNode.Content(treeData), objectSymbol, attributeQualifierNamespace)
		return newProcessorResult()
	}

	result := newProcessorResult()
	for _, resolvedObject := range callGraph.assignmentGraph.resolve(targetObject.Namespace) {
		finalAttributeNamespace := resolvedObject.Namespace + namespaceSeparator + attributeQualifierNamespace
		result.addAssignments(callGraph.assignmentGraph.addNode(finalAttributeNamespace, attributeNode))
	}

	return result
}

// processPythonChainRoot processes the root of the attribute chain of a
// member, as the call in PasswordHasher().hash or requests.get(url).text.
// The root runs, so a call in it is a call of its own. A name root holds no
// call.
func processPythonChainRoot(member *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) {
	if member.Type() != "attribute" {
		return
	}
	root := callGraph.unwrap(chainRoot(member.ChildByFieldName("object"), "attribute", "object"))
	if root != nil && root.Type() != "identifier" {
		processNode(root, treeData, currentNamespace, callGraph, metadata)
	}
}

// binaryOperatorProcessor gives the values of both operands.
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
	results.addResults(
		processNode(leftNode, treeData, currentNamespace, callGraph, metadata),
		processNode(rightNode, treeData, currentNamespace, callGraph, metadata),
	)
	return results
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

// functionCallProcessor adds the edges of a Python call. The value of a call
// to a class is the class, so an assignment of the call holds an instance.
func functionCallProcessor(functionCallNode *sitter.Node, argumentsNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	result := newProcessorResult()

	function := callGraph.unwrap(functionCallNode)
	functionName := function.Content(treeData)

	markClassAssignment := func(classAssignmentNode *assignmentNode) {
		switch {
		case classAssignmentNode == nil:
		case callGraph.classConstructors[classAssignmentNode.Namespace]:
			result.ImmediateAssignments = append(result.ImmediateAssignments, classAssignmentNode)
			log.Debugf("Class constructed - %s in fncall for %s\n", classAssignmentNode.Namespace, functionName)
		case callGraph.imported(classAssignmentNode.Namespace) && !callGraph.isPythonDynamicImport(classAssignmentNode.Namespace):
			// The file cannot tell an imported class from an imported
			// function. The value of the call is the callee, as for new X()
			// in JavaScript, so p.parse() in p = Parser() keeps the import
			// and the link step can find the class and its method. A
			// dynamic import gives a module that the call computes.
			result.ImmediateAssignments = append(result.ImmediateAssignments, classAssignmentNode)
		}
	}

	callArguments := []CallArgument{}
	if argumentsNode != nil {
		callArguments = resolveCallArguments(argumentsNode, treeData, currentNamespace, callGraph, metadata)
	}

	addEdge := func(callee string, calleeTreeNode *sitter.Node) {
		callGraph.addEdge(currentNamespace, CallCall, functionCallNode, callee, calleeTreeNode, callArguments)
		if callGraph.isPythonDynamicImport(callee) && !callGraph.profile.staticString(firstArgument(argumentsNode)) {
			callGraph.addUnresolved(currentNamespace, functionCallNode.Parent(), function, treeData, UnresolvedDynamicImport)
		}
	}

	var callees []string
	var reason UnresolvedReason

	switch function.Type() {
	case "identifier":
		if assignment, found := searchSymbolInScopeChain(functionName, currentNamespace, callGraph); found {
			log.Debugf("Call %s searched (direct) & resolved to %s\n", functionName, assignment.Namespace)
			addEdge(assignment.Namespace, nil)
			markClassAssignment(assignment)
			return result
		}
		callGraph.addUnboundCall(currentNamespace, functionName, CallCall, functionCallNode, callArguments)
		return result
	case "attribute", "subscript":
		processPythonChainRoot(function, treeData, currentNamespace, callGraph, metadata)
		callees, reason = pythonValueOf(function, treeData, currentNamespace, callGraph, 0)
	default:
		if callGraph.profile.functionLiterals[function.Type()] {
			// The body of an immediately called lambda runs in the caller
			processFunctionBody(function, treeData, currentNamespace, callGraph, metadata)
			return result
		}
		// The inner call is a call of its own, as getattr in getattr(o, n)()
		processNode(function, treeData, currentNamespace, callGraph, metadata)
		if object, name, ok := pythonGetattrLiteral(function, treeData, callGraph); ok {
			// getattr(o, "x")() reads the member x of o, as o.x() does.
			callees, reason = pythonValueOf(object, treeData, currentNamespace, callGraph, 0)
			callees = qualify(callees, name)
			if len(callees) == 0 && reason != "" {
				callGraph.addRecord(currentNamespace, functionCallNode, object.Content(treeData), name, treeData, reason)
				return result
			}
			break
		}
		// A call of the value of a call, as click.option(...)(f) in a
		// decorator factory, runs code that the inner call gives. The
		// graph reaches that code through the inner call and the
		// reference edges from its callee, so the outer call is no record
		// of its own. A reflective inner call, as getattr(o, n), gives a
		// value that no edge names, so it keeps the record. Any other
		// callee computes its name.
		if function.Type() == "call" && !pythonReflective(function, treeData) {
			return result
		}
		reason = UnresolvedComputedName
	}

	for _, callee := range callees {
		addEdge(callee, nil)
		if callGraph.imported(callee) {
			markClassAssignment(callGraph.assignmentGraph.addNode(callee, nil))
		} else {
			markClassAssignment(callGraph.assignmentGraph.Assignments[callee])
		}
	}
	if len(callees) == 0 && reason != "" {
		callGraph.addUnresolved(currentNamespace, functionCallNode, function, treeData, reason)
	}

	return result
}

// pythonGetattrLiteral returns the object and the member name of a call of
// getattr whose name is a string literal, as o and x in getattr(o, "x").
func pythonGetattrLiteral(call *sitter.Node, treeData []byte, callGraph *CallGraph) (*sitter.Node, string, bool) {
	if call.Type() != "call" || call.ChildByFieldName("function").Content(treeData) != "getattr" {
		return nil, "", false
	}
	arguments := call.ChildByFieldName("arguments")
	if arguments == nil || arguments.NamedChildCount() < 2 {
		return nil, "", false
	}
	name, ok := callGraph.profile.memberName(arguments.NamedChild(1), treeData)
	if !ok {
		return nil, "", false
	}
	return arguments.NamedChild(0), name, true
}

// imported reports whether every value of the namespace is an import of
// the file or a name under one, as requests//get.
func (cg *CallGraph) imported(namespace string) bool {
	values := cg.assignmentGraph.resolve(namespace)
	if len(values) == 0 {
		return false
	}
	return !slices.ContainsFunc(values, func(value *assignmentNode) bool {
		return !slices.ContainsFunc(cg.Imports, func(imp Import) bool {
			return imp.Namespace != "" && (value.Namespace == imp.Namespace || strings.HasPrefix(value.Namespace, imp.Namespace+namespaceSeparator))
		})
	})
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
	if pythonDynamicImports[namespace] {
		return true
	}
	return slices.ContainsFunc(cg.assignmentGraph.resolve(namespace), func(target *assignmentNode) bool {
		return pythonDynamicImports[target.Namespace]
	})
}

// pythonValueOf returns the namespaces that a Python expression names,
// through the scope chain and the assignment graph: the module of an
// import, the class of a value, or the type of a literal. The value of a
// call is its function. It adds no edge. It returns no namespace for an
// expression that it cannot resolve, and the reason.
func pythonValueOf(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, depth int) ([]string, UnresolvedReason) {
	if depth > maxAttributeDepth {
		callGraph.truncate(TruncationAttributeDepth, currentNamespace, node)
		return nil, UnresolvedLimit
	}

	node = callGraph.unwrap(node)
	if node == nil {
		return nil, UnresolvedUnknownReceiver
	}
	if builtin, ok := callGraph.profile.literalReceivers[node.Type()]; ok {
		return []string{builtin}, ""
	}

	switch node.Type() {
	case "identifier":
		assignment, found := searchSymbolInScopeChain(node.Content(treeData), currentNamespace, callGraph)
		if !found {
			return nil, UnresolvedUnknownReceiver
		}
		return namespacesOf(callGraph.receiverTargets(assignment.Namespace, currentNamespace, node)), ""
	case "attribute":
		attribute := node.ChildByFieldName("attribute")
		if attribute == nil {
			return nil, UnresolvedUnknownReceiver
		}
		bases, reason := pythonValueOf(node.ChildByFieldName("object"), treeData, currentNamespace, callGraph, depth+1)
		return qualify(bases, attribute.Content(treeData)), reason
	case "call":
		return pythonValueOf(node.ChildByFieldName("function"), treeData, currentNamespace, callGraph, depth+1)
	case "subscript":
		return nil, UnresolvedComputedName
	}
	return nil, UnresolvedUnknownReceiver
}

// dissectAttributeQualifier splits an attribute into the name of its object
// and the namespace of the rest, as xyz and attr//subattr for
// xyz.attr.subattr.
func dissectAttributeQualifier(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) (string, string, error) {
	return dissectAttributeQualifierWithDepth(node, treeData, currentNamespace, callGraph, metadata, 0)
}

func dissectAttributeQualifierWithDepth(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata, depth int) (string, string, error) {
	if node == nil {
		return "", "", fmt.Errorf("fnAttributeResolver - node is nil")
	}

	if depth > maxAttributeDepth {
		callGraph.truncate(TruncationAttributeDepth, currentNamespace, node)
		return "", "", fmt.Errorf("attribute deeper than %d levels", maxAttributeDepth)
	}

	if node.Type() == "identifier" {
		return node.Content(treeData), "", nil
	}

	// An attribute of a call, as in someClass().attr, has no object name and
	// gives no value.
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

// pythonReflectiveCallees name the calls whose value the graph cannot
// name: a lookup of an attribute or a module by a name, and a run of code
// in a string.
var pythonReflectiveCallees = map[string]bool{
	"getattr": true, "__import__": true, "importlib.import_module": true, "import_module": true,
	"eval": true, "exec": true, "compile": true,
}

// pythonReflective reports whether a call reads its value by reflection.
func pythonReflective(call *sitter.Node, treeData []byte) bool {
	callee := call.ChildByFieldName("function")
	return callee != nil && pythonReflectiveCallees[callee.Content(treeData)]
}
