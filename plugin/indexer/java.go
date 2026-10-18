package indexer

import (
	"slices"
	"strings"

	"github.com/safedep/code/core/ast"
	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/pkg/ts"
	sitter "github.com/smacker/go-tree-sitter"
)

// javaInjections name the annotations that let a container set a value,
// so the type that the value has at run time is not in the source.
var javaInjections = []string{"Autowired", "Inject", "Resource"}

// javaReflection name the calls that look up a class or a member by a
// string or construct an instance with no new.
var javaReflection = []string{"forName", "getMethod", "getDeclaredMethod", "getConstructor", "getDeclaredConstructor", "newInstance", "invoke", "loadClass"}

var javaRules = languageRules{
	kinds: map[string]fileindex.DefKind{
		"class_declaration":               fileindex.DefClass,
		"enum_declaration":                fileindex.DefClass,
		"record_declaration":              fileindex.DefClass,
		"interface_declaration":           fileindex.DefInterface,
		"method_declaration":              fileindex.DefMethod,
		"constructor_declaration":         fileindex.DefConstructor,
		"compact_constructor_declaration": fileindex.DefConstructor,
		// A Java enum constant is a definition only when it has a class body.
		"enum_constant": fileindex.DefClass,
	},
	memberScopes: true,
	rawUnbound:   true,
	packageOf:    javaPackage,
	bases:        javaBases,
	fields:       javaFields,
	decorators:   javaAnnotations,
	signature:    javaSignature,
	inits:        addJavaInits,
	prune:        dropIdleJavaInits,
	visitors:     []func(p *projection) visitor{javaFlagVisitor, javaReferenceVisitor},
}

// javaPackage returns the name of the package clause of a Java file.
func javaPackage(p *projection) string {
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
func javaAnnotations(p *projection, node *sitter.Node) []string {
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

// javaSignature adds the static mark, the typed parameters and the typed
// local variables of a Java method or constructor.
func javaSignature(p *projection, def *fileindex.Definition, node *sitter.Node) {
	if def.Kind != fileindex.DefMethod && def.Kind != fileindex.DefConstructor {
		return
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
				def.Params = append(def.Params, fileindex.Typed{Name: p.text(param.ChildByFieldName("name")), Type: typ})
			}
		}
	}
	def.Locals = p.javaLocals(node.ChildByFieldName("body"))
}

// javaInitName names the static initializer of a Java class.
const javaInitName = staticInitName

// addJavaInits adds the static initializer of each Java class whose body
// runs code when the class loads: a static block, the initializer of a
// static field, a constant of an interface or a constant of an enum. The
// JVM runs that code once, before the first static call, the first access
// to a static field and the first instance. So it is a static method
// <clinit> of the class, which takes the calls of those members. A call of
// a static method then does not run the constructor. The constants of an
// enum are instances, so the <clinit> of an enum calls the enum.
func addJavaInits(p *projection) {
	var classes []string
	for key, def := range p.defs {
		if def.Kind == fileindex.DefClass || def.Kind == fileindex.DefInterface {
			classes = append(classes, key)
		}
	}
	for _, class := range classes {
		node := p.syntax[class]
		if node == nil {
			continue
		}
		members := javaStaticMembers(node.ChildByFieldName("body"))
		if len(members) == 0 {
			continue
		}
		key := fileindex.Join(class, javaInitName)
		def := &fileindex.Definition{
			Key:       key,
			Kind:      fileindex.DefMethod,
			Name:      javaInitName,
			StartLine: lineOf(members[0]),
			EndLine:   int(members[len(members)-1].EndPoint().Row) + 1,
			Parent:    class,
			Static:    true,
		}
		constant := false
		for _, m := range members {
			p.bodies[spanOf(m)] = key
			switch {
			case m.Type() == "static_initializer" && m.NamedChildCount() > 0:
				def.Locals = append(def.Locals, p.javaLocals(m.NamedChild(0))...)
			case m.Type() == "enum_constant" && !constant:
				constant = true
				site := ast.GetNodePosition(m)
				p.ix.Calls = append(p.ix.Calls, fileindex.Call{
					Caller:  key,
					Line:    int(site.StartLine),
					Column:  int(site.StartColumn),
					Callee:  fileindex.Last(class),
					Targets: []string{class},
					Kind:    fileindex.CallNew,
				})
			}
		}
		p.defs[key] = def
	}
}

// dropIdleJavaInits removes each static initializer that runs no call, no
// reference and no unresolved site, as the one of static final int X = 1.
func dropIdleJavaInits(p *projection) {
	busy := map[string]bool{}
	for _, c := range p.ix.Calls {
		busy[c.Caller] = true
	}
	for _, r := range p.ix.References {
		busy[r.Caller] = true
	}
	for _, u := range p.ix.Unresolved {
		busy[u.Caller] = true
	}
	for key, def := range p.defs {
		if def.Name == javaInitName && !busy[key] && len(def.Flags) == 0 {
			delete(p.defs, key)
		}
	}
}

// javaStaticMembers returns the members of a class body that run when the
// class loads.
func javaStaticMembers(body *sitter.Node) []*sitter.Node {
	if body == nil {
		return nil
	}
	var members []*sitter.Node
	for _, m := range ts.NamedChildren(body) {
		switch m.Type() {
		case "static_initializer", "constant_declaration", "enum_constant":
			members = append(members, m)
		case "field_declaration":
			if modifiers := javaModifiers(m); modifiers != nil && hasKeyword(modifiers, "static") {
				members = append(members, m)
			}
		case "enum_body_declarations":
			members = append(members, javaStaticMembers(m)...)
		}
	}
	return members
}

// javaLocals returns the local variables of a body that declare a class
// type, as s in Service s = f(), or that a new constructs, as s in
// var s = new Service().
func (p *projection) javaLocals(body *sitter.Node) []fileindex.Typed {
	var locals []fileindex.Typed
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
					locals = append(locals, fileindex.Typed{Name: p.text(declarator.ChildByFieldName("name")), Type: t})
				}
			}
		case "enhanced_for_statement":
			if typ := p.javaTypeName(node.ChildByFieldName("type")); typ != "" {
				locals = append(locals, fileindex.Typed{Name: p.text(node.ChildByFieldName("name")), Type: typ})
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
// The components of a record are fields too.
func javaFields(p *projection, class *sitter.Node) []fileindex.Typed {
	var fields []fileindex.Typed
	if components := class.ChildByFieldName("parameters"); class.Type() == "record_declaration" && components != nil {
		for _, c := range ts.NamedChildren(components) {
			if typ := p.javaTypeName(c.ChildByFieldName("type")); c.Type() == "formal_parameter" && typ != "" {
				fields = append(fields, fileindex.Typed{Name: p.text(c.ChildByFieldName("name")), Type: typ})
			}
		}
	}
	body := class.ChildByFieldName("body")
	if body == nil {
		return fields
	}
	for _, member := range ts.NamedChildren(body) {
		if member.Type() != "field_declaration" {
			continue
		}
		typ := p.javaTypeName(member.ChildByFieldName("type"))
		if typ == "" {
			continue
		}
		for _, declarator := range childrenByField(member, "declarator") {
			fields = append(fields, fileindex.Typed{Name: p.text(declarator.ChildByFieldName("name")), Type: typ})
		}
	}
	return fields
}

// javaBases returns the class that a class extends and the interfaces that
// it implements, or the interfaces that an interface extends, as written. A
// constant of an enum with a class body extends the enum.
func javaBases(p *projection, node *sitter.Node) []string {
	if node.Type() == "enum_constant" {
		if enum := node.Parent().Parent(); enum != nil && enum.Type() == "enum_declaration" {
			return nonEmpty(p.text(enum.ChildByFieldName("name")))
		}
		return nil
	}
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
func javaFlagVisitor(p *projection) visitor {
	return func(node *sitter.Node, kind string) {
		switch kind {
		case "method_invocation":
			name := p.text(node.ChildByFieldName("name"))
			if slices.Contains(javaReflection, name) || p.text(node.ChildByFieldName("object")) == "ServiceLoader" && name == "load" {
				p.flag(fileindex.FlagReflection, node)
			}
		case "marker_annotation", "annotation":
			name := p.text(node.ChildByFieldName("name"))
			if slices.Contains(javaInjections, name[strings.LastIndex(name, ".")+1:]) {
				p.flag(fileindex.FlagInjection, node)
			}
		}
	}
}

// javaReferenceVisitor adds each method reference of a Java file, as
// this::helper or Parser::parse. The code that gets the value can call it.
// A constructor reference, as Parser::new, names the class.
func javaReferenceVisitor(p *projection) visitor {
	return func(node *sitter.Node, kind string) {
		if kind != "method_reference" || node.NamedChildCount() == 0 {
			return
		}
		caller := p.innermost(node.StartByte())
		object := node.NamedChild(0)
		target := strings.ReplaceAll(p.text(object), ".", fileindex.Separator)
		switch {
		case object.Type() == "this":
			for scope := caller; scope != p.cg.FileName; scope = fileindex.Parent(scope) {
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
			target = fileindex.Join(target, p.text(last))
		}
		p.addReference(node, caller, []string{target})
	}
}
