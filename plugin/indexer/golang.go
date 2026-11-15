package indexer

import (
	"slices"
	"unicode"
	"unicode/utf8"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/pkg/ts"
	sitter "github.com/smacker/go-tree-sitter"
)

var goRules = languageRules{
	kinds: map[string]fileindex.DefKind{
		"function_declaration": fileindex.DefFunction,
		"method_declaration":   fileindex.DefMethod,
	},
	packageOf:   goPackage,
	bases:       goBases,
	fields:      goFields,
	abstract:    goMethodElems,
	signature:   goSignature,
	exported:    goExported,
	definitions: goDefinitions,
	typedAt:     goTypedCall,
	visitors:    []func(p *projection) visitor{referenceVisitor},
	references: &referenceSyntax{
		holders: map[string]string{
			"argument_list":         "",
			"short_var_declaration": "right",
			"assignment_statement":  "right",
			"var_spec":              "value",
			"return_statement":      "",
			"literal_value":         "",
			"send_statement":        "value",
			// A range over a function calls it, as walk in
			// for x := range walk.
			"range_clause": "right",
			// A conversion, as (func())(f), or a call of a generic
			// function with type arguments, as id[func()](f), which the
			// grammar parses as a conversion.
			"type_conversion_expression": "operand",
		},
		member:      "selector_expression",
		object:      "operand",
		property:    "field",
		identifiers: map[string]bool{"identifier": true},
		lists:       map[string]bool{"expression_list": true, "literal_element": true, "keyed_element": true},
		value:       goReference,
	},
	arguments: &argumentSyntax{
		calls:    map[string]string{"call_expression": "function"},
		skipped:  map[string]bool{"comment": true},
		strings:  map[string]bool{"interpreted_string_literal": true, "raw_string_literal": true},
		literals: map[string]bool{"int_literal": true, "float_literal": true, "imaginary_literal": true, "rune_literal": true, "true": true, "false": true, "nil": true, "composite_literal": true},
	},
}

// goPackage returns the name of the package clause of a Go file.
func goPackage(p *projection) string {
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

// goDefinitions adds the top-level named types of a Go file with the fields
// of each struct, the constants and the package variables. The call graph
// has no node for them, so the tree gives them.
func goDefinitions(p *projection) {
	var specs []*sitter.Node
	for _, statement := range ts.NamedChildren(p.tree.Tree().RootNode()) {
		switch statement.Type() {
		case "type_declaration":
			p.goTypes(statement)
		case "const_declaration":
			for _, spec := range ts.NamedChildren(statement) {
				if spec.Type() == "const_spec" {
					for _, name := range goNames(spec) {
						p.goValue(name, spec, fileindex.DefConstant)
					}
				}
			}
		case "var_declaration":
			specs = append(specs, goVarSpecs(statement)...)
		}
	}
	vars := make([][]*fileindex.Definition, len(specs))
	for i, spec := range specs {
		for _, name := range goNames(spec) {
			vars[i] = append(vars[i], p.goValue(name, spec, fileindex.DefVariable))
		}
	}
	// A value can name a variable that the file declares after it.
	for i, spec := range specs {
		p.goVariableValues(spec, vars[i])
	}
}

// goTypes adds each named type of a type declaration. An interface is an
// interface. Any other named type is a class, because it can have methods.
// A named field of a struct is a field of its class.
func (p *projection) goTypes(statement *sitter.Node) {
	for _, spec := range ts.NamedChildren(statement) {
		if spec.Type() != "type_spec" && spec.Type() != "type_alias" {
			continue
		}
		key := p.local(p.text(spec.ChildByFieldName("name")))
		if _, defined := p.defs[key]; defined {
			continue
		}
		kind := fileindex.DefClass
		t := spec.ChildByFieldName("type")
		if t != nil && t.Type() == "interface_type" {
			kind = fileindex.DefInterface
		}
		p.addDefinition(key, kind, spec)
		p.addSyntax(key, spec)
		if t == nil || t.Type() != "struct_type" || spec.Type() != "type_spec" {
			continue
		}
		for _, field := range p.goStructFields(t) {
			for _, name := range childrenByField(field, "name") {
				p.addDefinition(fileindex.Join(key, p.text(name)), fileindex.DefField, field)
			}
		}
	}
}

// goVarSpecs returns the specs of a var declaration. A declaration with
// parentheses holds them in a list.
func goVarSpecs(statement *sitter.Node) []*sitter.Node {
	var specs []*sitter.Node
	for _, child := range ts.NamedChildren(statement) {
		switch child.Type() {
		case "var_spec":
			specs = append(specs, child)
		case "var_spec_list":
			specs = append(specs, goVarSpecs(child)...)
		}
	}
	return specs
}

// goNames returns the names that a const or a var spec declares. The commas
// between them have the name field too.
func goNames(spec *sitter.Node) []*sitter.Node {
	return slices.DeleteFunc(childrenByField(spec, "name"), func(n *sitter.Node) bool { return n.Type() != "identifier" })
}

// goValue adds a constant or a package variable of a spec, and returns it.
// The blank name _ declares nothing, and a name that the file defines
// already keeps its definition. Both give nil.
func (p *projection) goValue(name, spec *sitter.Node, kind fileindex.DefKind) *fileindex.Definition {
	key := p.local(p.text(name))
	if _, defined := p.defs[key]; defined || p.text(name) == "_" {
		return nil
	}
	return p.addDefinition(key, kind, spec)
}

// goVariableValues fills each package variable of a spec: the per-file
// target of a value that names a function or another value, as run for
// var Run = run, or the Data mark when the source shows that the value is
// no function. A variable with neither can hold a function that the index
// does not know, as the value of a function literal or of a call, so a
// call of it keeps its record. defs holds the definition that goValue
// added for each name of the spec, or nil.
func (p *projection) goVariableValues(spec *sitter.Node, defs []*fileindex.Definition) {
	var values []*sitter.Node
	if list := spec.ChildByFieldName("value"); list != nil {
		values = ts.NamedChildren(list)
	}
	typed := spec.ChildByFieldName("type")
	for i, def := range defs {
		if def == nil {
			continue
		}
		var value *sitter.Node
		if len(values) == len(defs) {
			value = values[i]
		}
		switch {
		case typed != nil && p.goDataType(typed):
			def.Data = true
		case value == nil:
		case p.goDataValue(value):
			def.Data = true
		default:
			if target, ok := p.goValueTarget(value); ok {
				def.Values = []string{target}
			}
		}
	}
}

// goDataType reports whether a value of the type is no function: a
// predeclared type, as int or error, or a type literal that is not a
// function type, as *T or []T. A named type can be a function type, as
// http.HandlerFunc.
func (p *projection) goDataType(t *sitter.Node) bool {
	switch t.Type() {
	case "type_identifier":
		return p.goPredeclared(p.text(t))
	case "parenthesized_type":
		return t.NamedChildCount() == 1 && p.goDataType(t.NamedChild(0))
	case "pointer_type", "slice_type", "array_type", "map_type", "channel_type", "struct_type", "interface_type":
		return true
	}
	return false
}

// goErrorConstructors are the functions of the standard library that give
// an error value, by import path and name.
var goErrorConstructors = map[[2]string]bool{{"errors", "New"}: true, {"fmt", "Errorf"}: true}

// goDataValue reports whether the source shows that a value is no
// function: a literal, a composite literal, an arithmetic expression, a
// conversion to a type of data, a call of a builtin, or an error that
// errors.New or fmt.Errorf gives.
func (p *projection) goDataValue(value *sitter.Node) bool {
	switch value.Type() {
	case "interpreted_string_literal", "raw_string_literal", "int_literal", "float_literal", "imaginary_literal", "rune_literal",
		"true", "false", "iota", "composite_literal", "binary_expression":
		return true
	case "parenthesized_expression":
		return value.NamedChildCount() == 1 && p.goDataValue(value.NamedChild(0))
	case "unary_expression":
		// A dereference of a pointer to a function or a receive from a
		// channel of functions gives a function.
		op := p.text(value.ChildByFieldName("operator"))
		return op != "*" && op != "<-"
	case "call_expression":
		function := value.ChildByFieldName("function")
		if function == nil {
			return false
		}
		switch function.Type() {
		case "identifier":
			return p.goPredeclared(p.text(function))
		case "selector_expression":
			operand, field := function.ChildByFieldName("operand"), function.ChildByFieldName("field")
			return operand != nil && field != nil && goErrorConstructors[[2]string{p.goImportPath(p.text(operand)), p.text(field)}]
		}
		return p.goDataType(function)
	}
	return false
}

// goImportPath returns the import path of the import that binds the alias.
func (p *projection) goImportPath(alias string) string {
	for _, imp := range p.ix.Imports {
		if imp.Alias == alias {
			return imp.Module
		}
	}
	return ""
}

// goValueTarget returns the per-file target of a value that names a
// function or another value, in each form that a reference reads.
func (p *projection) goValueTarget(value *sitter.Node) (string, bool) {
	target, _ := goReference(p, p.cg.FileName, value)
	return target, target != ""
}

// goBases returns the types that a Go type embeds, and the type that an
// alias names. A method of such a type is a method of the type too.
func goBases(p *projection, spec *sitter.Node) []string {
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

// goMethodElems returns the methods that a Go interface declares.
func goMethodElems(p *projection, spec *sitter.Node) []fileindex.Signature {
	t := spec.ChildByFieldName("type")
	if t == nil || t.Type() != "interface_type" {
		return nil
	}
	var sigs []fileindex.Signature
	for _, elem := range ts.NamedChildren(t) {
		if elem.Type() != "method_elem" {
			continue
		}
		_, arity, variadic := p.goParams(elem.ChildByFieldName("parameters"), nil)
		sigs = append(sigs, fileindex.Signature{Name: p.text(elem.ChildByFieldName("name")), Arity: arity, Variadic: variadic})
	}
	return sigs
}

// goFields returns the named fields of a Go struct that declare a type name.
func goFields(p *projection, spec *sitter.Node) []fileindex.Typed {
	t := spec.ChildByFieldName("type")
	if t == nil || t.Type() != "struct_type" || spec.Type() != "type_spec" {
		return nil
	}
	var fields []fileindex.Typed
	for _, field := range p.goStructFields(t) {
		typ := p.goTypeName(field.ChildByFieldName("type"))
		if typ == "" {
			continue
		}
		for _, name := range childrenByField(field, "name") {
			fields = append(fields, fileindex.Typed{Name: p.text(name), Type: typ})
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

// goSignature adds the receiver, the parameters, the arity, the first
// result and the typed local variables of a Go function or method.
func goSignature(p *projection, def *fileindex.Definition, node *sitter.Node) {
	if def.Kind != fileindex.DefFunction && def.Kind != fileindex.DefMethod {
		return
	}
	if receiver := node.ChildByFieldName("receiver"); receiver != nil {
		def.Params, _, _ = p.goParams(receiver, nil)
	}
	var params []fileindex.Typed
	params, def.Arity, def.Variadic = p.goParams(node.ChildByFieldName("parameters"), p.goConstraints(node))
	def.Params = append(def.Params, params...)
	results := p.goResults(node.ChildByFieldName("result"))
	if len(results) > 0 {
		def.Returns = results[0]
	}
	if len(results) > 1 {
		def.Results = results
	}
	def.Locals, p.goOrigins[def.Key] = p.goLocals(node.ChildByFieldName("body"))
}

// goExported reports that a name starts with an upper case letter, a
// method too.
func goExported(_ *projection, def *fileindex.Definition) bool {
	r, _ := utf8.DecodeRuneInString(def.Name)
	return unicode.IsUpper(r)
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
func (p *projection) goParams(list *sitter.Node, constraints map[string]string) ([]fileindex.Typed, int, bool) {
	if list == nil {
		return nil, 0, false
	}
	var typed []fileindex.Typed
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
			typed = append(typed, fileindex.Typed{Name: p.text(name), Type: typ})
		}
	}
	return typed, arity, variadic
}

// goResults returns the type name of each result of a function, by
// position. A result with no named type, as int or func(), gives "".
func (p *projection) goResults(result *sitter.Node) []string {
	if result == nil {
		return nil
	}
	if result.Type() != "parameter_list" {
		return []string{p.goTypeName(result)}
	}
	var out []string
	for _, param := range ts.NamedChildren(result) {
		if param.Type() != "parameter_declaration" {
			continue
		}
		typ := p.goTypeName(param.ChildByFieldName("type"))
		for range max(len(childrenByField(param, "name")), 1) {
			out = append(out, typ)
		}
	}
	return out
}

// goLocals returns the local variables of a function body whose type the
// source declares, as var p Parser, or constructs, as p := &Parser{},
// p := new(Parser) or p := v.(*Parser). A name that holds the first result
// of a call, as p := NewParser(), names the function. A function literal in
// the body shares the scope of the body, so its names count too. It also
// returns the origin of each local: the start of its scope as goScopes
// records it, or goAssignedOrigin for an assignment.
func (p *projection) goLocals(body *sitter.Node) ([]fileindex.Typed, []uint32) {
	var locals []fileindex.Typed
	var origins []uint32
	add := func(typed []fileindex.Typed, origin uint32) {
		locals = append(locals, typed...)
		for range typed {
			origins = append(origins, origin)
		}
	}
	walkBody(body, nil, func(node *sitter.Node) {
		switch node.Type() {
		case "var_spec":
			names := childrenByField(node, "name")
			if typ := p.goTypeName(node.ChildByFieldName("type")); typ != "" {
				for _, name := range names {
					add([]fileindex.Typed{{Name: p.text(name), Type: typ}}, node.EndByte())
				}
			} else {
				add(p.goAssigned(names, node.ChildByFieldName("value")), node.EndByte())
			}
		case "short_var_declaration", "assignment_statement":
			origin := node.EndByte()
			if node.Type() == "assignment_statement" {
				origin = goAssignedOrigin
			}
			if left := node.ChildByFieldName("left"); left != nil {
				add(p.goAssigned(ts.NamedChildren(left), node.ChildByFieldName("right")), origin)
			}
		}
	})
	return locals, origins
}

// goAssigned returns the typed names of an assignment. One call on the
// right of more names gives each name the result at its position, and the
// blank name _ takes none.
func (p *projection) goAssigned(names []*sitter.Node, right *sitter.Node) []fileindex.Typed {
	if right == nil {
		return nil
	}
	values := ts.NamedChildren(right)
	if len(values) == 1 && len(names) > 1 && values[0].Type() == "call_expression" {
		t, ok := p.goValueType(values[0])
		if !ok || !t.Result {
			return nil
		}
		var typed []fileindex.Typed
		for i, name := range names {
			if name.Type() == "identifier" && p.text(name) != "_" {
				typed = append(typed, fileindex.Typed{Name: p.text(name), Type: t.Type, Result: true, Index: i})
			}
		}
		return typed
	}
	if len(values) != len(names) {
		return nil
	}
	var typed []fileindex.Typed
	for i, name := range names {
		if name.Type() != "identifier" || p.text(name) == "_" {
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
func (p *projection) goValueType(value *sitter.Node) (fileindex.Typed, bool) {
	switch value.Type() {
	case "unary_expression":
		if operand := value.ChildByFieldName("operand"); operand != nil && operand.Type() == "composite_literal" {
			return p.goValueType(operand)
		}
	case "composite_literal", "type_assertion_expression":
		if typ := p.goTypeName(value.ChildByFieldName("type")); typ != "" {
			return fileindex.Typed{Type: typ}, true
		}
	case "type_conversion_expression":
		// A call of a generic function with type arguments, as
		// NewBox[int](1), parses as a conversion.
		if t := value.ChildByFieldName("type"); t != nil && t.Type() == "generic_type" {
			if function := t.ChildByFieldName("type"); function != nil {
				return fileindex.Typed{Type: p.text(function), Result: true}, true
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
				return fileindex.Typed{Type: typ}, true
			}
			break
		}
		switch function.Type() {
		case "identifier", "selector_expression":
			return fileindex.Typed{Type: p.text(function), Result: true}, true
		}
	}
	return fileindex.Typed{}, false
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
	for scope := caller; fileindex.HasKeyPrefix(scope, p.cg.FileName); scope = fileindex.Parent(scope) {
		if def, ok := p.defs[scope]; ok {
			for _, typed := range [][]fileindex.Typed{def.Params, def.Locals} {
				for _, t := range typed {
					if t.Name == name {
						return scope, true
					}
				}
			}
		}
		if scope == p.cg.FileName {
			break
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
