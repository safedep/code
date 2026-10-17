package callgraph

import (
	"maps"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/safedep/code/pkg/ts"
	sitter "github.com/smacker/go-tree-sitter"
)

// syntaxShapes tells where the call, member, assignment and scope nodes of a
// grammar keep their parts. shapedProfile makes the node processors of a
// language from it, so a grammar with named fields needs no processor code
// of its own.
type syntaxShapes struct {
	calls       map[string]callShape
	creations   map[string]creationShape
	members     map[string]memberShape
	assignments map[string]assignmentShape
	scopes      map[string]scopeShape

	// skipped are node types with no calls below them, such as imports.
	skipped []string

	// values are the node types whose value is the expression in one child:
	// the Rust turbofish, as in iter.collect::<Vec<_>>(), and a cast.
	values map[string]childRef

	// constructorMethods are methods that return a new instance of their
	// receiver, as in OpenAI::Client.new. A variable that gets the result is
	// an instance of the receiver.
	constructorMethods []string

	// classKeywords name the class that holds the code, as Self in Rust or
	// static in PHP. superKeywords name its base class, as parent in PHP.
	// Each one and the instance keyword of the profile resolve to the
	// namespace of the class, or to super, so that a call on this is a
	// call of a method of the class.
	classKeywords []string
	superKeywords []string

	// computedCallees are the node types in the callee position that compute
	// the callee, as $f in PHP $f(). A call of one is unresolved.
	computedCallees map[string]bool

	// classCalls marks a language where a call of a class name makes an
	// instance, as Parser() in Kotlin. A class name starts with an upper
	// case letter.
	classCalls bool

	// trailing names the trailing lambda of a call, as the block of
	// with(x) { } in Kotlin. The grammar makes such a call a call of the
	// call before the lambda, and the lambda is an argument of that call.
	trailing childRef
}

// childRef finds a child of a node by the name of its field. A grammar that
// names no fields, as Kotlin, finds the child by the types of the named
// children on the way down instead. An empty type takes the first named
// child.
type childRef struct {
	field string
	path  []string
	// position picks the named child at position-1, as the name of a
	// Kotlin infix call. Zero picks none.
	position int
}

func named(field string) childRef {
	return childRef{field: field}
}

func typed(types ...string) childRef {
	return childRef{path: types}
}

func childAt(i int) childRef {
	return childRef{position: i + 1}
}

func (r childRef) empty() bool {
	return r.field == "" && len(r.path) == 0 && r.position == 0
}

// of returns the child of node, or nil.
func (r childRef) of(node *sitter.Node) *sitter.Node {
	if r.field != "" {
		return node.ChildByFieldName(r.field)
	}
	if r.position > 0 {
		if r.position > int(node.NamedChildCount()) {
			return nil
		}
		return node.NamedChild(r.position - 1)
	}
	for _, t := range r.path {
		if node = namedChildOfType(node, t); node == nil {
			return nil
		}
	}
	if len(r.path) == 0 {
		return nil
	}
	return node
}

// namedChildOfType returns the first named child of one of the types, which
// a "|" joins, or the first named child for an empty type.
func namedChildOfType(node *sitter.Node, types string) *sitter.Node {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if types == "" || child.Type() == types || strings.Contains(types, "|") && slices.Contains(strings.Split(types, "|"), child.Type()) {
			return child
		}
	}
	return nil
}

// callShape names the parts of a call. A call has either a function, as in
// client.Send(x), or a receiver and a name, as in Ruby.
type callShape struct {
	function  childRef
	receiver  childRef
	name      childRef
	arguments childRef
}

// creationShape names the parts of an object creation, as in new Client(x).
// An empty type means the first named child.
type creationShape struct {
	typeField childRef
	arguments childRef
}

// assignmentShape names the parts of an assignment. An empty right side
// means the last named child.
type assignmentShape struct {
	left  childRef
	right childRef
}

// scopeShape names the parts of a declaration with its own namespace, such
// as a class or a function. A reachable scope is a start node of the DFS.
// A class scope holds the namespace that the instance keyword names. A
// scope with a header processes its other children, as the parameters and
// the base classes, in its namespace before the body. A scope with a
// required body is no definition without one, as an abstract method of
// Kotlin. It runs no code, and a call of it reaches the methods that
// override it. Its arguments still run, as for an enum entry.
type scopeShape struct {
	name         childRef
	body         childRef
	reachable    bool
	class        bool
	header       bool
	bodyRequired bool
}

// superKeyword names the base class of the class that holds a call, as the
// link step reads it.
const superKeyword = "super"

// maxKeywordBytes bounds the text of a node that can be a keyword, so that
// resolve reads no text of a large node.
const maxKeywordBytes = 16

var assignableNamePattern = regexp.MustCompile(`^[$@]*[A-Za-z_][A-Za-z0-9_]*$`)

type shapedProcessors struct {
	shapes     syntaxShapes
	separators []string
}

// shapedProfile adds to profile the node processors that shapes give, and
// the reachable scopes. A shape replaces a processor of profile for the same
// node type.
func shapedProfile(profile languageProfile, shapes syntaxShapes) languageProfile {
	p := &shapedProcessors{shapes: shapes, separators: profile.separators}

	processors := maps.Clone(profile.processors)
	if processors == nil {
		processors = map[string]nodeProcessor{}
	}
	reachable := maps.Clone(profile.reachable)
	if reachable == nil {
		reachable = map[string]bool{}
	}

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
			reachable[nodeType] = true
		}
	}
	for _, nodeType := range shapes.skipped {
		processors[nodeType] = skippedProcessor
	}

	members := maps.Clone(profile.members)
	if members == nil {
		members = map[string]memberShape{}
	}
	maps.Copy(members, shapes.members)
	for nodeType, call := range shapes.calls {
		if call.function.empty() {
			members[nodeType] = memberShape{object: call.receiver, name: call.name}
		}
	}

	profile.processors = processors
	profile.reachable = reachable
	profile.members = members
	return profile
}

// call adds an edge from the current namespace to the callee. Its result is
// the value of the call: the receiver for a constructor method, or else the
// callee.
func (p *shapedProcessors) call(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	shape := p.shapes.calls[node.Type()]

	var callee string
	expression := node
	handled := []*sitter.Node{}

	computed := false
	if function := shape.function.of(node); function != nil {
		callee = p.resolve(function, treeData, currentNamespace, callGraph, metadata)
		expression = function
		handled = append(handled, function)
		if p.computes(function) && shape.arguments.of(node) == nil && p.shapes.trailing.of(node) != nil {
			// A trailing lambda continues the call of the function, as
			// with(x) { } in Kotlin, and the lambda runs in the caller.
			p.processUnhandledChildren(node, handled, treeData, currentNamespace, callGraph, metadata)
			return p.valueResult(callee, callGraph)
		}
		// The value of a call is out of reach, so a call of it, as f()(),
		// can run any function.
		computed = p.computes(function)
	} else if name := shape.name.of(node); name != nil {
		if receiver := shape.receiver.of(node); receiver != nil {
			if base := p.resolve(receiver, treeData, currentNamespace, callGraph, metadata); base != "" {
				callee = base + namespaceSeparator + p.qualifiedName(name.Content(treeData))
			}
			handled = append(handled, receiver)
		} else {
			callee = p.resolvePath(name.Content(treeData), currentNamespace, callGraph)
		}
		handled = append(handled, name)
		computed = p.shapes.computedCallees[name.Type()]
	}

	arguments := []CallArgument{}
	if argumentsNode := shape.arguments.of(node); argumentsNode != nil {
		arguments = resolveCallArguments(argumentsNode, treeData, currentNamespace, callGraph, metadata)
		handled = append(handled, argumentsNode)
	}

	// Blocks and other children can hold calls, as in Ruby list.each do ... end
	p.processUnhandledChildren(node, handled, treeData, currentNamespace, callGraph, metadata)

	switch {
	case computed:
		callGraph.addUnresolved(currentNamespace, expression, nil, treeData, UnresolvedComputedName)
		return newProcessorResult()
	case callee == "":
		// The receiver is a value that the graph does not know, as a
		// literal or a lambda, so the call is a record.
		callGraph.addUnresolved(currentNamespace, expression, expression, treeData, UnresolvedUnknownReceiver)
		return newProcessorResult()
	case p.throughResult(callee, callGraph):
		// The receiver is the value of a call, as in
		// Runtime.getRuntime().exec() or log.info() after
		// log = Logger.getLogger(). The graph keys the call under the
		// callee of that call, which names no type, so the call is a
		// record too. The edge stays for the signatures of the file.
		callGraph.addUnresolved(currentNamespace, expression, expression, treeData, UnresolvedUnknownReceiver)
	}

	callGraph.addEdge(currentNamespace, CallCall, expression, callee, nil, arguments)
	value, instance := p.instanceOf(callee)
	if !instance {
		callGraph.results[value] = true
	}
	return p.valueResult(value, callGraph)
}

// throughResult reports whether a callee is a member of the value of a
// call that makes no instance of a type that it names.
func (p *shapedProcessors) throughResult(callee string, callGraph *CallGraph) bool {
	for base, _, found := cutLast(callee, namespaceSeparator); found; base, _, found = cutLast(base, namespaceSeparator) {
		if callGraph.results[base] {
			return true
		}
	}
	return false
}

// computes reports whether a callee computes the function that the call
// runs: the value of a call or a creation, or a node of computedCallees.
func (p *shapedProcessors) computes(function *sitter.Node) bool {
	nodeType := function.Type()
	_, isCall := p.shapes.calls[nodeType]
	_, isCreation := p.shapes.creations[nodeType]
	return isCall || isCreation || p.shapes.computedCallees[nodeType]
}

// creation adds an edge to the created type. Its result is the type.
func (p *shapedProcessors) creation(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	shape := p.shapes.creations[node.Type()]

	typeNode := shape.typeField.of(node)
	if shape.typeField.empty() && node.NamedChildCount() > 0 {
		typeNode = node.NamedChild(0)
	}
	if typeNode == nil {
		return emptyProcessor(node, treeData, currentNamespace, callGraph, metadata)
	}

	argumentsNode := shape.arguments.of(node)
	if shape.arguments.empty() {
		argumentsNode = childOfType(node, callGraph.profile.argumentList)
	}

	arguments := []CallArgument{}
	handled := []*sitter.Node{typeNode}
	if argumentsNode != nil {
		arguments = resolveCallArguments(argumentsNode, treeData, currentNamespace, callGraph, metadata)
		handled = append(handled, argumentsNode)
	}

	// An object initializer can hold calls, as in C#
	// new ChatOptions { Tools = { AIFunctionFactory.Create(Foo) } }
	p.processUnhandledChildren(node, handled, treeData, currentNamespace, callGraph, metadata)

	created := p.resolve(typeNode, treeData, currentNamespace, callGraph, metadata)
	if p.shapes.computedCallees[typeNode.Type()] {
		callGraph.addUnresolved(currentNamespace, typeNode, nil, treeData, UnresolvedComputedName)
		return newProcessorResult()
	}
	if created == "" {
		return newProcessorResult()
	}

	callGraph.addEdge(currentNamespace, CallNew, typeNode, created, nil, arguments)
	return p.valueResult(created, callGraph)
}

// assignment assigns the values of the right side to a variable in the
// current namespace. It ignores a left side that is not a plain name, as in
// self.client = x.
func (p *shapedProcessors) assignment(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	shape := p.shapes.assignments[node.Type()]

	left := shape.left.of(node)
	right := shape.right.of(node)
	if count := node.NamedChildCount(); shape.right.empty() && count > 0 {
		right = node.NamedChild(int(count) - 1)
	}

	if right == nil || (left != nil && right.StartByte() == left.StartByte()) {
		return newProcessorResult()
	}

	result := processNode(right, treeData, currentNamespace, callGraph, metadata)
	if left == nil || !assignableNamePattern.MatchString(left.Content(treeData)) {
		return newProcessorResult()
	}

	// The variable binds its name with no value too, as a lambda, so a
	// call of the name does not bind a function of an outer scope.
	variable := currentNamespace + namespaceSeparator + left.Content(treeData)
	callGraph.assignmentGraph.addNode(variable, left)
	for _, value := range result.ImmediateAssignments {
		callGraph.assignmentGraph.addAssignment(variable, left, value.Namespace, nil)
	}

	return newProcessorResult()
}

// scope processes the body of a declaration in the namespace of its name.
func (p *shapedProcessors) scope(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	shape := p.shapes.scopes[node.Type()]

	name := shape.name.of(node)
	if name == nil {
		return emptyProcessor(node, treeData, currentNamespace, callGraph, metadata)
	}

	body := shape.body.of(node)
	if body == nil && shape.bodyRequired {
		processNode(childOfType(node, callGraph.profile.argumentList), treeData, currentNamespace, callGraph, metadata)
		return newProcessorResult()
	}

	scopeNamespace := currentNamespace + namespaceSeparator + withoutTypeArguments(name.Content(treeData))
	callGraph.addNode(scopeNamespace, node)
	if shape.class {
		callGraph.classConstructors[scopeNamespace] = true
	}

	if shape.header {
		for _, child := range ts.NamedChildren(node) {
			if !sameNode(child, name) && (body == nil || !sameNode(child, body)) {
				processNode(child, treeData, scopeNamespace, callGraph, metadata)
			}
		}
	}
	if body != nil {
		processChildren(body, treeData, scopeNamespace, callGraph, metadata)
	}

	return newProcessorResult()
}

// enclosingClass returns the namespace of the innermost class scope that
// holds the namespace.
func (p *shapedProcessors) enclosingClass(namespace string, callGraph *CallGraph) (string, bool) {
	for ns := namespace; ; {
		if callGraph.classConstructors[ns] {
			return ns, true
		}
		parent, _, found := cutLast(ns, namespaceSeparator)
		if !found {
			return "", false
		}
		ns = parent
	}
}

// keyword returns the namespace that a keyword of the class names: the
// class for the instance keyword or a class keyword, and super for a super
// keyword. Outside a class a keyword names nothing of the file.
func (p *shapedProcessors) keyword(word, currentNamespace string, callGraph *CallGraph, instance bool) (string, bool) {
	switch {
	case slices.Contains(p.shapes.superKeywords, word):
		return superKeyword, true
	case instance && word == callGraph.profile.instanceKeyword, slices.Contains(p.shapes.classKeywords, word):
		return p.enclosingClass(currentNamespace, callGraph)
	}
	return "", false
}

// resolve returns the namespace of an expression in a callee, receiver or
// type position. A call in the expression adds its own edge, and its value
// becomes the base of the namespace, as in client.chat().create.
func (p *shapedProcessors) resolve(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) string {
	nodeType := node.Type()

	if node.EndByte()-node.StartByte() <= maxKeywordBytes {
		if namespace, ok := p.keyword(node.Content(treeData), currentNamespace, callGraph, true); ok {
			return namespace
		}
	}

	if _, isCall := p.shapes.calls[nodeType]; isCall {
		return firstValue(processNode(node, treeData, currentNamespace, callGraph, metadata))
	}
	if _, isCreation := p.shapes.creations[nodeType]; isCreation {
		return firstValue(processNode(node, treeData, currentNamespace, callGraph, metadata))
	}

	if position, ok := callGraph.profile.transparent[nodeType]; ok && position == firstChild && node.NamedChildCount() == 1 {
		return p.resolve(node.NamedChild(0), treeData, currentNamespace, callGraph, metadata)
	}
	if value, ok := p.shapes.values[nodeType]; ok {
		if inner := value.of(node); inner != nil {
			return p.resolve(inner, treeData, currentNamespace, callGraph, metadata)
		}
	}

	if shape, isMember := p.shapes.members[nodeType]; isMember {
		object := shape.object.of(node)
		name := shape.name.of(node)
		if object != nil && name != nil {
			base := p.resolve(object, treeData, currentNamespace, callGraph, metadata)
			if base == "" {
				return ""
			}
			return base + namespaceSeparator + p.qualifiedName(name.Content(treeData))
		}
	}

	p.processNestedCalls(node, treeData, currentNamespace, callGraph, metadata)
	path := withoutTypeArguments(node.Content(treeData))
	if !qualifiedPathPattern.MatchString(path) {
		// A lambda, a cast or another expression is not a namespace.
		return ""
	}
	return p.resolvePath(path, currentNamespace, callGraph)
}

// qualifiedPathPattern matches a name or a qualified name of a shaped
// language, as in Client, OpenAI::Client, Http\Client or client?.Chat.
var qualifiedPathPattern = regexp.MustCompile(`^[\p{L}_$@\\][\p{L}\p{N}_$@\\.:?!&]*$`)

// resolvePath returns the namespace of a qualified name. The first segment
// resolves through the scope chain, so an imported alias or a variable
// becomes its target. A name that does not resolve is global, as in puts or
// System.Console.
func (p *shapedProcessors) resolvePath(path string, currentNamespace string, callGraph *CallGraph) string {
	var segments []string
	for _, segment := range splitQualifiedName(withoutTypeArguments(path), p.separators) {
		if segment = cleanSegment(segment); segment != "" {
			segments = append(segments, segment)
		}
	}
	if len(segments) == 0 {
		return ""
	}

	if namespace, ok := p.keyword(segments[0], currentNamespace, callGraph, false); ok {
		segments[0] = namespace
	} else if assignment, found := searchSymbolInScopeChain(segments[0], currentNamespace, callGraph); found {
		segments[0] = assignment.Namespace
		if targets := callGraph.assignmentGraph.resolve(assignment.Namespace); len(targets) > 0 {
			segments[0] = targets[0].Namespace
		}
	}

	return strings.Join(segments, namespaceSeparator)
}

func (p *shapedProcessors) qualifiedName(name string) string {
	return strings.Join(splitQualifiedName(withoutTypeArguments(name), p.separators), namespaceSeparator)
}

// cleanSegment removes the spaces and the null-safe marks around a segment
// of a path, as in C# client?.Send() or client!.Send(). A Ruby predicate,
// such as empty?, loses its mark too. No signature names a predicate.
func cleanSegment(segment string) string {
	return strings.TrimRight(strings.TrimSpace(segment), "?!&")
}

// instanceOf returns the namespace of the value of a call to callee, and
// whether the value is an instance of a type that the call names: the
// receiver of a constructor method, as OpenAI::Client.new, or a class that a
// call of its name makes in a language of class calls, as Parser().
func (p *shapedProcessors) instanceOf(callee string) (string, bool) {
	receiver, method, found := cutLast(callee, namespaceSeparator)
	if found && slices.Contains(p.shapes.constructorMethods, method) {
		return receiver, true
	}
	first, _ := utf8.DecodeRuneInString(lastOf(callee))
	return callee, p.shapes.classCalls && unicode.IsUpper(first)
}

// lastOf returns the last segment of a namespace.
func lastOf(key string) string {
	_, last, found := cutLast(key, namespaceSeparator)
	if !found {
		return key
	}
	return last
}

func (p *shapedProcessors) valueResult(namespace string, callGraph *CallGraph) processorResult {
	result := newProcessorResult()
	result.addAssignments(callGraph.assignmentGraph.addNode(namespace, nil))
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

// processUnhandledChildren processes each child of node that is not a
// handled node. A child that holds a handled node, as the call suffix that
// holds the arguments of a Kotlin call, gets the same walk.
func (p *shapedProcessors) processUnhandledChildren(node *sitter.Node, handled []*sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		switch {
		case slices.ContainsFunc(handled, func(h *sitter.Node) bool { return sameNode(h, child) }):
		case slices.ContainsFunc(handled, func(h *sitter.Node) bool { return holds(child, h) }):
			p.processUnhandledChildren(child, handled, treeData, currentNamespace, callGraph, metadata)
		default:
			processNode(child, treeData, currentNamespace, callGraph, metadata)
		}
	}
}

// holds reports whether the node spans the inner node.
func holds(node, inner *sitter.Node) bool {
	return node.StartByte() <= inner.StartByte() && inner.EndByte() <= node.EndByte()
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

func childOfType(node *sitter.Node, types ...string) *sitter.Node {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		if child := node.NamedChild(i); slices.Contains(types, child.Type()) {
			return child
		}
	}
	return nil
}

// withoutTypeArguments removes each type argument list from a name, as in
// List<string>, serde_json::from_str::<Value> or Hmac::<Sha256>::new.
func withoutTypeArguments(name string) string {
	if !strings.Contains(name, "<") {
		return name
	}
	var b strings.Builder
	depth := 0
	for _, r := range name {
		switch {
		case r == '<':
			depth++
		case r == '>' && depth > 0:
			depth--
		case depth == 0:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func cutLast(s, separator string) (string, string, bool) {
	i := strings.LastIndex(s, separator)
	if i < 0 {
		return s, "", false
	}
	return s[:i], s[i+len(separator):], true
}
