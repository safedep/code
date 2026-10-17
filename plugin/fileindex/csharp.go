package fileindex

import (
	"cmp"
	"slices"

	fidx "github.com/safedep/code/fileindex"
	"github.com/safedep/code/pkg/ts"
	sitter "github.com/smacker/go-tree-sitter"
)

// csharpReflection name the calls that look up a type or a member by a
// string or make an instance with no new.
// GetType is one only on Type, because GetType() of an object is common.
var csharpReflection = []string{"GetMethod", "GetMethods", "GetProperty", "InvokeMember", "CreateInstance", "LoadFrom", "LoadFile"}

// csharpTopLevel names the definition that holds the top-level statements
// of a C# file, as the compiler names the method that it makes of them.
const csharpTopLevel = "<Main>$"

var csharpRules = languageRules{
	packageOf:    csharpNamespace,
	bases:        csharpBases,
	fields:       csharpFields,
	decorators:   csharpAttributes,
	signature:    csharpSignature,
	definitions:  csharpTopLevelStatements,
	visitors:     []func(p *projection) visitor{csharpFlagVisitor},
	memberScopes: true,
}

// csharpNamespace returns the name of the first namespace of the file. A
// file of more than one namespace keeps the first one.
func csharpNamespace(p *projection) string {
	for _, statement := range ts.NamedChildren(p.tree.Tree().RootNode()) {
		if statement.Type() == "namespace_declaration" || statement.Type() == "file_scoped_namespace_declaration" {
			return p.text(statement.ChildByFieldName("name"))
		}
	}
	return ""
}

// csharpTypeName returns the name of the class type that a type node names,
// as Store for Store, Store? or Store<T>, and App.Store for a qualified
// name. A predefined type, an array and var name no class.
func (p *projection) csharpTypeName(node *sitter.Node) string {
	if node == nil {
		return ""
	}
	switch node.Type() {
	case "identifier":
		if name := p.text(node); name != "var" {
			return name
		}
	case "qualified_name":
		return withoutTypeArgs(p.text(node))
	case "generic_name":
		return p.text(childOfType(node, "identifier"))
	case "nullable_type":
		return p.csharpTypeName(node.ChildByFieldName("type"))
	}
	return ""
}

func csharpBases(p *projection, class *sitter.Node) []string {
	list := childOfType(class, "base_list")
	if list == nil {
		return nil
	}
	var bases []string
	for _, base := range ts.NamedChildren(list) {
		if base.Type() == "primary_constructor_base_type" {
			base = base.NamedChild(0)
		}
		bases = append(bases, nonEmpty(p.csharpTypeName(base))...)
	}
	return bases
}

// csharpFields returns the fields and the properties of a class that
// declare a class type, and the parameters of a primary constructor.
func csharpFields(p *projection, class *sitter.Node) []fidx.Typed {
	fields := p.csharpParams(childOfType(class, "parameter_list"))
	body := class.ChildByFieldName("body")
	if body == nil {
		return fields
	}
	for _, member := range ts.NamedChildren(body) {
		switch member.Type() {
		case "field_declaration":
			fields = append(fields, p.csharpVariables(childOfType(member, "variable_declaration"))...)
		case "property_declaration":
			if typ := p.csharpTypeName(member.ChildByFieldName("type")); typ != "" {
				fields = append(fields, fidx.Typed{Name: p.text(member.ChildByFieldName("name")), Type: typ})
			}
		}
	}
	return fields
}

// csharpVariables returns the names of a variable declaration that declare
// a class type, or that a new of a class initializes.
func (p *projection) csharpVariables(declaration *sitter.Node) []fidx.Typed {
	if declaration == nil {
		return nil
	}
	typ := p.csharpTypeName(declaration.ChildByFieldName("type"))
	var out []fidx.Typed
	for _, declarator := range ts.NamedChildren(declaration) {
		if declarator.Type() != "variable_declarator" {
			continue
		}
		t := typ
		if creation := childOfType(declarator, "object_creation_expression"); t == "" && creation != nil {
			t = p.csharpTypeName(creation.ChildByFieldName("type"))
		}
		if t != "" {
			out = append(out, fidx.Typed{Name: p.text(declarator.ChildByFieldName("name")), Type: t})
		}
	}
	return out
}

// csharpParams returns the parameters that declare a class type. The
// receiver of an extension method, as s in this string s, is the parameter
// this with the type as written.
func (p *projection) csharpParams(list *sitter.Node) []fidx.Typed {
	if list == nil {
		return nil
	}
	var out []fidx.Typed
	for i, param := range ts.NamedChildren(list) {
		if param.Type() != "parameter" {
			continue
		}
		typ := p.csharpTypeName(param.ChildByFieldName("type"))
		if i == 0 && slices.ContainsFunc(ts.NamedChildren(param), func(m *sitter.Node) bool { return m.Type() == "modifier" && p.text(m) == "this" }) {
			out = append(out, fidx.Typed{Name: extensionParam, Type: cmp.Or(typ, p.text(param.ChildByFieldName("type")))})
			continue
		}
		if typ != "" {
			out = append(out, fidx.Typed{Name: p.text(param.ChildByFieldName("name")), Type: typ})
		}
	}
	return out
}

// csharpAttributes returns the attributes of a declaration as written, with
// no arguments and an @ for the brackets, as @HttpGet for [HttpGet("/a")].
func csharpAttributes(p *projection, node *sitter.Node) []string {
	var attributes []string
	for _, list := range ts.NamedChildren(node) {
		if list.Type() != "attribute_list" {
			continue
		}
		for _, a := range ts.NamedChildren(list) {
			if a.Type() == "attribute" {
				attributes = append(attributes, "@"+withoutTypeArgs(p.text(a.ChildByFieldName("name"))))
			}
		}
	}
	return attributes
}

// csharpSignature sets the static mark, the parameters, the typed locals and
// the result of a method, a constructor or a local function.
func csharpSignature(p *projection, def *fidx.Definition, node *sitter.Node) {
	switch {
	case def.Name == staticInitName || def.Name == csharpTopLevel:
		def.Static = true
		return
	case node.Type() != "method_declaration" && node.Type() != "constructor_declaration" && node.Type() != "local_function_statement":
		return
	}
	def.Static = csharpStatic(node)
	def.Params = p.csharpParams(node.ChildByFieldName("parameters"))
	def.Returns = p.csharpTypeName(node.ChildByFieldName("returns"))
	def.Locals = p.csharpLocals(node.ChildByFieldName("body"))
}

func csharpStatic(node *sitter.Node) bool {
	return slices.ContainsFunc(ts.NamedChildren(node), func(child *sitter.Node) bool {
		return child.Type() == "modifier" && hasKeyword(child, "static")
	})
}

// csharpLocals returns the local variables of a body that declare a class
// type or that a new initializes, and the variable of a foreach.
func (p *projection) csharpLocals(body *sitter.Node) []fidx.Typed {
	var locals []fidx.Typed
	var walk func(node *sitter.Node, depth int)
	walk = func(node *sitter.Node, depth int) {
		switch {
		case depth > maxLocalDepth:
			return
		case node.Type() == "variable_declaration":
			locals = append(locals, p.csharpVariables(node)...)
		case node.Type() == "foreach_statement":
			if typ := p.csharpTypeName(node.ChildByFieldName("type")); typ != "" {
				locals = append(locals, fidx.Typed{Name: p.text(node.ChildByFieldName("left")), Type: typ})
			}
		case node.Type() == "local_function_statement" && depth > 0:
			return
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

// csharpTopLevelStatements adds the definition <Main>$ that runs the
// top-level statements of a file, which the compiler makes the entry point.
// It takes the calls of the statements. A local function of the top level
// stays a definition of its own.
func csharpTopLevelStatements(p *projection) {
	var statements []*sitter.Node
	for _, statement := range ts.NamedChildren(p.tree.Tree().RootNode()) {
		if statement.Type() == "global_statement" && childOfType(statement, "local_function_statement") == nil {
			statements = append(statements, statement)
		}
	}
	if len(statements) == 0 {
		return
	}
	key := p.local(csharpTopLevel)
	p.defs[key] = &fidx.Definition{
		Key:       key,
		Kind:      fidx.DefFunction,
		Name:      csharpTopLevel,
		StartLine: lineOf(statements[0]),
		EndLine:   int(statements[len(statements)-1].EndPoint().Row) + 1,
		Static:    true,
	}
	for _, statement := range statements {
		p.bodies[spanOf(statement)] = key
	}
}

// csharpFlagVisitor marks the reflection calls of a C# file.
func csharpFlagVisitor(p *projection) visitor {
	return func(node *sitter.Node, kind string) {
		if kind != "invocation_expression" {
			return
		}
		function := node.ChildByFieldName("function")
		if function == nil || function.Type() != "member_access_expression" {
			return
		}
		name := withoutTypeArgs(p.text(function.ChildByFieldName("name")))
		if slices.Contains(csharpReflection, name) || name == "GetType" && p.text(function.ChildByFieldName("expression")) == "Type" {
			p.flag(fidx.FlagReflection, node)
		}
	}
}
