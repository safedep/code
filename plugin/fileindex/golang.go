package fileindex

import (
	"github.com/safedep/code/core"
	fidx "github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/pkg/ts"
	sitter "github.com/smacker/go-tree-sitter"
)

// maxLocalDepth bounds the walk of a Go function body for its local
// variables.
const maxLocalDepth = 64

// goPackage returns the name of the package clause of a Go file.
func (p *projection) goPackage() string {
	for _, statement := range ts.NamedChildren(p.tree.Tree().RootNode()) {
		if statement.Type() != "package_clause" {
			continue
		}
		for _, name := range ts.NamedChildren(statement) {
			if name.Type() == "package_identifier" {
				return p.text(name)
			}
		}
	}
	return ""
}

// addGoTypes adds each top-level named type of a Go file. An interface is an
// interface. Any other named type is a class, because it can have methods.
// The call graph has no node for a type, so the tree gives the types.
func (p *projection) addGoTypes() {
	if !p.isGo() {
		return
	}
	for _, statement := range ts.NamedChildren(p.tree.Tree().RootNode()) {
		if statement.Type() != "type_declaration" {
			continue
		}
		for _, spec := range ts.NamedChildren(statement) {
			if spec.Type() != "type_spec" && spec.Type() != "type_alias" {
				continue
			}
			key := p.local(p.text(spec.ChildByFieldName("name")))
			if _, defined := p.defs[key]; defined {
				continue
			}
			kind := fidx.DefClass
			if t := spec.ChildByFieldName("type"); t != nil && t.Type() == "interface_type" {
				kind = fidx.DefInterface
			}
			p.defs[key] = &fidx.Definition{
				Key:       key,
				Kind:      kind,
				Name:      fidx.Last(key),
				StartLine: lineOf(spec),
				EndLine:   int(spec.EndPoint().Row) + 1,
			}
			p.addSyntax(key, spec)
		}
	}
}

// goBases returns the types that a Go type embeds, and the type that an
// alias names. A method of such a type is a method of the type too.
func (p *projection) goBases(spec *sitter.Node) []string {
	t := spec.ChildByFieldName("type")
	if t == nil {
		return nil
	}
	if spec.Type() == "type_alias" {
		return nonEmpty(p.goTypeName(t))
	}
	var bases []string
	switch t.Type() {
	case "struct_type":
		for _, field := range p.goStructFields(t) {
			if field.ChildByFieldName("name") == nil {
				bases = append(bases, nonEmpty(p.goTypeName(field.ChildByFieldName("type")))...)
			}
		}
	case "interface_type":
		for _, elem := range ts.NamedChildren(t) {
			if elem.Type() == "type_elem" && elem.NamedChildCount() == 1 {
				bases = append(bases, nonEmpty(p.goTypeName(elem.NamedChild(0)))...)
			}
		}
	}
	return bases
}

// goFields returns the named fields of a Go struct that declare a type name.
func (p *projection) goFields(spec *sitter.Node) []fidx.Typed {
	t := spec.ChildByFieldName("type")
	if t == nil || t.Type() != "struct_type" || spec.Type() != "type_spec" {
		return nil
	}
	var fields []fidx.Typed
	for _, field := range p.goStructFields(t) {
		typ := p.goTypeName(field.ChildByFieldName("type"))
		if typ == "" {
			continue
		}
		for _, name := range childrenByField(field, "name") {
			fields = append(fields, fidx.Typed{Name: p.text(name), Type: typ})
		}
	}
	return fields
}

func (p *projection) goStructFields(structType *sitter.Node) []*sitter.Node {
	var fields []*sitter.Node
	for _, list := range ts.NamedChildren(structType) {
		if list.Type() != "field_declaration_list" {
			continue
		}
		for _, field := range ts.NamedChildren(list) {
			if field.Type() == "field_declaration" {
				fields = append(fields, field)
			}
		}
	}
	return fields
}

// addGoSignatures adds the receiver, the parameters, the arity, the first
// result and the typed local variables of each Go function and method.
func (p *projection) addGoSignatures() {
	if !p.isGo() {
		return
	}
	for key, def := range p.defs {
		node := p.syntax[key]
		if node == nil || def.Kind != fidx.DefFunction && def.Kind != fidx.DefMethod {
			continue
		}
		if receiver := node.ChildByFieldName("receiver"); receiver != nil {
			def.Params, _, _ = p.goParams(receiver, nil)
		}
		var params []fidx.Typed
		params, def.Arity, def.Variadic = p.goParams(node.ChildByFieldName("parameters"), p.goConstraints(node))
		def.Params = append(def.Params, params...)
		def.Returns = p.goResult(node.ChildByFieldName("result"))
		def.Locals = p.goLocals(node.ChildByFieldName("body"))
	}
}

// goConstraints returns the type that the constraint of each type parameter
// of a generic function names, as Runner for T in func Apply[T Runner](t T).
// A value of the type parameter holds a type that implements it. A
// constraint that names no one type, as any or ~int | ~string, gives "".
func (p *projection) goConstraints(function *sitter.Node) map[string]string {
	list := function.ChildByFieldName("type_parameters")
	if list == nil {
		return nil
	}
	constraints := map[string]string{}
	for _, param := range ts.NamedChildren(list) {
		if param.Type() != "type_parameter_declaration" {
			continue
		}
		typ := ""
		if constraint := param.ChildByFieldName("type"); constraint != nil && constraint.NamedChildCount() == 1 {
			typ = p.goTypeName(constraint.NamedChild(0))
		}
		for _, name := range childrenByField(param, "name") {
			constraints[p.text(name)] = typ
		}
	}
	return constraints
}

// goParams returns the parameters of a list that declare a type name, the
// count of all parameters, and whether the last one is variadic. A type
// parameter stands for its constraint.
func (p *projection) goParams(list *sitter.Node, constraints map[string]string) ([]fidx.Typed, int, bool) {
	if list == nil {
		return nil, 0, false
	}
	var typed []fidx.Typed
	arity, variadic := 0, false
	for _, param := range ts.NamedChildren(list) {
		switch param.Type() {
		case "parameter_declaration":
		case "variadic_parameter_declaration":
			variadic = true
		default:
			continue
		}
		names := childrenByField(param, "name")
		arity += max(len(names), 1)
		typ := p.goTypeName(param.ChildByFieldName("type"))
		if constraint, ok := constraints[typ]; ok {
			typ = constraint
		}
		if typ == "" || variadic {
			continue
		}
		for _, name := range names {
			typed = append(typed, fidx.Typed{Name: p.text(name), Type: typ})
		}
	}
	return typed, arity, variadic
}

// goResult returns the type name of the first result of a function.
func (p *projection) goResult(result *sitter.Node) string {
	if result == nil {
		return ""
	}
	if result.Type() != "parameter_list" {
		return p.goTypeName(result)
	}
	for _, param := range ts.NamedChildren(result) {
		if param.Type() == "parameter_declaration" {
			return p.goTypeName(param.ChildByFieldName("type"))
		}
	}
	return ""
}

// goLocals returns the local variables of a function body whose type the
// source declares, as var p Parser, or constructs, as p := &Parser{},
// p := new(Parser) or p := v.(*Parser). A name that holds the first result
// of a call, as p := NewParser(), names the function. A function literal in
// the body shares the scope of the body, so its names count too.
func (p *projection) goLocals(body *sitter.Node) []fidx.Typed {
	var locals []fidx.Typed
	var walk func(node *sitter.Node, depth int)
	walk = func(node *sitter.Node, depth int) {
		if depth > maxLocalDepth {
			return
		}
		switch node.Type() {
		case "var_spec":
			names := childrenByField(node, "name")
			if typ := p.goTypeName(node.ChildByFieldName("type")); typ != "" {
				for _, name := range names {
					locals = append(locals, fidx.Typed{Name: p.text(name), Type: typ})
				}
			} else {
				locals = append(locals, p.goAssigned(names, node.ChildByFieldName("value"))...)
			}
		case "short_var_declaration", "assignment_statement":
			if left := node.ChildByFieldName("left"); left != nil {
				locals = append(locals, p.goAssigned(ts.NamedChildren(left), node.ChildByFieldName("right"))...)
			}
		}
		for _, child := range ts.NamedChildren(node) {
			walk(child, depth+1)
		}
	}
	if body != nil {
		walk(body, 0)
	}
	return locals
}

// goAssigned returns the typed names of an assignment. One call on the
// right of more names gives its first result to the first name.
func (p *projection) goAssigned(names []*sitter.Node, right *sitter.Node) []fidx.Typed {
	if right == nil {
		return nil
	}
	values := ts.NamedChildren(right)
	if len(values) == 1 && len(names) > 1 && values[0].Type() == "call_expression" {
		names = names[:1]
	}
	if len(values) != len(names) {
		return nil
	}
	var typed []fidx.Typed
	for i, name := range names {
		if name.Type() != "identifier" {
			continue
		}
		if t, ok := p.goValueType(values[i]); ok {
			t.Name = p.text(name)
			typed = append(typed, t)
		}
	}
	return typed
}

// goValueType returns the type that a value constructs, or the function
// whose result it holds.
func (p *projection) goValueType(value *sitter.Node) (fidx.Typed, bool) {
	switch value.Type() {
	case "unary_expression":
		if operand := value.ChildByFieldName("operand"); operand != nil && operand.Type() == "composite_literal" {
			return p.goValueType(operand)
		}
	case "composite_literal", "type_assertion_expression":
		if typ := p.goTypeName(value.ChildByFieldName("type")); typ != "" {
			return fidx.Typed{Type: typ}, true
		}
	case "type_conversion_expression":
		// A call of a generic function with type arguments, as
		// NewBox[int](1), parses as a conversion.
		if t := value.ChildByFieldName("type"); t != nil && t.Type() == "generic_type" {
			if function := t.ChildByFieldName("type"); function != nil {
				return fidx.Typed{Type: p.text(function), Result: true}, true
			}
		}
	case "call_expression":
		function := value.ChildByFieldName("function")
		if function == nil {
			break
		}
		args := value.ChildByFieldName("arguments")
		if p.text(function) == "new" && args != nil && args.NamedChildCount() == 1 {
			if typ := p.goTypeName(args.NamedChild(0)); typ != "" {
				return fidx.Typed{Type: typ}, true
			}
			break
		}
		switch function.Type() {
		case "identifier", "selector_expression":
			return fidx.Typed{Type: p.text(function), Result: true}, true
		}
	}
	return fidx.Typed{}, false
}

// goTypeName returns the name of the named type that a type node names, as
// Parser for Parser, *Parser or Parser[T], and pkg.Parser for a qualified
// type. A predeclared type, as string, names no type of the project.
func (p *projection) goTypeName(node *sitter.Node) string {
	if node == nil {
		return ""
	}
	switch node.Type() {
	case "type_identifier":
		if name := p.text(node); !langcode.IsBuiltin(langcode.Go, name) {
			return name
		}
	case "qualified_type":
		return p.text(node)
	case "pointer_type", "parenthesized_type":
		if node.NamedChildCount() == 1 {
			return p.goTypeName(node.NamedChild(0))
		}
	case "generic_type":
		return p.goTypeName(node.ChildByFieldName("type"))
	}
	return ""
}

// childrenByField returns the children of the node with the field name, as
// the names of a, b int.
func childrenByField(node *sitter.Node, field string) []*sitter.Node {
	var children []*sitter.Node
	for i := 0; i < int(node.ChildCount()); i++ {
		if node.FieldNameForChild(i) == field {
			children = append(children, node.Child(i))
		}
	}
	return children
}

// typedScope returns the definition in the scope chain of the caller that
// declares the type of a parameter or a local variable of the name.
func (p *projection) typedScope(caller, name string) (string, bool) {
	for scope := caller; fidx.HasKeyPrefix(scope, p.cg.FileName) && scope != p.cg.FileName; scope = fidx.Parent(scope) {
		def, ok := p.defs[scope]
		if !ok {
			continue
		}
		for _, typed := range [][]fidx.Typed{def.Params, def.Locals} {
			for _, t := range typed {
				if t.Name == name {
					return scope, true
				}
			}
		}
	}
	return "", false
}

func nonEmpty(s string) []string {
	if s == "" {
		return nil
	}
	return []string{s}
}

// isGo reports whether the projection is of a Go file.
func (p *projection) isGo() bool {
	return p.ix.Language == core.LanguageCodeGo
}
