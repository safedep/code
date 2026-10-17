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

// interfaceBases returns the interfaces that an interface extends.
func (p *projection) interfaceBases(node *sitter.Node) []string {
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

// propertyVisitor adds each JavaScript function that an object property, a
// property assignment or a class field holds, as go in { go() {} },
// { go: function () {} }, api.go = () => {} or go = () => {} in a class.
// The call graph names no such function, so its calls land on the scope
// that holds it. The definition has the property name in that scope, and
// it takes the calls of its body. A function value that already is a
// definition, or a key that a definition already holds, adds nothing.
//
// The visitor runs before the others of the walk, and it sees a property
// before the function of the property, so the flags and the references of
// the body land on the new definition.
func (p *projection) propertyVisitor() visitor {
	if !p.ix.Language.IsECMAScript() {
		return nil
	}
	return func(node *sitter.Node, kind string) {
		var name, value *sitter.Node
		switch kind {
		case "pair":
			name, value = node.ChildByFieldName("key"), node.ChildByFieldName("value")
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
		default:
			return
		}
		if name == nil || value == nil || !functionValues[value.Type()] {
			return
		}
		if _, defined := p.spans[spanOf(value)]; defined {
			return
		}
		switch name.Type() {
		case "property_identifier", "identifier", "private_property_identifier":
		default:
			return
		}
		key := fidx.Join(p.innermost(value.StartByte()), p.text(name))
		if _, defined := p.defs[key]; defined {
			return
		}
		def := &fidx.Definition{
			Key:       key,
			Kind:      fidx.DefMethod,
			Name:      p.text(name),
			StartLine: lineOf(value),
			EndLine:   int(value.EndPoint().Row) + 1,
		}
		p.defs[key] = def
		p.addSyntax(key, value)
		p.properties[spanOf(value)] = key
		p.fillDefinition(def)
	}
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

// fields returns the fields of a TypeScript class that declare a type name:
// the instance fields of the class body and the parameter properties of the
// constructor.
func (p *projection) fields(class *sitter.Node) []fidx.Typed {
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
