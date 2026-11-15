package indexer

import (
	"fmt"
	"slices"
	"strings"

	"github.com/safedep/code/core/ast"
	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/pkg/ts"
	"github.com/safedep/code/plugin/callgraph"
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
	// typ is the declared type of a parameter or a var, or nil. value is
	// the value that a declaration gives the name, or nil.
	typ, value *sitter.Node
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

// goAssignedType reports whether the one variable of a name has the type
// that an assignment gives it, as the link step reads it. A declared type
// is the type of the variable, so it must be that type. A declared
// interface holds any type that implements it. With no declared type,
// each write of the variable must give that type, as the declaration
// cl := o.c with a field c *client and cl = &client{}. A write whose type
// the index does not know, as a range or l := ls[0], can give another type.
func (p *projection) goAssignedType(caller, name string, binding goScope, read fileindex.Typed) bool {
	if read.Result {
		return false
	}
	if binding.typ != nil {
		return p.goTypeName(binding.typ) == read.Type
	}
	fn := p.syntax[caller]
	if fn == nil {
		return false
	}
	agree := true
	walkBody(fn.ChildByFieldName("body"), nil, func(node *sitter.Node) {
		var names, values []*sitter.Node
		switch node.Type() {
		case "short_var_declaration", "assignment_statement", "var_spec":
			names, values = goAssignment(node)
		case "range_clause", "receive_statement":
			names = ts.NamedChildren(node.ChildByFieldName("left"))
		}
		i := slices.IndexFunc(names, func(n *sitter.Node) bool { return p.text(n) == name })
		if i < 0 {
			return
		}
		var value *sitter.Node
		switch {
		case len(values) == len(names):
			value = values[i]
		case len(values) == 1 && len(names) == 2 && i == 0 && goCommaOk(values[0]):
			// The first name of v, ok := m[k] or v, ok := x.(T) takes the
			// value of the expression.
			value = values[0]
		default:
			agree = false
			return
		}
		t, ok := p.goWriteType(caller, value)
		agree = agree && ok && !t.Result && t.Type == read.Type
	})
	return agree
}

// goCommaOk reports whether an expression gives a second value of the form
// ok when two names take it: an index of a map or a type assertion.
func goCommaOk(value *sitter.Node) bool {
	value = goUnparen(value)
	return value != nil && (value.Type() == "index_expression" || value.Type() == "type_assertion_expression")
}

// goWriteType returns the type that a value written to a variable gives:
// the type that it constructs or asserts, the declared type of a field of a
// parameter that it reads, or the declared element type of a map or a slice
// that it indexes, as addConnCall for p.conns[key] with a field
// conns map[string]*addConnCall of the type of the receiver p.
func (p *projection) goWriteType(caller string, value *sitter.Node) (fileindex.Typed, bool) {
	value = goUnparen(value)
	if t, ok := p.goValueType(value); ok {
		return t, true
	}
	if t, ok := p.goFieldValueType(caller, value); ok {
		return t, true
	}
	if value.Type() != "index_expression" {
		return fileindex.Typed{}, false
	}
	var elem *sitter.Node
	switch container := p.goDeclaredType(caller, value.ChildByFieldName("operand")); {
	case container == nil:
	case container.Type() == "map_type":
		elem = container.ChildByFieldName("value")
	case container.Type() == "slice_type" || container.Type() == "array_type":
		elem = container.ChildByFieldName("element")
	}
	if typ := p.goTypeName(elem); typ != "" {
		return fileindex.Typed{Type: typ}, true
	}
	return fileindex.Typed{}, false
}

// goDeclaredType returns the type node that the source declares for a
// value: a parameter or a local of a declared type in scope, a variable
// of the file with a declared type, or a field of a struct of the file
// that a parameter reads. It returns nil for any other value.
func (p *projection) goDeclaredType(caller string, value *sitter.Node) *sitter.Node {
	value = goUnparen(value)
	if value == nil {
		return nil
	}
	switch value.Type() {
	case "identifier":
		if binding, ok := p.goBinding(caller, p.text(value), value.StartByte()); ok {
			return binding.typ
		}
		return p.goFileVarType(p.text(value))
	case "selector_expression":
		return p.goParamField(caller, value)
	}
	return nil
}

// goFileVarType returns the declared type of a variable at the top level
// of the file, or nil.
func (p *projection) goFileVarType(name string) *sitter.Node {
	for _, statement := range ts.NamedChildren(p.tree.Tree().RootNode()) {
		if statement.Type() != "var_declaration" {
			continue
		}
		for _, spec := range goVarSpecs(statement) {
			if slices.ContainsFunc(goNames(spec), func(n *sitter.Node) bool { return p.text(n) == name }) {
				return spec.ChildByFieldName("type")
			}
		}
	}
	return nil
}

// goFieldValueType returns the declared type of a field that a value
// reads, as client for o.c with a parameter o opts and a field c *client
// of the struct opts of the file.
func (p *projection) goFieldValueType(caller string, value *sitter.Node) (fileindex.Typed, bool) {
	if typ := p.goTypeName(p.goParamField(caller, value)); typ != "" {
		return fileindex.Typed{Type: typ}, true
	}
	return fileindex.Typed{}, false
}

// goParamField returns the declared type of a field of a struct of the file
// that a parameter of the caller reads, as the type of c in o.c, or nil.
func (p *projection) goParamField(caller string, value *sitter.Node) *sitter.Node {
	def := p.defs[caller]
	if def == nil || value.Type() != "selector_expression" {
		return nil
	}
	operand, field := value.ChildByFieldName("operand"), value.ChildByFieldName("field")
	if operand == nil || field == nil || operand.Type() != "identifier" {
		return nil
	}
	i := slices.IndexFunc(def.Params, func(t fileindex.Typed) bool { return t.Name == p.text(operand) })
	if i < 0 {
		return nil
	}
	// A local can shadow the parameter, as o in o := other{} after
	// func f(o opts), and hold a value of another type.
	if binding, ok := p.goBinding(caller, p.text(operand), operand.StartByte()); !ok || binding.from != goParamOrigin(p.syntax[caller]) {
		return nil
	}
	return p.goFileField(def.Params[i].Type, p.text(field))
}

// goFileField returns the type of a field of a struct that the file
// declares, or nil.
func (p *projection) goFileField(structName, field string) *sitter.Node {
	for spec := range p.goTypeSpecs() {
		t := spec.ChildByFieldName("type")
		if t == nil || t.Type() != "struct_type" || p.text(spec.ChildByFieldName("name")) != structName {
			continue
		}
		for _, f := range p.goStructFields(t) {
			for _, n := range childrenByField(f, "name") {
				if p.text(n) == field {
					return f.ChildByFieldName("type")
				}
			}
		}
	}
	return nil
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
		// result c in c = &client{}, if the type of the variable is that
		// type.
		return p.goAssignedType(caller, name, binding, read)
	}
	for i, t := range def.Locals {
		if t.Name == name && i < len(origins) && origins[i] == binding.from {
			return t.Type == read.Type && t.Result == read.Result && t.Index == read.Index
		}
	}
	return false
}

// goTypedCall reports whether the type that the link step reads for each
// typed name that starts a target or the receiver of a call holds at the
// call. The call graph can keep a raw target, as x//run for a parameter x
// of a function literal, or bind a local that shadows an import to the
// import. Such a call gets the target of the local, under an alias when its
// binding has a type of its own. docs/projectgraph.md has the cases.
func goTypedCall(p *projection, call *fileindex.Call) bool {
	at := p.byteAt(call.Line, call.Column)
	names := strings.Split(call.Receiver, ".")
	name := names[0]
	if _, bound := p.goBinding(call.Caller, name, at); !isIdentifier(name) || !bound {
		return p.goTypedTargets(call, at)
	}
	local := name
	if p.goTypedAt(call.Caller, name, at) {
		if !slices.ContainsFunc(call.Targets, func(t string) bool { return p.goImportTarget(name, t) }) {
			return p.goTypedTargets(call, at)
		}
	} else {
		alias, ok := p.goAlias(call.Caller, name, at)
		if !ok {
			return false
		}
		local = alias
	}
	call.Targets = []string{fileindex.Join(call.Caller, append(append([]string{local}, names[1:]...), call.Method)...)}
	return true
}

// goTypedTargets reports whether the type that the link step reads for
// each typed name that starts a target of a call holds at the call.
func (p *projection) goTypedTargets(call *fileindex.Call, at uint32) bool {
	for _, target := range call.Targets {
		if n, ok := p.typedName(call.Caller, target); ok && !p.goTypedAt(call.Caller, n, at) {
			return false
		}
	}
	return true
}

// goImportTarget reports whether a target names a member of the import
// that binds the name.
func (p *projection) goImportTarget(name, target string) bool {
	return slices.ContainsFunc(p.ix.Imports, func(imp fileindex.Import) bool {
		return imp.Alias == name && imp.Namespace != "" && fileindex.HasKeyPrefix(target, imp.Namespace)
	})
}

// goAlias returns a name under which the link step reads the type of the
// binding of a name in scope at the byte, when that binding has a type of
// its own. The index adds a typed local of the alias to the caller once.
func (p *projection) goAlias(caller, name string, at uint32) (string, bool) {
	def := p.defs[caller]
	binding, bound := p.goBinding(caller, name, at)
	if def == nil || !bound {
		return "", false
	}
	origins := p.goOrigins[caller]
	i := slices.IndexFunc(def.Locals, func(t fileindex.Typed) bool { return t.Name == name })
	for j := i; i >= 0 && j < len(def.Locals) && j < len(origins); j++ {
		t := def.Locals[j]
		if t.Name != name || origins[j] != binding.from {
			continue
		}
		alias := fmt.Sprintf("%s#%d", name, binding.from)
		if !slices.ContainsFunc(def.Locals, func(l fileindex.Typed) bool { return l.Name == alias }) {
			t.Name = alias
			def.Locals = append(def.Locals, t)
			p.goOrigins[caller] = append(origins, binding.from)
		}
		return alias, true
	}
	return "", false
}

// goValueVisitor records the Go code that calls a function value with no
// call record of its own: a range over a value that the source does not
// show to be data, a call of a field of an anonymous struct of the file,
// and a call of a dereference or a receive. The link step makes each record
// a function-value record. docs/projectgraph.md has the forms.
func goValueVisitor(p *projection) visitor {
	return func(node *sitter.Node, kind string) {
		switch kind {
		case "range_clause":
			value := goUnparen(node.ChildByFieldName("right"))
			if value == nil {
				return
			}
			if r, ok := p.goRangeRecord(value, 0); ok {
				p.addValueCall(r)
			}
		case "call_expression":
			function := goUnparen(node.ChildByFieldName("function"))
			switch {
			case function == nil:
			case function.Type() == "unary_expression" && goOperator(function) == "<-",
				function.Type() == "unary_expression" && goOperator(function) == "*" && p.goValueOperand(function.ChildByFieldName("operand")):
				p.goValueRecord(node)
			case function.Type() == "selector_expression" && p.goAnonFields()[p.text(function.ChildByFieldName("field"))]:
				p.goValueRecord(node)
			}
		}
	}
}

// goValueOperand reports whether the operand of a dereference in the
// function of a call is a value, as p in (*p)(), and no type, as T in the
// conversion (*T)(v) or reflect.SliceHeader in (*reflect.SliceHeader)(v).
// A value is a local, a variable of the file, or any expression other
// than a name or a member of an import.
func (p *projection) goValueOperand(operand *sitter.Node) bool {
	operand = goUnparen(operand)
	if operand == nil {
		return false
	}
	switch operand.Type() {
	case "identifier", "type_identifier":
		name := p.text(operand)
		if _, ok := p.goBinding(p.innermost(operand.StartByte()), name, operand.StartByte()); ok {
			return true
		}
		def := p.defs[p.local(name)]
		return def != nil && def.Kind == fileindex.DefVariable
	case "selector_expression", "qualified_type":
		object := operand.ChildByFieldName("operand")
		if object == nil {
			object = operand.ChildByFieldName("package")
		}
		return object == nil || p.goImportPath(p.text(object)) == ""
	}
	return !strings.HasSuffix(operand.Type(), "_type")
}

// goValueRecord adds the record of a call of a function value at the node.
func (p *projection) goValueRecord(node *sitter.Node) {
	p.addValueCall(goValueCall{node: node, reason: fileindex.UnresolvedComputedName})
}

// goValueCall is the record of a call of a function value, by a call or by
// a range over the value: its site and what the link step reads to decide
// it, as fileindex.UnresolvedRangeValue says.
type goValueCall struct {
	node             *sitter.Node
	reason           fileindex.UnresolvedReason
	receiver, method string
}

// addValueCall adds the record of a call of a function value.
func (p *projection) addValueCall(r goValueCall) {
	site := ast.GetNodePosition(r.node)
	p.ix.Unresolved = append(p.ix.Unresolved, fileindex.Unresolved{
		Caller:   p.caller(p.innermost(r.node.StartByte()), site),
		Line:     int(site.StartLine),
		Column:   int(site.StartColumn),
		Text:     callgraph.ClipText(p.text(r.node)),
		Receiver: r.receiver,
		Method:   r.method,
		Reason:   r.reason,
	})
}

// goRangeRecord returns the record of a range over a value, or false for
// a value that the source shows to be data: a literal, a slice of a value,
// a value of a type of data, an element of a map or a slice of a declared
// type of data, or a local whose declaration gives such a value. A name of
// the package reaches its value through the load code of the package. A
// member, a value of a named type and the result of a call are records of
// the reason range-value, which the link step decides from the fields, the
// types and the functions of the graph. A local that a call gives has the
// record at the call.
func (p *projection) goRangeRecord(value *sitter.Node, depth int) (goValueCall, bool) {
	value = goUnparen(value)
	if value == nil {
		return goValueCall{}, false
	}
	plain := goValueCall{node: value, reason: fileindex.UnresolvedComputedName}
	if depth > maxReferenceSegments {
		return plain, true
	}
	if p.goDataValue(value) {
		return goValueCall{}, false
	}
	switch value.Type() {
	case "slice_expression":
		return goValueCall{}, false
	case "identifier":
		binding, ok := p.goBinding(p.innermost(value.StartByte()), p.text(value), value.StartByte())
		switch {
		case !ok:
			return goValueCall{}, false
		case binding.typ != nil:
			return p.goRangeType(binding.typ, value)
		case binding.value != nil:
			return p.goRangeRecord(binding.value, depth+1)
		}
	case "selector_expression":
		return goValueCall{node: value, reason: fileindex.UnresolvedRangeValue, receiver: callgraph.ClipText(p.text(value.ChildByFieldName("operand"))), method: p.text(value.ChildByFieldName("field"))}, true
	case "index_expression":
		if t := p.goElementType(value.ChildByFieldName("operand")); t != nil {
			return p.goRangeType(t, value)
		}
	case "type_assertion_expression":
		return p.goRangeType(value.ChildByFieldName("type"), value)
	case "call_expression":
		return goValueCall{node: value, reason: fileindex.UnresolvedRangeValue}, true
	}
	return plain, true
}

// goRangeType returns the record of a range over a value of a declared
// type, or false for a type of data. A named type that the file does not
// declare is a record with its name.
func (p *projection) goRangeType(t, at *sitter.Node) (goValueCall, bool) {
	if p.goDataField(t) {
		return goValueCall{}, false
	}
	if name := p.goTypeName(t); name != "" && t.Type() != "function_type" {
		return goValueCall{node: at, reason: fileindex.UnresolvedRangeValue, method: name}, true
	}
	return goValueCall{node: at, reason: fileindex.UnresolvedComputedName}, true
}

// goElementType returns the declared type of an element of a map, a slice
// or an array that a local of a declared type holds, or nil.
func (p *projection) goElementType(operand *sitter.Node) *sitter.Node {
	operand = goUnparen(operand)
	if operand == nil || operand.Type() != "identifier" {
		return nil
	}
	binding, ok := p.goBinding(p.innermost(operand.StartByte()), p.text(operand), operand.StartByte())
	if !ok || binding.typ == nil {
		return nil
	}
	switch binding.typ.Type() {
	case "map_type":
		return binding.typ.ChildByFieldName("value")
	case "slice_type", "array_type":
		return binding.typ.ChildByFieldName("element")
	}
	return nil
}

// goUnparen returns the expression in parentheses.
func goUnparen(node *sitter.Node) *sitter.Node {
	for depth := 0; node != nil && node.Type() == "parenthesized_expression" && depth < maxReferenceSegments; depth++ {
		node = node.NamedChild(0)
	}
	return node
}

// goAnonFields returns the names of the fields of each anonymous struct of
// the file and of each struct type that a function declares, true for a
// name that a field which can hold a function has.
func (p *projection) goAnonFields() map[string]bool {
	if p.anonFields != nil {
		return p.anonFields
	}
	p.anonFields = map[string]bool{}
	var walk func(node *sitter.Node, depth int)
	walk = func(node *sitter.Node, depth int) {
		if depth > maxLocalDepth {
			return
		}
		if node.Type() == "struct_type" && !goTopLevelType(node) {
			for _, field := range p.goStructFields(node) {
				holds := !p.goDataField(field.ChildByFieldName("type"))
				for _, name := range childrenByField(field, "name") {
					p.anonFields[p.text(name)] = p.anonFields[p.text(name)] || holds
				}
			}
		}
		for _, child := range ts.NamedChildren(node) {
			walk(child, depth+1)
		}
	}
	walk(p.tree.Tree().RootNode(), 0)
	return p.anonFields
}

// goTopLevelType reports whether a type is the type of a type declaration
// at the top level of the file, whose fields are definitions of their type.
func goTopLevelType(t *sitter.Node) bool {
	spec := t.Parent()
	if spec == nil || spec.Type() != "type_spec" {
		return false
	}
	decl := spec.Parent()
	return decl != nil && decl.Parent() != nil && decl.Parent().Type() == "source_file"
}

// goAnonFieldDefinitions adds a field definition under the key struct of
// the file for each name of goAnonFields, as file.go//struct//run. The
// keyword struct names no type, so the key binds no name. The link step
// reads the fields of the graph by their names to tell a call of a field
// that can hold a function, as s.run(), from a call of a method.
func (p *projection) goAnonFieldDefinitions() {
	for name, holds := range p.goAnonFields() {
		p.addDefinition(fileindex.Join(p.cg.FileName, "struct", name), fileindex.DefField, p.tree.Tree().RootNode()).Data = !holds
	}
}

// goLiteralCall reports whether a call names a local that the caller
// binds once, to a function literal, and never assigns again, as f in
// f := func() {...}; f(). The index gives the calls of the literal to the
// caller, so the call runs no code that the caller does not hold. An
// assignment, a range or a receive into the name, or an address of it, can
// give it another function.
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
			names, values := goAssignment(node)
			for i, n := range names {
				if p.text(n) == name {
					declared++
					literal = len(values) == len(names) && values[i].Type() == "func_literal"
				}
			}
		case "assignment_statement", "inc_statement", "dec_statement", "range_clause", "receive_statement":
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

// goAssignment returns the names and the values of a short variable
// declaration, an assignment statement, or a var or a const spec.
func goAssignment(node *sitter.Node) (names, values []*sitter.Node) {
	switch node.Type() {
	case "var_spec", "const_spec":
		return childrenByField(node, "name"), ts.NamedChildren(node.ChildByFieldName("value"))
	}
	return ts.NamedChildren(node.ChildByFieldName("left")), ts.NamedChildren(node.ChildByFieldName("right"))
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
	add := func(block *sitter.Node, d goDeclaration) {
		start := goBlockStart(block)
		for i, n := range d.names {
			if n.Type() != "identifier" {
				continue
			}
			name := p.text(n)
			// A short variable declaration assigns a name that its block
			// declares already, and binds no new variable.
			if d.redeclares && slices.ContainsFunc(locals[name], func(s goScope) bool { return s.block == start && s.to == block.EndByte() }) {
				continue
			}
			scope := goScope{from: d.from, to: block.EndByte(), block: start, typ: d.typ}
			if len(d.values) == len(d.names) {
				scope.value = d.values[i]
			}
			locals[name] = append(locals[name], scope)
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
			add(block, goDeclaration{names: childrenByField(node, "name"), from: goParamOrigin(block), typ: node.ChildByFieldName("type")})
		case "short_var_declaration":
			names, values := goAssignment(node)
			add(block, goDeclaration{names: names, values: values, from: node.EndByte(), redeclares: true})
		case "var_spec", "const_spec":
			names, values := goAssignment(node)
			add(block, goDeclaration{names: names, values: values, from: node.EndByte(), typ: node.ChildByFieldName("type")})
		case "type_spec", "type_alias":
			add(block, goDeclaration{names: childrenByField(node, "name"), from: node.StartByte()})
		case "range_clause", "receive_statement":
			if goDeclares(node) {
				add(block, goDeclaration{names: ts.NamedChildren(node.ChildByFieldName("left")), from: node.EndByte()})
			}
		case "type_switch_statement":
			if alias := node.ChildByFieldName("alias"); alias != nil {
				add(block, goDeclaration{names: ts.NamedChildren(alias), from: alias.EndByte()})
			}
		}
		for _, child := range ts.NamedChildren(node) {
			walk(child, block, depth+1)
		}
	}
	walk(function, function, 0)
	return locals
}

// goDeclaration is a declaration that goScopes reads: the names, their
// values, the start of their scope, their declared type, and whether a
// short variable declaration declares them.
type goDeclaration struct {
	names, values []*sitter.Node
	from          uint32
	typ           *sitter.Node
	redeclares    bool
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
