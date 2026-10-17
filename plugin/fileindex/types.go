package fileindex

import (
	"github.com/safedep/code/core"
	fidx "github.com/safedep/code/fileindex"
	"github.com/safedep/code/pkg/ts"
	sitter "github.com/smacker/go-tree-sitter"
)

// span identifies a tree node across lookups. The tree gives a new
// *sitter.Node on each lookup, so a pointer does not identify a node.
type span struct {
	start, end uint32
	kind       string
}

func spanOf(node *sitter.Node) span {
	return span{node.StartByte(), node.EndByte(), node.Type()}
}

// functionValues are the node types of a function that an expression holds.
var functionValues = map[string]bool{
	"func_literal":        true,
	"function_expression": true,
	"function":            true,
	"arrow_function":      true,
	"generator_function":  true,
}

// addSyntax records the tree node of a definition.
func (p *projection) addSyntax(key string, node *sitter.Node) {
	p.syntax[key] = node
	if node != nil {
		p.spans[spanOf(node)] = key
	}
}

// addInterfaces adds each top-level TypeScript interface as a definition and
// a class with the interfaces that it extends. The call graph has no node
// for a type, so the tree gives the interfaces.
func (p *projection) addInterfaces() {
	if p.ix.Language != core.LanguageCodeTypescript {
		return
	}
	for _, statement := range ts.NamedChildren(p.tree.Tree().RootNode()) {
		if statement.Type() == "export_statement" {
			statement = statement.ChildByFieldName("declaration")
		}
		if statement == nil || statement.Type() != "interface_declaration" {
			continue
		}
		key := p.local(p.text(statement.ChildByFieldName("name")))
		if _, defined := p.defs[key]; defined {
			continue
		}
		p.defs[key] = &fidx.Definition{
			Key:       key,
			Kind:      fidx.DefInterface,
			Name:      fidx.Last(key),
			StartLine: lineOf(statement),
			EndLine:   int(statement.EndPoint().Row) + 1,
		}
		p.addSyntax(key, statement)
	}
}

// interfaceBases returns the interfaces that an interface extends or, in
// Go, embeds.
func (p *projection) interfaceBases(node *sitter.Node) []string {
	switch {
	case p.isGo():
		return p.goBases(node)
	case p.isJava():
		return p.javaBases(node)
	}
	var bases []string
	for _, child := range ts.NamedChildren(node) {
		if child.Type() != "extends_type_clause" {
			continue
		}
		for _, base := range ts.NamedChildren(child) {
			if name := p.typeName(base); name != "" {
				bases = append(bases, name)
			}
		}
	}
	return bases
}

// bodyVisitor adds each JavaScript function that an object property, a
// property assignment, a class field or a call argument holds, as go in
// api = { go: function () {} }, api.go = () => {}, go = () => {} in a class
// or the handler in app.get('/x', (req, res) => {}). The call graph names no
// such function, so its calls land on the scope that holds it. The
// definition has the names of the object and the property in that scope,
// as api//go, and it takes the calls of its body. An argument has no name,
// so its definition gets an anonymous name, as <anonymous:12>. A function
// value that already is a definition, or a key that a definition already
// holds, adds nothing.
//
// The visitor runs before the others of the walk, and it sees a property or
// an argument list before the function that it holds, so the flags and the
// references of the body land on the new definition.
func (p *projection) bodyVisitor() visitor {
	if !p.ix.Language.IsECMAScript() {
		return nil
	}
	return func(node *sitter.Node, kind string) {
		var name, value *sitter.Node
		var holder []string
		switch kind {
		case "arguments":
			for _, arg := range ts.NamedChildren(node) {
				p.addArgumentFunction(arg)
			}
			return
		case "pair":
			name, value = node.ChildByFieldName("key"), node.ChildByFieldName("value")
			holder = p.objectNames(node.Parent(), 0)
		case "public_field_definition", "field_definition":
			name, value = node.ChildByFieldName("name"), node.ChildByFieldName("value")
			if name == nil {
				name = node.ChildByFieldName("property")
			}
		case "assignment_expression":
			left := node.ChildByFieldName("left")
			if left == nil || left.Type() != "member_expression" {
				return
			}
			name, value = left.ChildByFieldName("property"), node.ChildByFieldName("right")
			holder = p.objectNames(left.ChildByFieldName("object"), 0)
		default:
			return
		}
		if name == nil || value == nil || !functionValues[value.Type()] || p.defined(value) {
			return
		}
		switch name.Type() {
		case "property_identifier", "identifier", "private_property_identifier":
		default:
			return
		}
		key := fidx.Join(p.innermost(value.StartByte()), append(holder, p.text(name))...)
		if _, defined := p.defs[key]; !defined {
			p.addBody(key, fidx.DefMethod, value)
		}
	}
}

// addArgumentFunction adds a function literal that a call takes as an
// argument as a function with an anonymous name in the innermost
// definition. Two such functions on one line get an ordinal.
func (p *projection) addArgumentFunction(value *sitter.Node) {
	if !functionValues[value.Type()] || p.defined(value) {
		return
	}
	scope := p.innermost(value.StartByte())
	key := ""
	for n := 1; key == "" || p.defs[key] != nil; n++ {
		key = fidx.Join(scope, fidx.Anonymous(lineOf(value), n))
	}
	p.addBody(key, fidx.DefFunction, value)
}

// defined reports whether a tree node already is a definition.
func (p *projection) defined(node *sitter.Node) bool {
	_, ok := p.spans[spanOf(node)]
	return ok
}

// addBody adds a definition that the call graph does not know and that
// takes the calls of the node.
func (p *projection) addBody(key string, kind fidx.DefKind, node *sitter.Node) {
	def := &fidx.Definition{
		Key:       key,
		Kind:      kind,
		Name:      fidx.Last(key),
		StartLine: lineOf(node),
		EndLine:   int(node.EndPoint().Row) + 1,
	}
	p.defs[key] = def
	p.addSyntax(key, node)
	p.bodies[spanOf(node)] = key
	p.fillDefinition(def)
}

// objectNames returns the names that lead to an object, as [api] for the
// object of const api = {} and [api, routes] for api.routes. The module
// object of exports and module.exports, the prototype of a class and this
// add no name, so exports.go is the go of the module and A.prototype.go is
// the go of the class A. An object that no name holds, as an argument, adds
// no name.
func (p *projection) objectNames(object *sitter.Node, depth int) []string {
	if object == nil || depth > maxReferenceSegments {
		return nil
	}
	switch object.Type() {
	case "identifier":
		if name := p.text(object); name != "exports" {
			return []string{name}
		}
	case "member_expression":
		if p.text(object) == "module.exports" {
			return nil
		}
		names := p.objectNames(object.ChildByFieldName("object"), depth+1)
		if property := p.text(object.ChildByFieldName("property")); property != "prototype" {
			names = append(names, property)
		}
		return names
	case "object":
		switch parent := object.Parent(); {
		case parent == nil:
		case parent.Type() == "variable_declarator":
			if name := parent.ChildByFieldName("name"); name != nil && name.Type() == "identifier" {
				return []string{p.text(name)}
			}
		case parent.Type() == "pair":
			return append(p.objectNames(parent.Parent(), depth+1), p.text(parent.ChildByFieldName("key")))
		case parent.Type() == "assignment_expression":
			return p.objectNames(parent.ChildByFieldName("left"), depth+1)
		}
	}
	return nil
}

// innermost returns the key of the innermost definition that holds the
// byte, or the module key.
func (p *projection) innermost(at uint32) string {
	var inner *sitter.Node
	key := p.cg.FileName
	for k, node := range p.syntax {
		if node == nil || at < node.StartByte() || at >= node.EndByte() {
			continue
		}
		if inner == nil || node.EndByte()-node.StartByte() < inner.EndByte()-inner.StartByte() {
			inner, key = node, k
		}
	}
	return key
}

// addDeclaredTypes adds the declared class types of the parameters of each
// TypeScript function, and of the fields of each class. A constructor
// parameter with an accessibility modifier or readonly is a field too.
func (p *projection) addDeclaredTypes() {
	if p.ix.Language != core.LanguageCodeTypescript {
		return
	}
	for key, def := range p.defs {
		node := p.syntax[key]
		if node == nil {
			continue
		}
		switch def.Kind {
		case fidx.DefFunction, fidx.DefMethod, fidx.DefConstructor:
			def.Params = p.params(node)
		}
	}
}

// params returns the parameters of a function that declare a type name.
func (p *projection) params(function *sitter.Node) []fidx.Typed {
	parameters := function.ChildByFieldName("parameters")
	if parameters == nil {
		return nil
	}
	var typed []fidx.Typed
	for _, param := range ts.NamedChildren(parameters) {
		if t, ok := p.typedParam(param); ok {
			typed = append(typed, t)
		}
	}
	return typed
}

func (p *projection) typedParam(param *sitter.Node) (fidx.Typed, bool) {
	switch param.Type() {
	case "required_parameter", "optional_parameter":
	default:
		return fidx.Typed{}, false
	}
	pattern := param.ChildByFieldName("pattern")
	if pattern == nil || pattern.Type() != "identifier" {
		return fidx.Typed{}, false
	}
	name := p.typeName(param.ChildByFieldName("type"))
	return fidx.Typed{Name: p.text(pattern), Type: name}, name != ""
}

// fields returns the fields of a class that declare a type name. A
// TypeScript class has the instance fields of the class body and the
// parameter properties of the constructor. A Go struct has its named
// fields.
func (p *projection) fields(class *sitter.Node) []fidx.Typed {
	switch {
	case p.isGo():
		return p.goFields(class)
	case p.isJava():
		return p.javaFields(class)
	}
	body := class.ChildByFieldName("body")
	if body == nil || p.ix.Language != core.LanguageCodeTypescript {
		return nil
	}
	var fields []fidx.Typed
	for _, member := range ts.NamedChildren(body) {
		switch member.Type() {
		case "public_field_definition":
			name := member.ChildByFieldName("name")
			typ := p.typeName(member.ChildByFieldName("type"))
			if name != nil && typ != "" && !hasKeyword(member, "static") {
				fields = append(fields, fidx.Typed{Name: p.text(name), Type: typ})
			}
		case "method_definition":
			if p.text(member.ChildByFieldName("name")) != constructorNames[p.ix.Language] {
				continue
			}
			parameters := member.ChildByFieldName("parameters")
			if parameters == nil {
				continue
			}
			for _, param := range ts.NamedChildren(parameters) {
				if !parameterProperty(param) {
					continue
				}
				if t, ok := p.typedParam(param); ok {
					fields = append(fields, t)
				}
			}
		}
	}
	return fields
}

// parameterProperty reports whether a constructor parameter is also a field,
// as private store: Store.
func parameterProperty(param *sitter.Node) bool {
	for _, child := range ts.NamedChildren(param) {
		if child.Type() == "accessibility_modifier" || child.Type() == "override_modifier" {
			return true
		}
	}
	return hasKeyword(param, "readonly")
}

// typeName returns the name of the class type that a type node names, as
// Parser for Parser, Parser<T> or Parser | undefined, and ns.Parser for a
// nested name. A union names a class when one member names a class and the
// others are null, undefined or a predefined type.
func (p *projection) typeName(node *sitter.Node) string {
	if node == nil {
		return ""
	}
	switch node.Type() {
	case "type_annotation", "parenthesized_type":
		if node.NamedChildCount() == 0 {
			return ""
		}
		return p.typeName(node.NamedChild(0))
	case "type_identifier", "nested_type_identifier", "identifier":
		return p.text(node)
	case "generic_type":
		return p.typeName(node.ChildByFieldName("name"))
	case "union_type":
		name := ""
		for _, member := range ts.NamedChildren(node) {
			switch member.Type() {
			case "literal_type", "predefined_type":
				continue
			}
			if name != "" {
				return ""
			}
			if name = p.typeName(member); name == "" {
				return ""
			}
		}
		return name
	}
	return ""
}
