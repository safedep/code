package callgraph

import (
	"fmt"
	"slices"

	"github.com/safedep/code/core"
	"github.com/safedep/dry/log"
	sitter "github.com/smacker/go-tree-sitter"
)

var pythonProfile = languageProfile{
	code:              core.LanguageCodePython,
	separators:        []string{"."},
	instanceKeyword:   "self",
	constructorMethod: "__init__",
	builtins: []string{
		"ArithmeticError", "AssertionError", "AttributeError", "BaseException", "BaseExceptionGroup",
		"BlockingIOError", "BrokenPipeError", "BufferError", "BytesWarning", "ChildProcessError",
		"ConnectionAbortedError", "ConnectionError", "ConnectionRefusedError",
		"ConnectionResetError", "DeprecationWarning", "EOFError", "Ellipsis", "EncodingWarning",
		"EnvironmentError", "Exception", "ExceptionGroup", "False", "FileExistsError",
		"FileNotFoundError", "FloatingPointError", "FutureWarning", "GeneratorExit", "IOError",
		"ImportError", "ImportWarning", "IndentationError", "IndexError", "InterruptedError",
		"IsADirectoryError", "KeyError", "KeyboardInterrupt", "LookupError", "MemoryError",
		"ModuleNotFoundError", "NameError", "None", "NotADirectoryError", "NotImplemented",
		"NotImplementedError", "OSError", "OverflowError", "PendingDeprecationWarning",
		"PermissionError", "ProcessLookupError", "RecursionError", "ReferenceError",
		"ResourceWarning", "RuntimeError", "RuntimeWarning", "StopAsyncIteration", "StopIteration",
		"SyntaxError", "SyntaxWarning", "SystemError", "SystemExit", "TabError", "TimeoutError",
		"True", "TypeError", "UnboundLocalError", "UnicodeDecodeError", "UnicodeEncodeError",
		"UnicodeError", "UnicodeTranslateError", "UnicodeWarning", "UserWarning", "ValueError",
		"Warning", "ZeroDivisionError", "__build_class__", "__debug__", "__doc__", "__import__",
		"__loader__", "__name__", "__package__", "__spec__", "abs", "aiter", "all", "anext", "any",
		"ascii", "bin", "bool", "breakpoint", "bytearray", "bytes", "callable", "chr", "classmethod",
		"compile", "complex", "copyright", "credits", "delattr", "dict", "dir", "divmod",
		"enumerate", "eval", "exec", "exit", "filter", "float", "format", "frozenset", "getattr",
		"globals", "hasattr", "hash", "help", "hex", "id", "input", "int", "isinstance",
		"issubclass", "iter", "len", "license", "list", "locals", "map", "max", "memoryview", "min",
		"next", "object", "oct", "open", "ord", "pow", "print", "property", "quit", "range", "repr",
		"reversed", "round", "set", "setattr", "slice", "sorted", "staticmethod", "str", "sum",
		"super", "tuple", "type", "vars", "zip",
	},
	processors: map[string]nodeProcessor{
		"assignment":          assignmentProcessor,
		"attribute":           attributeProcessor,
		"binary_operator":     binaryOperatorProcessor,
		"call":                callProcessor,
		"class_definition":    classDefinitionProcessor,
		"function_definition": functionDefinitionProcessor,
		"subscript":           skippedProcessor,
	},
	reachable: map[string]bool{
		"module":              true,
		"class_definition":    true,
		"function_definition": true,
	},
	transparent: map[string]bool{"parenthesized_expression": true},
}

// pythonLiteralReceivers map a literal node to the builtin type of its
// value, so that "a,b".split() calls str//split.
var pythonLiteralReceivers = map[string]string{
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
}

// attributeProcessor gives the attribute of each value of its object, as
// xyz//attr for xyz.attr. callProcessor handles a called attribute.
func attributeProcessor(attributeNode *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	if attributeNode == nil {
		return newProcessorResult()
	}

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
		if classAssignmentNode != nil && callGraph.classConstructors[classAssignmentNode.Namespace] {
			result.ImmediateAssignments = append(result.ImmediateAssignments, classAssignmentNode)
			log.Debugf("Class constructed - %s in fncall for %s\n", classAssignmentNode.Namespace, functionName)
		}
	}

	callArguments := []CallArgument{}
	if argumentsNode != nil {
		callArguments = resolveCallArguments(argumentsNode, treeData, currentNamespace, callGraph, metadata)
	}

	addEdge := func(callee string, calleeTreeNode *sitter.Node) {
		callGraph.addEdge(currentNamespace, nil, functionCallNode, callee, calleeTreeNode, callArguments)
		if callGraph.isPythonDynamicImport(callee) && !staticString(firstArgument(argumentsNode)) {
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
		ref := callGraph.reference(functionName, functionCallNode, callArguments)
		ref.Unbound = true
		callGraph.addCall(currentNamespace, nil, nil, ref)
		return result
	case "lambda":
		// The body of an immediately called lambda runs in the caller
		processNode(function, treeData, currentNamespace, callGraph, metadata)
		return result
	case "attribute", "subscript":
		// A chained call, as in PasswordHasher().hash(), calls the inner
		// function too, and the inner function is the base of the name
		if root := callGraph.unwrap(chainRoot(function.ChildByFieldName("object"), "attribute", "object")); root != nil && pythonValueRoots[root.Type()] {
			processNode(root, treeData, currentNamespace, callGraph, metadata)
		}
		callees, reason = pythonValueOf(function, treeData, currentNamespace, callGraph, 0)
	default:
		// The inner call is a call of its own, as getattr in getattr(o, n)()
		processNode(function, treeData, currentNamespace, callGraph, metadata)
		reason = UnresolvedComputedName
	}

	for _, callee := range callees {
		addEdge(callee, nil)
		markClassAssignment(callGraph.assignmentGraph.Assignments[callee])
	}
	if len(callees) == 0 && reason != "" {
		callGraph.addUnresolved(currentNamespace, functionCallNode, function, treeData, reason)
	}

	return result
}

// pythonValueRoots are the Python nodes at the root of a member chain that
// hold calls of their own, as Config() in Config().load().
var pythonValueRoots = map[string]bool{
	"call":       true,
	"list":       true,
	"dictionary": true,
	"set":        true,
	"tuple":      true,
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
	if builtin, ok := pythonLiteralReceivers[node.Type()]; ok {
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
