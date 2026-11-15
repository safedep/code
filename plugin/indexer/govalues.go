package indexer

import (
	"slices"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/pkg/ts"
	sitter "github.com/smacker/go-tree-sitter"
)

// goReference returns the per-file target of a Go value that can give a
// function, or the member node of a method value whose receiver the index
// cannot name. A call of a Go function value concerns no target. That is
// sound only when the index records each function that a value can give,
// in each form that docs/projectgraph.md lists.
func goReference(p *projection, caller string, value *sitter.Node) (string, *sitter.Node) {
	value = goValueName(value)
	if value == nil {
		return "", nil
	}
	switch value.Type() {
	case "identifier":
		if p.goTypedLocal(caller, value) {
			return "", nil
		}
		target, _ := p.goName(caller, value)
		return target, nil
	case "selector_expression":
		// The link step reads the type of the first binding of a typed
		// name, so a method value on another binding of the name keeps
		// only its method name.
		if root := goSelectorRoot(value); root != nil && !p.goTypedAt(caller, p.text(root), root.StartByte()) {
			return "", value
		}
		if target, ok := p.reference(*p.rules().references, caller, value); ok {
			return target, nil
		}
		if base, ok := p.goOperand(caller, value.ChildByFieldName("operand")); ok {
			return fileindex.Join(base, p.text(value.ChildByFieldName("field"))), nil
		}
		return "", value
	case "type_instantiation_expression":
		// The grammar parses F[A, B] with more than one type argument
		// as a type.
		target, _ := p.goTypeTarget(caller, value.NamedChild(0))
		return target, nil
	}
	return "", nil
}

// goSelectorRoot returns the name that starts a member chain, as x for
// x.cfg.Run, or nil for a chain that starts with no name.
func goSelectorRoot(node *sitter.Node) *sitter.Node {
	for depth := 0; node != nil && node.Type() == "selector_expression" && depth < maxReferenceSegments; depth++ {
		node = node.ChildByFieldName("operand")
	}
	if node == nil || node.Type() != "identifier" {
		return nil
	}
	return node
}

// goValueName returns the expression that names the function of a value:
// the value in parentheses, the operand of &, which a dereference gives
// back, and the function of a generic instantiation, as F for F[int]. An
// index expression can be an element of a slice or a map, and then its
// operand names a variable of the container.
func goValueName(node *sitter.Node) *sitter.Node {
	for depth := 0; node != nil && depth < maxReferenceSegments; depth++ {
		switch node.Type() {
		case "parenthesized_expression":
			node = node.NamedChild(0)
		case "index_expression":
			node = node.ChildByFieldName("operand")
		case "unary_expression":
			if goOperator(node) != "&" {
				return node
			}
			node = node.ChildByFieldName("operand")
		default:
			return node
		}
	}
	return node
}

// goOperand returns the per-file target of the receiver of a method value,
// as the key that a call on the receiver walks from: the type of a
// composite literal, a conversion, a type assertion or new, the function
// whose result a call gives, or a name or a member chain. A receiver that
// the index cannot type, as an element of a slice, gives none.
func (p *projection) goOperand(caller string, node *sitter.Node) (string, bool) {
	if node == nil {
		return "", false
	}
	switch node.Type() {
	case "parenthesized_expression":
		return p.goOperand(caller, node.NamedChild(0))
	case "unary_expression":
		if op := goOperator(node); op != "*" && op != "&" {
			return "", false
		}
		return p.goOperand(caller, node.ChildByFieldName("operand"))
	case "identifier":
		return p.goName(caller, node)
	case "selector_expression":
		base, ok := p.goOperand(caller, node.ChildByFieldName("operand"))
		return fileindex.Join(base, p.text(node.ChildByFieldName("field"))), ok
	case "composite_literal", "type_assertion_expression", "type_conversion_expression":
		return p.goTypeTarget(caller, node.ChildByFieldName("type"))
	case "call_expression":
		return p.goResultOperand(caller, node)
	}
	return "", false
}

// goResultOperand returns the per-file target of the result of a call:
// the type that new or make gives, or the function that the call names. A
// call of a local function value has a result of no known type.
func (p *projection) goResultOperand(caller string, call *sitter.Node) (string, bool) {
	function := call.ChildByFieldName("function")
	for function != nil && function.Type() == "parenthesized_expression" {
		function = function.NamedChild(0)
	}
	if function == nil || function.Type() != "identifier" {
		return p.goOperand(caller, function)
	}
	name := p.text(function)
	if _, typed := p.typedScope(caller, name); typed {
		return "", false
	}
	if p.goPredeclared(name) {
		args := call.ChildByFieldName("arguments")
		if (name == "new" || name == "make") && args != nil && args.NamedChildCount() > 0 {
			return p.goTypeTarget(caller, args.NamedChild(0))
		}
		return "", false
	}
	return p.goName(caller, function)
}

// goTypeTarget returns the per-file target of the named type that a type
// node names, as dep//Thing for *dep.Thing or Box for Box[int].
func (p *projection) goTypeTarget(caller string, node *sitter.Node) (string, bool) {
	for depth := 0; node != nil && depth < maxReferenceSegments; depth++ {
		switch node.Type() {
		case "pointer_type", "parenthesized_type":
			node = node.NamedChild(0)
		case "generic_type":
			node = node.ChildByFieldName("type")
		case "type_identifier", "identifier":
			return p.goName(caller, node)
		case "qualified_type":
			pkg, ok := p.goName(caller, node.ChildByFieldName("package"))
			return fileindex.Join(pkg, p.text(node.ChildByFieldName("name"))), ok
		default:
			return "", false
		}
	}
	return "", false
}

// goName returns the per-file target of a name: the definition that the
// scope chain binds, a typed local, or an import. A name that no local of
// the caller declares in scope keeps its text, as a function of another
// file of the package, and the link step resolves it or drops it. A typed
// local of a closed block does not bind the name. A predeclared name keeps
// its text too, because a function of another file of the package can
// shadow it, as func clear(). A local that the index does not type gives
// none.
func (p *projection) goName(caller string, node *sitter.Node) (string, bool) {
	if node == nil {
		return "", false
	}
	name := p.text(node)
	inScope := p.goLocal(caller, name, node.StartByte())
	if _, typed := p.typedScope(caller, name); typed && !inScope {
		return name, true
	}
	if target, ok := p.valueOf(*p.rules().references, caller, name); ok {
		return target, true
	}
	if name == "_" || inScope {
		return "", false
	}
	return name, true
}

// goTypedLocal reports whether a name is a parameter or a local of a
// declared type that is in scope at the node. Such a name holds a value of
// the type, and a value is no function. The index records the typed locals
// of a function with no extent. So a local of a closed block can hide a
// function of the package with the same name. A call of the value is a
// function-value record that concerns no target, and the reference to the
// function is then the only edge to it.
func (p *projection) goTypedLocal(caller string, node *sitter.Node) bool {
	name := p.text(node)
	_, typed := p.typedScope(caller, name)
	return typed && p.goLocal(caller, name, node.StartByte())
}

// goPredeclared reports whether a name is a predeclared name of Go, as int
// or len, that no definition of the file shadows. A declaration of another
// file of the package can still shadow it. So a reference keeps the name,
// and only two rules read this. The Data mark of a package variable hides
// no function, because its value is a reference of its file, and a search
// reaches the file with the variable. new and make take a type argument
// only as the builtins.
func (p *projection) goPredeclared(name string) bool {
	return langcode.IsBuiltin(langcode.Go, name) && p.defs[p.local(name)] == nil
}

// goScope holds the bytes of the source where a local name of a Go
// function is in scope, and the start of the block that declares it.
type goScope struct {
	from, to, block uint32
}

// goBlocks are the nodes whose extent is the scope of a name that a
// declaration in them makes.
var goBlocks = map[string]bool{
	"block": true, "if_statement": true, "for_statement": true, "expression_switch_statement": true,
	"type_switch_statement": true, "select_statement": true, "expression_case": true, "type_case": true,
	"default_case": true, "communication_case": true, "func_literal": true,
	"function_declaration": true, "method_declaration": true,
}

// goTypeLiterals are the type literals whose names are no locals.
var goTypeLiterals = map[string]bool{"function_type": true, "interface_type": true, "struct_type": true}

// goLocal reports whether a local name of the caller is in scope at the
// byte: a parameter, a result, a variable, a constant or a type that the
// function or a function literal in it declares.
func (p *projection) goLocal(caller, name string, at uint32) bool {
	_, ok := p.goBinding(caller, name, at)
	return ok
}

// goBinding returns the scope of the local name of the caller that is in
// scope at the byte. An inner scope starts after an outer one that holds
// it, so the scope with the last start is the binding.
func (p *projection) goBinding(caller, name string, at uint32) (goScope, bool) {
	var binding goScope
	found := false
	for _, s := range p.goScopesOf(caller)[name] {
		if s.from <= at && at < s.to && (!found || s.from > binding.from) {
			binding, found = s, true
		}
	}
	return binding, found
}

// goScopesOf returns the scopes of each local name of the caller, as
// goScopes gives them, once per caller.
func (p *projection) goScopesOf(caller string) map[string][]goScope {
	if p.goScoped == nil {
		p.goScoped = map[string]map[string][]goScope{}
	}
	locals, ok := p.goScoped[caller]
	if !ok {
		locals = p.goScopes(p.syntax[caller])
		p.goScoped[caller] = locals
	}
	return locals
}

// goTypedAt reports whether the binding of a typed name in scope at the
// byte has the type that the link step reads. The index records the typed
// locals of a function with no extent, and the link step reads the first
// parameter or local of the name. An inner block can bind the name again,
// as x in x := local{}; { x := remote{}; x.run() }. A call on the name
// there runs a method of the inner type, and the type of the first binding
// loses it.
func (p *projection) goTypedAt(caller, name string, at uint32) bool {
	def, fn := p.defs[caller], p.syntax[caller]
	if def == nil || fn == nil {
		return true
	}
	origins := p.goOrigins[caller]
	read, origin, ok := fileindex.Typed{}, goParamOrigin(fn), false
	for _, t := range def.Params {
		if t.Name == name {
			read, ok = t, true
			break
		}
	}
	if !ok {
		i := slices.IndexFunc(def.Locals, func(t fileindex.Typed) bool { return t.Name == name })
		if i < 0 || i >= len(origins) {
			return true
		}
		read, origin = def.Locals[i], origins[i]
	}
	binding, bound := p.goBinding(caller, name, at)
	switch {
	case !bound:
		// An assignment types a variable of the package, which no local
		// binds.
		return origin == goAssignedOrigin
	case binding.from == origin:
		return true
	case origin == goAssignedOrigin && len(p.goScopesOf(caller)[name]) == 1:
		// The function binds the name once, so the assignment types that
		// one variable, as cl in cl := o.c; cl = &client{} or a named
		// result c in c = &client{}.
		return true
	}
	for i, t := range def.Locals {
		if t.Name == name && i < len(origins) && origins[i] == binding.from {
			return t.Type == read.Type && t.Result == read.Result && t.Index == read.Index
		}
	}
	return false
}

// goTypedCall reports whether the type that the link step reads for each
// typed name that starts a target of a call holds at the call.
func goTypedCall(p *projection, call fileindex.Call) bool {
	for _, target := range call.Targets {
		if name, ok := p.typedName(call.Caller, target); ok && !p.goTypedAt(call.Caller, name, p.byteAt(call.Line, call.Column)) {
			return false
		}
	}
	return true
}

// goLiteralCall reports whether a call names a local that the caller
// binds once, to a function literal, and never assigns again, as f in
// f := func() {...}; f(). The index gives the calls of the literal to the
// caller, so the call runs no code that the caller does not hold. An
// assignment or an address of the name can give it another function.
func goLiteralCall(p *projection, call fileindex.Call) bool {
	name := call.Callee
	if call.Receiver != "" || !isIdentifier(name) || len(p.goScopesOf(call.Caller)[name]) != 1 {
		return false
	}
	if _, bound := p.goBinding(call.Caller, name, p.byteAt(call.Line, call.Column)); !bound {
		return false
	}
	fn := p.syntax[call.Caller]
	if fn == nil {
		return false
	}
	literal, other, declared := false, false, 0
	walkBody(fn.ChildByFieldName("body"), nil, func(node *sitter.Node) {
		switch node.Type() {
		case "short_var_declaration", "var_spec":
			left, right := node.ChildByFieldName("left"), node.ChildByFieldName("right")
			var names []*sitter.Node
			if node.Type() == "var_spec" {
				names, right = childrenByField(node, "name"), node.ChildByFieldName("value")
			} else if left != nil {
				names = ts.NamedChildren(left)
			}
			var values []*sitter.Node
			if right != nil {
				values = ts.NamedChildren(right)
			}
			for i, n := range names {
				if p.text(n) == name {
					declared++
					literal = len(values) == len(names) && values[i].Type() == "func_literal"
				}
			}
		case "assignment_statement", "inc_statement", "dec_statement":
			if left := node.ChildByFieldName("left"); left != nil && slices.ContainsFunc(ts.NamedChildren(left), func(n *sitter.Node) bool { return p.text(n) == name }) {
				other = true
			}
		case "unary_expression":
			if operand := node.ChildByFieldName("operand"); goOperator(node) == "&" && operand != nil && p.text(operand) == name {
				other = true
			}
		}
	})
	return literal && !other && declared == 1
}

// byteAt returns the start of the innermost named node at a line and a
// column of a site, as a call or a record holds them.
func (p *projection) byteAt(line, column int) uint32 {
	at := sitter.Point{Row: uint32(max(line-1, 0)), Column: uint32(max(column-1, 0))}
	return p.tree.Tree().RootNode().NamedDescendantForPointRange(at, at).StartByte()
}

// goAssignedOrigin is the origin of a typed local that an assignment types and
// that no declaration binds, as client in client = &http.Client{}.
const goAssignedOrigin = ^uint32(0)

// goParamOrigin returns the start of the scope of the parameters of a
// function, as goScopes records it.
func goParamOrigin(fn *sitter.Node) uint32 {
	return fn.StartByte()
}

// goBlockStart returns the start of the block that a name of the block
// belongs to. The parameters of a function share the block of its body,
// so x, err := f() in the body binds no new x for a parameter x.
func goBlockStart(block *sitter.Node) uint32 {
	switch block.Type() {
	case "function_declaration", "method_declaration", "func_literal":
		if body := block.ChildByFieldName("body"); body != nil {
			return body.StartByte()
		}
	}
	return block.StartByte()
}

// goScopes returns the scopes of each name that a function declares. The
// scope of a variable starts after its declaration, so x in x := x names
// the outer x.
func (p *projection) goScopes(function *sitter.Node) map[string][]goScope {
	locals := map[string][]goScope{}
	if function == nil || function.Type() != "function_declaration" && function.Type() != "method_declaration" {
		return locals
	}
	add := func(nodes []*sitter.Node, from uint32, block *sitter.Node, redeclares bool) {
		start := goBlockStart(block)
		for _, n := range nodes {
			if n.Type() != "identifier" {
				continue
			}
			name := p.text(n)
			// A short variable declaration assigns a name that its block
			// declares already, and binds no new variable.
			if redeclares && slices.ContainsFunc(locals[name], func(s goScope) bool { return s.block == start && s.to == block.EndByte() }) {
				continue
			}
			locals[name] = append(locals[name], goScope{from: from, to: block.EndByte(), block: start})
		}
	}
	var walk func(node, block *sitter.Node, depth int)
	walk = func(node, block *sitter.Node, depth int) {
		if depth > maxLocalDepth {
			return
		}
		switch {
		case goTypeLiterals[node.Type()]:
			// The parameters of a function type and the fields of a
			// struct type declare no local.
			return
		case goBlocks[node.Type()]:
			block = node
		}
		switch node.Type() {
		case "parameter_declaration", "variadic_parameter_declaration":
			add(childrenByField(node, "name"), goParamOrigin(block), block, false)
		case "short_var_declaration":
			add(ts.NamedChildren(node.ChildByFieldName("left")), node.EndByte(), block, true)
		case "var_spec", "const_spec":
			add(childrenByField(node, "name"), node.EndByte(), block, false)
		case "type_spec", "type_alias":
			add(childrenByField(node, "name"), node.StartByte(), block, false)
		case "range_clause", "receive_statement":
			if goDeclares(node) {
				add(ts.NamedChildren(node.ChildByFieldName("left")), node.EndByte(), block, false)
			}
		case "type_switch_statement":
			if alias := node.ChildByFieldName("alias"); alias != nil {
				add(ts.NamedChildren(alias), alias.EndByte(), block, false)
			}
		}
		for _, child := range ts.NamedChildren(node) {
			walk(child, block, depth+1)
		}
	}
	walk(function, function, 0)
	return locals
}

// goOperator returns the operator of a unary expression, as & or <-.
func goOperator(node *sitter.Node) string {
	if op := node.ChildByFieldName("operator"); op != nil {
		return op.Type()
	}
	return ""
}

// goDeclares reports whether a range clause or a receive declares its left
// side with :=.
func goDeclares(node *sitter.Node) bool {
	for i := 0; i < int(node.ChildCount()); i++ {
		if node.Child(i).Type() == ":=" {
			return true
		}
	}
	return false
}
