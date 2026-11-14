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
		// A Go parameter or local of a declared type holds a value of
		// the type, and a value is no function.
		if _, typed := p.typedScope(caller, p.text(value)); typed {
			return "", nil
		}
		target, _ := p.goName(caller, value)
		return target, nil
	case "selector_expression":
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
// the caller declares keeps its text, as a function of another file of
// the package, and the link step resolves it or drops it. A predeclared
// name keeps its text too, because a function of another file of the
// package can shadow it, as func clear(). A local that the index does not
// type gives none.
func (p *projection) goName(caller string, node *sitter.Node) (string, bool) {
	if node == nil {
		return "", false
	}
	name := p.text(node)
	if target, ok := p.valueOf(*p.rules().references, caller, name); ok {
		return target, true
	}
	if name == "_" || p.goLocal(caller, name, node.StartByte()) {
		return "", false
	}
	return name, true
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
// function is in scope.
type goScope struct {
	from, to uint32
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
	if p.goScoped == nil {
		p.goScoped = map[string]map[string][]goScope{}
	}
	locals, ok := p.goScoped[caller]
	if !ok {
		locals = p.goScopes(p.syntax[caller])
		p.goScoped[caller] = locals
	}
	return slices.ContainsFunc(locals[name], func(s goScope) bool { return s.from <= at && at < s.to })
}

// goScopes returns the scopes of each name that a function declares. The
// scope of a variable starts after its declaration, so x in x := x names
// the outer x.
func (p *projection) goScopes(function *sitter.Node) map[string][]goScope {
	locals := map[string][]goScope{}
	if function == nil || function.Type() != "function_declaration" && function.Type() != "method_declaration" {
		return locals
	}
	add := func(nodes []*sitter.Node, from uint32, block *sitter.Node) {
		for _, n := range nodes {
			if n.Type() == "identifier" {
				locals[p.text(n)] = append(locals[p.text(n)], goScope{from: from, to: block.EndByte()})
			}
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
			add(childrenByField(node, "name"), block.StartByte(), block)
		case "short_var_declaration":
			add(ts.NamedChildren(node.ChildByFieldName("left")), node.EndByte(), block)
		case "var_spec", "const_spec":
			add(childrenByField(node, "name"), node.EndByte(), block)
		case "type_spec", "type_alias":
			add(childrenByField(node, "name"), node.StartByte(), block)
		case "range_clause", "receive_statement":
			if goDeclares(node) {
				add(ts.NamedChildren(node.ChildByFieldName("left")), node.EndByte(), block)
			}
		case "type_switch_statement":
			if alias := node.ChildByFieldName("alias"); alias != nil {
				add(ts.NamedChildren(alias), alias.EndByte(), block)
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
