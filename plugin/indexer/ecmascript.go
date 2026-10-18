package indexer

import (
	"maps"
	"strings"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/pkg/ts"
	"github.com/safedep/code/plugin/callgraph"
	sitter "github.com/smacker/go-tree-sitter"
)

var javascriptRules = ecmascriptRules(false)

var typescriptRules = ecmascriptRules(true)

// ecmascriptRules returns the rules of JavaScript, or of TypeScript with its
// interfaces and its declared types.
func ecmascriptRules(typescript bool) languageRules {
	r := languageRules{
		kinds: map[string]fileindex.DefKind{
			"class_declaration":              fileindex.DefClass,
			"class":                          fileindex.DefClass,
			"function_declaration":           fileindex.DefFunction,
			"generator_function_declaration": fileindex.DefFunction,
			"function_expression":            fileindex.DefFunction,
			"arrow_function":                 fileindex.DefFunction,
			"method_definition":              fileindex.DefMethod,
		},
		constructor: "constructor",
		bases:       ecmascriptBases,
		decorators:  decorators,
		exported:    ecmascriptExported,
		assigned:    ecmascriptAssigned,
		exports:     ecmascriptExports,
		visitors:    []func(p *projection) visitor{bodyVisitor, flagVisitor, referenceVisitor},
		dynamic: &dynamicSyntax{
			call:           "call_expression",
			assignment:     "assignment_expression",
			subscript:      "subscript_expression",
			subscriptIndex: "index",
			members:        map[string]string{"member_expression": "object", "subscript_expression": "object"},
			wrappers:       map[string]bool{"parenthesized_expression": true, "as_expression": true, "non_null_expression": true},
			eval:           map[string]bool{"eval": true, "Function": true},
			importers:      map[string]bool{"require": true},
			reflection:     func(callee string) bool { return strings.HasPrefix(callee, "Reflect.") },
			reflectionName: -1,
			staticTopLevel: true,
		},
		references: &referenceSyntax{
			holders: map[string]string{
				"arguments":                "",
				"array":                    "",
				"object":                   "",
				"pair":                     "value",
				"variable_declarator":      "value",
				"assignment_expression":    "right",
				"assignment_pattern":       "right",
				"return_statement":         "",
				"ternary_expression":       "",
				"binary_expression":        "",
				"parenthesized_expression": "",
				"export_statement":         "value",
			},
			member:      "member_expression",
			object:      "object",
			property:    "property",
			identifiers: map[string]bool{"identifier": true, "shorthand_property_identifier": true},
			instances:   map[string]bool{"this": true},
			logical:     map[string]bool{"||": true, "??": true, "&&": true},
		},
		arguments: &argumentSyntax{
			calls:    map[string]string{"call_expression": "function", "new_expression": "constructor"},
			skipped:  map[string]bool{"comment": true},
			strings:  map[string]bool{"string": true, "template_string": true},
			literals: map[string]bool{"number": true, "true": true, "false": true, "null": true, "undefined": true, "array": true, "object": true, "regex": true},
		},
	}
	if typescript {
		r.kinds = maps.Clone(r.kinds)
		r.kinds["abstract_class_declaration"] = fileindex.DefClass
		r.fields = typescriptFields
		r.definitions = typescriptInterfaces
		r.afterWalk = typescriptDeclaredTypes
	}
	return r
}

// ecmascriptBases returns the bases of a class from its class heritage, as
// Base in class A extends Base, and the interfaces that a TypeScript
// interface extends.
func ecmascriptBases(p *projection, node *sitter.Node) []string {
	switch node.Type() {
	case "class_declaration", "class":
		return p.heritage(node)
	case "interface_declaration":
		return p.interfaceExtends(node)
	}
	return nil
}

// heritage returns the bases that the class_heritage of a class names, as
// Base in class A extends Base.
func (p *projection) heritage(class *sitter.Node) []string {
	var bases []string
	for _, child := range ts.NamedChildren(class) {
		if child.Type() != "class_heritage" {
			continue
		}
		for _, base := range ts.NamedChildren(child) {
			// TypeScript wraps the bases in an extends clause and an
			// implements clause.
			switch base.Type() {
			case "extends_clause", "implements_clause":
				for _, name := range ts.NamedChildren(base) {
					if name.Type() != "type_arguments" {
						bases = append(bases, p.text(name))
					}
				}
			default:
				bases = append(bases, p.text(base))
			}
		}
	}
	return bases
}

// interfaceExtends returns the interfaces that a TypeScript interface
// extends.
func (p *projection) interfaceExtends(node *sitter.Node) []string {
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

// ecmascriptExported reports that an export names the definition. A method
// is exported with its class.
func ecmascriptExported(p *projection, def *fileindex.Definition) bool {
	if def.Parent != "" {
		return p.exported(p.defs[def.Parent])
	}
	return p.exports[def.Key]
}

// ecmascriptAssigned returns the names that a top-level declaration binds,
// as x in const x = f(), with or without an export.
func ecmascriptAssigned(_ *projection, statement *sitter.Node) []*sitter.Node {
	if statement.Type() == "export_statement" {
		if statement = statement.ChildByFieldName("declaration"); statement == nil {
			return nil
		}
	}
	var names []*sitter.Node
	switch statement.Type() {
	case "lexical_declaration", "variable_declaration":
		for _, declarator := range ts.NamedChildren(statement) {
			names = append(names, declarator.ChildByFieldName("name"))
		}
	}
	return names
}

// ecmascriptExports adds the exports of a JavaScript or TypeScript file: its
// export statements and its CommonJS assignments.
func ecmascriptExports(p *projection) {
	for _, statement := range ts.NamedChildren(p.tree.Tree().RootNode()) {
		switch {
		case statement.Type() == "export_statement":
			p.exportStatements = append(p.exportStatements, statement)
			p.addExportStatement(statement)
		case statement.Type() == "expression_statement" && statement.NamedChildCount() > 0:
			p.addExportAssignment(statement, statement.NamedChild(0))
		}
	}
}

// typescriptInterfaces adds each top-level TypeScript interface as a
// definition and a class with the interfaces that it extends. The call graph
// has no node for a type, so the tree gives the interfaces.
func typescriptInterfaces(p *projection) {
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
		p.defs[key] = &fileindex.Definition{
			Key:       key,
			Kind:      fileindex.DefInterface,
			Name:      fileindex.Last(key),
			StartLine: lineOf(statement),
			EndLine:   int(statement.EndPoint().Row) + 1,
		}
		p.addSyntax(key, statement)
	}
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
func bodyVisitor(p *projection) visitor {
	return func(node *sitter.Node, kind string) {
		var name, value *sitter.Node
		var holder []string
		switch kind {
		case "arguments":
			for _, arg := range ts.NamedChildren(node) {
				p.addArgumentFunction(arg)
			}
			return
		case "export_statement":
			p.addDefaultFunction(node.ChildByFieldName("value"))
			return
		case "pair":
			name, value = node.ChildByFieldName("key"), node.ChildByFieldName("value")
			holder = callgraph.ObjectNames(node.Parent(), p.source)
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
			value = node.ChildByFieldName("right")
			if p.text(left) == "module.exports" {
				p.addDefaultFunction(value)
				return
			}
			name = left.ChildByFieldName("property")
			holder = callgraph.ObjectNames(left.ChildByFieldName("object"), p.source)
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
		key := fileindex.Join(p.innermost(value.StartByte()), append(holder, p.text(name))...)
		if _, defined := p.defs[key]; !defined {
			p.addBody(key, fileindex.DefMethod, value)
		}
	}
}

// addDefaultFunction adds the function of module.exports = function () {}
// or of export default function () {} as the definition default.
func (p *projection) addDefaultFunction(value *sitter.Node) {
	key := p.local(defaultExport)
	if value == nil || !functionValues[value.Type()] || p.defined(value) || p.defs[key] != nil {
		return
	}
	p.addBody(key, fileindex.DefFunction, value)
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
		key = fileindex.Join(scope, fileindex.Anonymous(lineOf(value), n))
	}
	p.addBody(key, fileindex.DefFunction, value)
}

// typescriptDeclaredTypes adds the declared class types of the parameters
// of each TypeScript function. It runs after the walk of the tree, so a
// function that an argument holds gets its parameters too.
func typescriptDeclaredTypes(p *projection) {
	for key, def := range p.defs {
		node := p.syntax[key]
		if node == nil {
			continue
		}
		switch def.Kind {
		case fileindex.DefFunction, fileindex.DefMethod, fileindex.DefConstructor:
			def.Params = p.params(node)
		}
	}
}

// params returns the parameters of a function that declare a type name.
func (p *projection) params(function *sitter.Node) []fileindex.Typed {
	parameters := function.ChildByFieldName("parameters")
	if parameters == nil {
		return nil
	}
	var typed []fileindex.Typed
	for _, param := range ts.NamedChildren(parameters) {
		if t, ok := p.typedParam(param); ok {
			typed = append(typed, t)
		}
	}
	return typed
}

func (p *projection) typedParam(param *sitter.Node) (fileindex.Typed, bool) {
	switch param.Type() {
	case "required_parameter", "optional_parameter":
	default:
		return fileindex.Typed{}, false
	}
	pattern := param.ChildByFieldName("pattern")
	if pattern == nil || pattern.Type() != "identifier" {
		return fileindex.Typed{}, false
	}
	name := p.typeName(param.ChildByFieldName("type"))
	return fileindex.Typed{Name: p.text(pattern), Type: name}, name != ""
}

// typescriptFields returns the fields of a class that declare a type name:
// the instance fields of the class body and the parameter properties of the
// constructor.
func typescriptFields(p *projection, class *sitter.Node) []fileindex.Typed {
	body := class.ChildByFieldName("body")
	if body == nil {
		return nil
	}
	var fields []fileindex.Typed
	for _, member := range ts.NamedChildren(body) {
		switch member.Type() {
		case "public_field_definition":
			name := member.ChildByFieldName("name")
			typ := p.typeName(member.ChildByFieldName("type"))
			if name != nil && typ != "" && !hasKeyword(member, "static") {
				fields = append(fields, fileindex.Typed{Name: p.text(name), Type: typ})
			}
		case "method_definition":
			if p.text(member.ChildByFieldName("name")) != p.rules().constructor {
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
