package indexer

import (
	"slices"
	"strings"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/pkg/ts"
	sitter "github.com/smacker/go-tree-sitter"
)

// extensionParam names the parameter that holds the receiver of an
// extension function, as the String of fun String.shout() in Kotlin or s of
// this string s in C#. No parameter can have the name, so the name marks an
// extension.
const extensionParam = "this"

var kotlinRules = languageRules{
	kinds: map[string]fileindex.DefKind{
		"class_declaration":     fileindex.DefClass,
		"object_declaration":    fileindex.DefClass,
		"function_declaration":  fileindex.DefFunction,
		"secondary_constructor": fileindex.DefConstructor,
	},
	packageOf:    kotlinPackage,
	kindOf:       kotlinKind,
	bases:        kotlinBases,
	fields:       kotlinFields,
	decorators:   kotlinAnnotations,
	signature:    kotlinSignature,
	definitions:  kotlinProperties,
	visitors:     []func(p *projection) visitor{kotlinReferenceVisitor, kotlinFlagVisitor},
	memberScopes: true,
}

// kotlinProperties adds each top-level property as a variable with no
// value. Its initializer runs when the class of the file loads, and its
// value is often the result of a call, which the index does not know. A
// call on it is then a record and not an edge to the callee of the call.
func kotlinProperties(p *projection) {
	for _, statement := range ts.NamedChildren(p.tree.Tree().RootNode()) {
		if statement.Type() != "property_declaration" {
			continue
		}
		name := ts.ChildOfType(ts.ChildOfType(statement, "variable_declaration"), "simple_identifier")
		key := p.local(p.text(name))
		if _, defined := p.defs[key]; name == nil || defined {
			continue
		}
		p.defs[key] = &fileindex.Definition{
			Key:       key,
			Kind:      fileindex.DefVariable,
			Name:      p.text(name),
			StartLine: lineOf(statement),
			EndLine:   int(statement.EndPoint().Row) + 1,
		}
	}
}

func kotlinPackage(p *projection) string {
	header := ts.ChildOfType(p.tree.Tree().RootNode(), "package_header")
	if header == nil {
		return ""
	}
	return p.text(ts.ChildOfType(header, "identifier"))
}

// kotlinKind tells an interface from a class. Both are class declarations.
func kotlinKind(_ *projection, node *sitter.Node, kind fileindex.DefKind) fileindex.DefKind {
	if node != nil && node.Type() == "class_declaration" && ts.HasKeyword(node, "interface") {
		return fileindex.DefInterface
	}
	return kind
}

// kotlinBases returns the class and the interfaces of the delegation
// specifiers of a class or an object, as Base and Runner in
// class A : Base(), Runner. An enum entry with a body extends its enum.
func kotlinBases(p *projection, class *sitter.Node) []string {
	if class.Type() == "enum_entry" {
		if enum := enumOf(class, "class_declaration"); enum != nil {
			return nonEmpty(p.text(ts.ChildOfType(enum, "type_identifier")))
		}
		return nil
	}
	var bases []string
	for _, spec := range ts.NamedChildren(class) {
		if spec.Type() != "delegation_specifier" || spec.NamedChildCount() == 0 {
			continue
		}
		typ := spec.NamedChild(0)
		if typ.Type() == "constructor_invocation" || typ.Type() == "explicit_delegation" {
			typ = ts.ChildOfType(typ, "user_type")
		}
		bases = append(bases, nonEmpty(p.kotlinTypeName(typ))...)
	}
	return bases
}

// kotlinTypeName returns the class name that a type names, as Parser for
// Parser<T> or Parser?, and com.x.Parser for a qualified name.
func (p *projection) kotlinTypeName(node *sitter.Node) string {
	if node == nil {
		return ""
	}
	switch node.Type() {
	case "nullable_type", "not_nullable_type", "parenthesized_type":
		return p.kotlinTypeName(ts.ChildOfType(node, "user_type", "parenthesized_type"))
	case "user_type":
		var parts []string
		for _, id := range ts.NamedChildren(node) {
			if id.Type() == "type_identifier" {
				parts = append(parts, p.text(id))
			}
		}
		return strings.Join(parts, ".")
	}
	return ""
}

// kotlinTypeOf returns the class name of the type of a parameter, a
// property or a variable declaration, the first type node among its
// children.
func (p *projection) kotlinTypeOf(node *sitter.Node) string {
	return p.kotlinTypeName(ts.ChildOfType(node, "user_type", "nullable_type", "not_nullable_type"))
}

// kotlinFields returns the properties of a class that declare a type: the
// val and var parameters of the primary constructor and the properties of
// the body. An object has the field INSTANCE and a class with a companion
// object has the field Companion, which Java code names, and both hold the
// class itself, because the members of a companion are members of the
// class.
func kotlinFields(p *projection, class *sitter.Node) []fileindex.Typed {
	name := p.text(ts.ChildOfType(class, "type_identifier"))
	var fields []fileindex.Typed
	if class.Type() == "object_declaration" {
		fields = append(fields, fileindex.Typed{Name: "INSTANCE", Type: name})
	}
	if ctor := ts.ChildOfType(class, "primary_constructor"); ctor != nil {
		for _, param := range ts.NamedChildren(ctor) {
			if param.Type() != "class_parameter" || ts.ChildOfType(param, "binding_pattern_kind") == nil {
				continue
			}
			if typ := p.kotlinTypeOf(param); typ != "" {
				fields = append(fields, fileindex.Typed{Name: p.text(ts.ChildOfType(param, "simple_identifier")), Type: typ})
			}
		}
	}
	body := ts.ChildOfType(class, "class_body", "enum_class_body")
	if body == nil {
		return fields
	}
	for _, member := range ts.NamedChildren(body) {
		switch member.Type() {
		case "property_declaration":
			declaration := ts.ChildOfType(member, "variable_declaration")
			if typ := p.kotlinTypeOf(declaration); declaration != nil && typ != "" {
				fields = append(fields, fileindex.Typed{Name: p.text(ts.ChildOfType(declaration, "simple_identifier")), Type: typ})
			}
		case "companion_object":
			companion := "Companion"
			if id := ts.ChildOfType(member, "type_identifier"); id != nil {
				companion = p.text(id)
			}
			fields = append(fields, fileindex.Typed{Name: companion, Type: name})
		}
	}
	return fields
}

// kotlinAnnotations returns the annotations of a declaration as written,
// with no arguments, as @GetMapping for @GetMapping("/a").
func kotlinAnnotations(p *projection, node *sitter.Node) []string {
	modifiers := ts.ChildOfType(node, "modifiers")
	if modifiers == nil {
		return nil
	}
	var annotations []string
	for _, a := range ts.NamedChildren(modifiers) {
		if a.Type() != "annotation" {
			continue
		}
		typ := ts.ChildOfType(a, "user_type", "constructor_invocation")
		if typ != nil && typ.Type() == "constructor_invocation" {
			typ = ts.ChildOfType(typ, "user_type")
		}
		if name := p.kotlinTypeName(typ); name != "" {
			annotations = append(annotations, "@"+name)
		}
	}
	return annotations
}

// kotlinSignature sets the static mark, the parameters and the typed locals
// of a function. A top-level function, a member of a companion object and
// a function with @JvmStatic are static, as the JVM calls them. The
// receiver of an extension function is its first parameter, with the name
// this.
func kotlinSignature(p *projection, def *fileindex.Definition, node *sitter.Node) {
	if def.Name == staticInitName {
		def.Static = true
		return
	}
	if node.Type() != "function_declaration" && node.Type() != "secondary_constructor" {
		return
	}
	def.Static = node.Type() == "function_declaration" && (def.Parent == "" && def.Kind == fileindex.DefFunction ||
		slices.Contains(def.Decorators, "@JvmStatic") || inCompanion(node))
	// The receiver type of an extension comes before the name.
	for _, child := range ts.NamedChildren(node) {
		if child.Type() == "simple_identifier" {
			break
		}
		if typ := p.kotlinTypeName(child); typ != "" {
			def.Params = append(def.Params, fileindex.Typed{Name: extensionParam, Type: typ})
		}
	}
	if params := ts.ChildOfType(node, "function_value_parameters"); params != nil {
		for _, param := range ts.NamedChildren(params) {
			if typ := p.kotlinTypeOf(param); param.Type() == "parameter" && typ != "" {
				def.Params = append(def.Params, fileindex.Typed{Name: p.text(ts.ChildOfType(param, "simple_identifier")), Type: typ})
			}
		}
		// The result type comes after the parameters.
		for next := params.NextNamedSibling(); next != nil && next.Type() != "function_body"; next = next.NextNamedSibling() {
			if typ := p.kotlinTypeName(next); typ != "" {
				def.Returns = typ
				break
			}
		}
	}
	def.Locals = p.kotlinLocals(ts.ChildOfType(node, "function_body", "statements"))
}

// inCompanion reports whether a member is a member of a companion object.
func inCompanion(node *sitter.Node) bool {
	body := node.Parent()
	return body != nil && body.Type() == "class_body" && body.Parent() != nil && body.Parent().Type() == "companion_object"
}

// kotlinLocals returns the local variables of a body that declare a class
// type, as s in val s: Service = f().
func (p *projection) kotlinLocals(body *sitter.Node) []fileindex.Typed {
	var locals []fileindex.Typed
	walkBody(body, map[string]bool{"class_body": true, "function_declaration": true}, func(node *sitter.Node) {
		if node.Type() != "variable_declaration" {
			return
		}
		if typ := p.kotlinTypeOf(node); typ != "" {
			locals = append(locals, fileindex.Typed{Name: p.text(ts.ChildOfType(node, "simple_identifier")), Type: typ})
		}
	})
	return locals
}

// kotlinReferenceVisitor adds each callable reference of a Kotlin file, as
// ::helper, this::helper or Parser::parse. The code that gets the value can
// call it. A reference to a constructor, as ::Parser, names the class.
func kotlinReferenceVisitor(p *projection) visitor {
	return func(node *sitter.Node, kind string) {
		if kind != "callable_reference" {
			return
		}
		name := ts.ChildOfType(node, "simple_identifier")
		if name == nil {
			return
		}
		caller := p.innermost(node.StartByte())
		target := p.text(name)
		receiver := ""
		for i := 0; i < int(node.NamedChildCount()); i++ {
			if child := node.NamedChild(i); !ts.SameNode(child, name) {
				receiver = p.text(child)
				break
			}
		}
		switch receiver {
		case "":
			if targets := p.rebind(caller, target); len(targets) > 0 {
				target = targets[0]
			}
		case "this":
			target = fileindex.Join(p.enclosingClassKey(caller), target)
		default:
			base := strings.ReplaceAll(receiver, ".", fileindex.Separator)
			if targets := p.rebind(caller, fileindex.Unit(base)); len(targets) > 0 {
				base = fileindex.Join(targets[0], fileindex.Segments(base)...)
			}
			target = fileindex.Join(base, target)
		}
		p.addReference(node, caller, []string{target})
	}
}

// kotlinFlagVisitor marks the reflection calls and the injected values of
// a Kotlin file, as the Java rules do.
func kotlinFlagVisitor(p *projection) visitor {
	return func(node *sitter.Node, kind string) {
		switch kind {
		case "call_expression":
			callee := node.NamedChild(0)
			if callee == nil || callee.Type() != "navigation_expression" {
				return
			}
			name := p.text(ts.ChildOfType(ts.ChildOfType(callee, "navigation_suffix"), "simple_identifier"))
			if slices.Contains(javaReflection, name) {
				p.flag(fileindex.FlagReflection, node)
			}
		case "annotation":
			typ := ts.ChildOfType(node, "user_type", "constructor_invocation")
			if typ != nil && typ.Type() == "constructor_invocation" {
				typ = ts.ChildOfType(typ, "user_type")
			}
			name := p.kotlinTypeName(typ)
			if injected(name) {
				p.flag(fileindex.FlagInjection, node)
			}
		}
	}
}

// enclosingClassKey returns the class that holds the key, or the key of
// the file.
func (p *projection) enclosingClassKey(key string) string {
	for scope := key; scope != p.cg.FileName; scope = fileindex.Parent(scope) {
		if def := p.defs[scope]; def != nil && (def.Kind == fileindex.DefClass || def.Kind == fileindex.DefInterface) {
			return scope
		}
	}
	return p.cg.FileName
}
