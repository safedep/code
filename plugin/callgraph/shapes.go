package callgraph

import (
	"regexp"
	"slices"
	"strings"

	"github.com/safedep/code/core"
	sitter "github.com/smacker/go-tree-sitter"
)

// syntaxShapes tells where the call, member, assignment and scope nodes of a
// grammar keep their parts. registerShapedLanguage makes the node processors
// of a language from it, so a grammar with named fields needs no processor
// code of its own.
type syntaxShapes struct {
	calls       map[string]callShape
	creations   map[string]creationShape
	members     map[string]memberShape
	assignments map[string]assignmentShape
	scopes      map[string]scopeShape

	// skipped are node types with no calls below them, such as imports.
	skipped []string

	// constructorMethods are methods that return a new instance of their
	// receiver, as in OpenAI::Client.new. A variable that gets the result is
	// an instance of the receiver.
	constructorMethods []string
}

// callShape names the fields of a call. A call has either a function field,
// as in client.Send(x), or a receiver field and a name field, as in Ruby.
type callShape struct {
	function  string
	receiver  string
	name      string
	arguments string
}

// creationShape names the fields of an object creation, as in new Client(x).
// An empty type field means the first named child.
type creationShape struct {
	typeField string
	arguments string
}

// memberShape names the fields of a member access whose object can be a
// call, as in client.chat().create.
type memberShape struct {
	object string
	name   string
}

// assignmentShape names the fields of an assignment. An empty right field
// means the last named child.
type assignmentShape struct {
	left  string
	right string
}

// scopeShape names the fields of a declaration with its own namespace, such
// as a class or a function. A reachable scope is a start node of the DFS.
type scopeShape struct {
	name      string
	body      string
	reachable bool
}

var assignableNamePattern = regexp.MustCompile(`^[$@]*[A-Za-z_][A-Za-z0-9_]*$`)

type shapedProcessors struct {
	shapes syntaxShapes
	code   core.LanguageCode
}

func registerShapedLanguage(code core.LanguageCode, shapes syntaxShapes) {
	p := &shapedProcessors{shapes: shapes, code: code}

	processors := map[string]nodeProcessor{}
	for nodeType := range shapes.calls {
		processors[nodeType] = p.call
	}
	for nodeType := range shapes.creations {
		processors[nodeType] = p.creation
	}
	for nodeType := range shapes.assignments {
		processors[nodeType] = p.assignment
	}
	for nodeType, scope := range shapes.scopes {
		processors[nodeType] = p.scope
		if scope.reachable {
			dfsSourceNodeTypes[nodeType] = true
		}
	}
	for _, nodeType := range shapes.skipped {
		processors[nodeType] = skippedProcessor
	}

	registerLanguageProcessors(code, processors)
}

// call adds an edge from the current namespace to the callee. Its result is
// the value of the call: the receiver for a constructor method, or else the
// callee.
func (p *shapedProcessors) call(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	shape := p.shapes.calls[node.Type()]

	var callee string
	var identifier *sitter.Node
	handled := []*sitter.Node{}

	if function := fieldChild(node, shape.function); function != nil {
		callee = p.resolve(function, treeData, currentNamespace, callGraph, metadata)
		identifier = function
		handled = append(handled, function)
	} else if name := fieldChild(node, shape.name); name != nil {
		if receiver := fieldChild(node, shape.receiver); receiver != nil {
			if base := p.resolve(receiver, treeData, currentNamespace, callGraph, metadata); base != "" {
				callee = base + namespaceSeparator + p.qualifiedName(name.Content(treeData))
			}
			handled = append(handled, receiver)
		} else {
			callee = p.resolvePath(name.Content(treeData), currentNamespace, callGraph)
		}
		identifier = name
		handled = append(handled, name)
	}

	arguments := []CallArgument{}
	if argumentsNode := fieldChild(node, shape.arguments); argumentsNode != nil {
		arguments = resolveCallArguments(argumentsNode, treeData, currentNamespace, callGraph, metadata)
		handled = append(handled, argumentsNode)
	}

	// Blocks and other children can hold calls, as in Ruby list.each do ... end
	p.processUnhandledChildren(node, handled, treeData, currentNamespace, callGraph, metadata)

	if callee == "" {
		return newProcessorResult()
	}

	callGraph.addEdge(currentNamespace, nil, identifier, callee, nil, arguments)
	return p.valueResult(p.instanceOf(callee), callGraph)
}

// creation adds an edge to the created type. Its result is the type.
func (p *shapedProcessors) creation(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	shape := p.shapes.creations[node.Type()]

	typeNode := fieldChild(node, shape.typeField)
	if shape.typeField == "" && node.NamedChildCount() > 0 {
		typeNode = node.NamedChild(0)
	}
	if typeNode == nil {
		return emptyProcessor(node, treeData, currentNamespace, callGraph, metadata)
	}

	argumentsNode := fieldChild(node, shape.arguments)
	if shape.arguments == "" {
		argumentsNode = childOfType(node, "arguments", "argument_list")
	}

	arguments := []CallArgument{}
	if argumentsNode != nil {
		arguments = resolveCallArguments(argumentsNode, treeData, currentNamespace, callGraph, metadata)
	}

	created := p.resolve(typeNode, treeData, currentNamespace, callGraph, metadata)
	if created == "" {
		return newProcessorResult()
	}

	callGraph.addEdge(currentNamespace, nil, typeNode, created, nil, arguments)
	return p.valueResult(created, callGraph)
}

// assignment assigns the values of the right side to a variable in the
// current namespace. It ignores a left side that is not a plain name, as in
// self.client = x.
func (p *shapedProcessors) assignment(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	shape := p.shapes.assignments[node.Type()]

	left := fieldChild(node, shape.left)
	right := fieldChild(node, shape.right)
	if count := node.NamedChildCount(); shape.right == "" && count > 0 {
		right = node.NamedChild(int(count) - 1)
	}

	if right == nil || (left != nil && right.StartByte() == left.StartByte()) {
		return newProcessorResult()
	}

	result := processNode(right, treeData, currentNamespace, callGraph, metadata)
	if left == nil || !assignableNamePattern.MatchString(left.Content(treeData)) {
		return newProcessorResult()
	}

	variable := currentNamespace + namespaceSeparator + left.Content(treeData)
	for _, value := range result.ImmediateAssignments {
		callGraph.assignmentGraph.addAssignment(variable, left, value.Namespace, value.TreeNode)
	}

	return newProcessorResult()
}

// scope processes the body of a declaration in the namespace of its name.
func (p *shapedProcessors) scope(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	shape := p.shapes.scopes[node.Type()]

	name := fieldChild(node, shape.name)
	if name == nil {
		return emptyProcessor(node, treeData, currentNamespace, callGraph, metadata)
	}

	scopeNamespace := currentNamespace + namespaceSeparator + withoutTypeArguments(name.Content(treeData))
	callGraph.addNode(scopeNamespace, node)

	if body := fieldChild(node, shape.body); body != nil {
		processChildren(body, treeData, scopeNamespace, callGraph, metadata)
	}

	return newProcessorResult()
}

// resolve returns the namespace of an expression in a callee, receiver or
// type position. A call in the expression adds its own edge, and its value
// becomes the base of the namespace, as in client.chat().create.
func (p *shapedProcessors) resolve(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) string {
	nodeType := node.Type()

	if _, isCall := p.shapes.calls[nodeType]; isCall {
		return firstValue(processNode(node, treeData, currentNamespace, callGraph, metadata))
	}
	if _, isCreation := p.shapes.creations[nodeType]; isCreation {
		return firstValue(processNode(node, treeData, currentNamespace, callGraph, metadata))
	}

	if shape, isMember := p.shapes.members[nodeType]; isMember {
		object := fieldChild(node, shape.object)
		name := fieldChild(node, shape.name)
		if object != nil && name != nil {
			base := p.resolve(object, treeData, currentNamespace, callGraph, metadata)
			if base == "" {
				return ""
			}
			return base + namespaceSeparator + p.qualifiedName(name.Content(treeData))
		}
	}

	p.processNestedCalls(node, treeData, currentNamespace, callGraph, metadata)
	return p.resolvePath(node.Content(treeData), currentNamespace, callGraph)
}

// resolvePath returns the namespace of a qualified name. The first segment
// resolves through the scope chain, so an imported alias or a variable
// becomes its target. A name that does not resolve is global, as in puts or
// System.Console.
func (p *shapedProcessors) resolvePath(path string, currentNamespace string, callGraph *CallGraph) string {
	var segments []string
	for _, segment := range splitQualifiedName(withoutTypeArguments(path), p.code) {
		if segment = cleanSegment(segment); segment != "" {
			segments = append(segments, segment)
		}
	}
	if len(segments) == 0 {
		return ""
	}

	if assignment, found := searchSymbolInScopeChain(segments[0], currentNamespace, callGraph); found {
		segments[0] = assignment.Namespace
		if targets := callGraph.assignmentGraph.resolve(assignment.Namespace); len(targets) > 0 {
			segments[0] = targets[0].Namespace
		}
	}

	return strings.Join(segments, namespaceSeparator)
}

func (p *shapedProcessors) qualifiedName(name string) string {
	return strings.Join(splitQualifiedName(withoutTypeArguments(name), p.code), namespaceSeparator)
}

// cleanSegment removes the spaces and the null-safe marks around a segment
// of a path, as in C# client?.Send() or client!.Send(). A Ruby predicate,
// such as empty?, loses its mark too. No signature names a predicate.
func cleanSegment(segment string) string {
	return strings.TrimRight(strings.TrimSpace(segment), "?!&")
}

// instanceOf returns the namespace of the value of a call to callee.
func (p *shapedProcessors) instanceOf(callee string) string {
	receiver, method, found := cutLast(callee, namespaceSeparator)
	if found && slices.Contains(p.shapes.constructorMethods, method) {
		return receiver
	}
	return callee
}

func (p *shapedProcessors) valueResult(namespace string, callGraph *CallGraph) processorResult {
	result := newProcessorResult()
	result.ImmediateAssignments = append(result.ImmediateAssignments, callGraph.assignmentGraph.addNode(namespace, nil))
	return result
}

// processNestedCalls processes the calls and creations below a node that is
// not itself a call, as in the index of items[build()].
func (p *shapedProcessors) processNestedCalls(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		_, isCall := p.shapes.calls[child.Type()]
		_, isCreation := p.shapes.creations[child.Type()]
		if isCall || isCreation {
			processNode(child, treeData, currentNamespace, callGraph, metadata)
			continue
		}
		p.processNestedCalls(child, treeData, currentNamespace, callGraph, metadata)
	}
}

func (p *shapedProcessors) processUnhandledChildren(node *sitter.Node, handled []*sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if !slices.ContainsFunc(handled, func(h *sitter.Node) bool { return sameNode(h, child) }) {
			processNode(child, treeData, currentNamespace, callGraph, metadata)
		}
	}
}

func firstValue(result processorResult) string {
	if len(result.ImmediateAssignments) == 0 {
		return ""
	}
	return result.ImmediateAssignments[0].Namespace
}

func sameNode(a, b *sitter.Node) bool {
	return a.StartByte() == b.StartByte() && a.EndByte() == b.EndByte() && a.Type() == b.Type()
}

// fieldChild returns the child in a field, or nil for an empty field name.
func fieldChild(node *sitter.Node, field string) *sitter.Node {
	if field == "" {
		return nil
	}
	return node.ChildByFieldName(field)
}

func childOfType(node *sitter.Node, types ...string) *sitter.Node {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		if child := node.NamedChild(i); slices.Contains(types, child.Type()) {
			return child
		}
	}
	return nil
}

// withoutTypeArguments cuts the type arguments from a name, as in
// List<string> or serde_json::from_str::<Value>.
func withoutTypeArguments(name string) string {
	before, _, _ := strings.Cut(name, "<")
	return before
}

func cutLast(s, separator string) (string, string, bool) {
	i := strings.LastIndex(s, separator)
	if i < 0 {
		return s, "", false
	}
	return s[:i], s[i+len(separator):], true
}
