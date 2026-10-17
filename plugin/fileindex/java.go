package fileindex

import (
	"slices"
	"strings"

	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
	fidx "github.com/safedep/code/fileindex"
	"github.com/safedep/code/pkg/ts"
	sitter "github.com/smacker/go-tree-sitter"
)

// javaInjections name the annotations that let a container set a value,
// so the type that the value has at run time is not in the source.
var javaInjections = []string{"Autowired", "Inject", "Resource"}

// javaReflection name the calls that look up a class or a member by a
// string or construct an instance with no new.
var javaReflection = []string{"forName", "getMethod", "getDeclaredMethod", "getConstructor", "getDeclaredConstructor", "newInstance", "invoke", "loadClass"}

func (p *projection) isJava() bool {
	return p.ix.Language == core.LanguageCodeJava
}

// javaPackage returns the name of the package clause of a Java file.
func (p *projection) javaPackage() string {
	for _, statement := range ts.NamedChildren(p.tree.Tree().RootNode()) {
		if statement.Type() != "package_declaration" {
			continue
		}
		for _, name := range ts.NamedChildren(statement) {
			if name.Type() == "scoped_identifier" || name.Type() == "identifier" {
				return p.text(name)
			}
		}
	}
	return ""
}

// javaModifiers returns the modifiers of a declaration.
func javaModifiers(node *sitter.Node) *sitter.Node {
	for _, child := range ts.NamedChildren(node) {
		if child.Type() == "modifiers" {
			return child
		}
	}
	return nil
}

// javaAnnotations returns the annotations of a declaration as written, with
// no arguments, as @GetMapping for @GetMapping("/a").
func (p *projection) javaAnnotations(node *sitter.Node) []string {
	modifiers := javaModifiers(node)
	if modifiers == nil {
		return nil
	}
	var annotations []string
	for _, a := range ts.NamedChildren(modifiers) {
		if a.Type() == "marker_annotation" || a.Type() == "annotation" {
			annotations = append(annotations, "@"+p.text(a.ChildByFieldName("name")))
		}
	}
	return annotations
}

// addJavaSignatures adds the static mark, the typed parameters and the typed
// local variables of each Java method and constructor.
func (p *projection) addJavaSignatures() {
	if !p.isJava() {
		return
	}
	for key, def := range p.defs {
		node := p.syntax[key]
		if node == nil || def.Kind != fidx.DefMethod && def.Kind != fidx.DefConstructor {
			continue
		}
		if modifiers := javaModifiers(node); modifiers != nil {
			def.Static = hasKeyword(modifiers, "static")
		}
		if params := node.ChildByFieldName("parameters"); params != nil {
			for _, param := range ts.NamedChildren(params) {
				if param.Type() != "formal_parameter" {
					continue
				}
				if typ := p.javaTypeName(param.ChildByFieldName("type")); typ != "" {
					def.Params = append(def.Params, fidx.Typed{Name: p.text(param.ChildByFieldName("name")), Type: typ})
				}
			}
		}
		def.Locals = p.javaLocals(node.ChildByFieldName("body"))
	}
}

// javaLocals returns the local variables of a body that declare a class
// type, as s in Service s = f(), or that a new constructs, as s in
// var s = new Service().
func (p *projection) javaLocals(body *sitter.Node) []fidx.Typed {
	var locals []fidx.Typed
	var walk func(node *sitter.Node, depth int)
	walk = func(node *sitter.Node, depth int) {
		if depth > maxLocalDepth || node.Type() == "class_body" {
			return
		}
		switch node.Type() {
		case "local_variable_declaration":
			typ := p.javaTypeName(node.ChildByFieldName("type"))
			for _, declarator := range childrenByField(node, "declarator") {
				t := typ
				if value := declarator.ChildByFieldName("value"); t == "" && value != nil && value.Type() == "object_creation_expression" {
					t = p.javaTypeName(value.ChildByFieldName("type"))
				}
				if t != "" {
					locals = append(locals, fidx.Typed{Name: p.text(declarator.ChildByFieldName("name")), Type: t})
				}
			}
		case "enhanced_for_statement":
			if typ := p.javaTypeName(node.ChildByFieldName("type")); typ != "" {
				locals = append(locals, fidx.Typed{Name: p.text(node.ChildByFieldName("name")), Type: typ})
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

// javaFields returns the fields of a Java class that declare a class type.
func (p *projection) javaFields(class *sitter.Node) []fidx.Typed {
	body := class.ChildByFieldName("body")
	if body == nil {
		return nil
	}
	var fields []fidx.Typed
	for _, member := range ts.NamedChildren(body) {
		if member.Type() != "field_declaration" {
			continue
		}
		typ := p.javaTypeName(member.ChildByFieldName("type"))
		if typ == "" {
			continue
		}
		for _, declarator := range childrenByField(member, "declarator") {
			fields = append(fields, fidx.Typed{Name: p.text(declarator.ChildByFieldName("name")), Type: typ})
		}
	}
	return fields
}

// javaBases returns the class that a class extends and the interfaces that
// it implements, or the interfaces that an interface extends, as written.
func (p *projection) javaBases(node *sitter.Node) []string {
	var bases []string
	for _, child := range ts.NamedChildren(node) {
		switch child.Type() {
		case "superclass":
			if child.NamedChildCount() > 0 {
				bases = append(bases, nonEmpty(p.javaTypeName(child.NamedChild(0)))...)
			}
		case "super_interfaces", "extends_interfaces":
			for _, list := range ts.NamedChildren(child) {
				for _, t := range ts.NamedChildren(list) {
					bases = append(bases, nonEmpty(p.javaTypeName(t))...)
				}
			}
		}
	}
	return bases
}

// javaTypeName returns the name of the class type that a type node names,
// as Parser for Parser or Parser<T>, and com.x.Parser for a qualified name.
// A primitive type, an array and var name no class.
func (p *projection) javaTypeName(node *sitter.Node) string {
	if node == nil {
		return ""
	}
	switch node.Type() {
	case "type_identifier":
		if name := p.text(node); name != "var" {
			return name
		}
	case "scoped_type_identifier":
		return p.text(node)
	case "generic_type":
		if node.NamedChildCount() > 0 {
			return p.javaTypeName(node.NamedChild(0))
		}
	}
	return ""
}

// javaFlagVisitor marks the reflection calls and the injected values of a
// Java file. A Java call has no function field, so the flags pass of the
// other languages does not read it.
func (p *projection) javaFlagVisitor() visitor {
	if !p.isJava() {
		return nil
	}
	return func(node *sitter.Node, kind string) {
		switch kind {
		case "method_invocation":
			name := p.text(node.ChildByFieldName("name"))
			if slices.Contains(javaReflection, name) || p.text(node.ChildByFieldName("object")) == "ServiceLoader" && name == "load" {
				p.flag(fidx.FlagReflection, node)
			}
		case "marker_annotation", "annotation":
			name := p.text(node.ChildByFieldName("name"))
			if slices.Contains(javaInjections, name[strings.LastIndex(name, ".")+1:]) {
				p.flag(fidx.FlagInjection, node)
			}
		}
	}
}

// javaReferenceVisitor adds each method reference of a Java file, as
// this::helper or Parser::parse. The code that gets the value can call it.
// A constructor reference, as Parser::new, names the class.
func (p *projection) javaReferenceVisitor() visitor {
	if !p.isJava() {
		return nil
	}
	return func(node *sitter.Node, kind string) {
		if kind != "method_reference" || node.NamedChildCount() == 0 {
			return
		}
		caller := p.innermost(node.StartByte())
		object := node.NamedChild(0)
		target := strings.ReplaceAll(p.text(object), ".", fidx.Separator)
		switch {
		case object.Type() == "this":
			for scope := caller; scope != p.cg.FileName; scope = fidx.Parent(scope) {
				if def := p.defs[scope]; def != nil && def.Parent != "" {
					target = def.Parent
					break
				}
			}
		case object.Type() == "identifier" || object.Type() == "type_identifier":
			if targets := p.rebind(caller, p.text(object)); len(targets) > 0 {
				target = targets[0]
			}
		}
		if last := node.NamedChild(int(node.NamedChildCount()) - 1); node.NamedChildCount() > 1 && last.Type() == "identifier" {
			target = fidx.Join(target, p.text(last))
		}
		site := ast.GetNodePosition(node)
		p.ix.References = append(p.ix.References, fidx.Reference{
			Caller:  caller,
			Line:    int(site.StartLine),
			Column:  int(site.StartColumn),
			Text:    p.text(node),
			Targets: []string{target},
		})
	}
}
