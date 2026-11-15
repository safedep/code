package indexer

import (
	"slices"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/pkg/ts"
	sitter "github.com/smacker/go-tree-sitter"
)

// exported reports that the rule of the language makes the definition
// visible outside the file. With no rule, a top-level definition is
// exported, and a method is exported with its class.
func (p *projection) exported(def *fileindex.Definition) bool {
	switch exported := p.rules().exported; {
	case def.Kind == fileindex.DefModule:
		return true
	case exported != nil:
		return exported(p, def)
	case def.Parent != "":
		return p.exported(p.defs[def.Parent])
	default:
		return def.Key == p.local(def.Name)
	}
}

func (p *projection) local(name string) string {
	return fileindex.Join(p.cg.FileName, name)
}

// export adds the export of a local name as name. A name that an import
// binds is given on, and Item holds the local name when it is not name.
func (p *projection) export(name, local string, line int) {
	if p.bindings[local] {
		item := local
		if item == name {
			item = ""
		}
		p.ix.Exports = append(p.ix.Exports, fileindex.Export{Name: name, Item: item, Line: line})
		return
	}
	target := p.local(local)
	p.exports[target] = true
	p.ix.Exports = append(p.ix.Exports, fileindex.Export{Name: name, Target: target, Line: line})
}

// reexport adds an export of item of the module from as name. An empty
// item exports the whole module, and "*" each name of it.
func (p *projection) reexport(name, from, item string, line int) {
	p.ix.Exports = append(p.ix.Exports, fileindex.Export{Name: name, From: from, Item: item, Line: line})
}

// addExportStatement adds the exports of an export statement. A TypeScript
// export type exports no value, so it gives no export.
func (p *projection) addExportStatement(statement *sitter.Node) {
	if typeOnly(statement) {
		return
	}
	line := lineOf(statement)
	isDefault := false
	for i := 0; i < int(statement.ChildCount()); i++ {
		if statement.Child(i).Type() == "default" {
			isDefault = true
		}
	}
	named := func(name string) string {
		if isDefault {
			return "default"
		}
		return name
	}

	source := ""
	if node := statement.ChildByFieldName("source"); node != nil {
		source, _ = p.stringPrefix(node)
	}

	if declaration := statement.ChildByFieldName("declaration"); declaration != nil {
		if t := declaration.Type(); t == "lexical_declaration" || t == "variable_declaration" {
			p.exportDeclarators(declaration, line)
			return
		}
		for _, name := range p.declaredNames(declaration) {
			p.export(named(name), name, line)
		}
		return
	}
	if value := statement.ChildByFieldName("value"); value != nil {
		switch {
		case value.Type() == "identifier":
			p.export(defaultExport, p.text(value), line)
		case functionValues[value.Type()] || value.Type() == "class":
			p.export(defaultExport, defaultExport, line)
		case opaque(value):
			// A default of a call, as export default NextAuth({...}), has
			// no definition. The record keeps it for a rule that roots it.
			p.export(defaultExport, defaultExport, line)
			p.ix.Exports[len(p.ix.Exports)-1].Opaque = true
		}
		return
	}

	clause := false
	for _, child := range ts.NamedChildren(statement) {
		switch child.Type() {
		case "export_clause":
			clause = true
			for _, specifier := range ts.NamedChildren(child) {
				if typeOnly(specifier) {
					continue
				}
				local := p.text(specifier.ChildByFieldName("name"))
				name := local
				if alias := specifier.ChildByFieldName("alias"); alias != nil {
					name = p.text(alias)
				}
				if source != "" {
					p.reexport(name, source, local, line)
				} else {
					p.exportValue(name, local, p.topValue(local), line)
				}
			}
		case "namespace_export":
			clause = true
			p.reexport(p.text(child.NamedChild(0)), source, "", line)
		}
	}
	if !clause && source != "" {
		p.reexport("*", source, "*", line)
	}
}

// declaredNames returns the names that a declaration binds. A type alias
// binds no value and gives no name. An interface binds no value either, but
// a declared type in another file can name it.
func (p *projection) declaredNames(declaration *sitter.Node) []string {
	switch declaration.Type() {
	case "lexical_declaration", "variable_declaration":
		var names []string
		for _, declarator := range ts.NamedChildren(declaration) {
			if name := declarator.ChildByFieldName("name"); name != nil && name.Type() == "identifier" {
				names = append(names, p.text(name))
			}
		}
		return names
	case "type_alias_declaration":
		return nil
	}
	if name := declaration.ChildByFieldName("name"); name != nil {
		return []string{p.text(name)}
	}
	return nil
}

// exportDeclarators adds the exports of the names that an exported
// variable declaration binds. A name of a destructuring pattern, as GET in
// export const { GET } = handlers, takes the member of the value that it
// names, as handlers//GET. A framework calls the function of an exported
// name, and without the record no root names the function. A name of an
// array pattern or a rest element takes no member that a key names, so
// its target is the object that holds it.
func (p *projection) exportDeclarators(declaration *sitter.Node, line int) {
	for _, declarator := range ts.NamedChildren(declaration) {
		pattern, value := declarator.ChildByFieldName("name"), unwrap(declarator.ChildByFieldName("value"))
		switch {
		case pattern == nil:
		case pattern.Type() == "identifier":
			name := p.text(pattern)
			p.exportValue(name, name, value, line)
		case value != nil && value.Type() == "identifier" && !opaque(p.topValue(p.text(value))):
			p.exportPattern(pattern, []string{p.text(value)}, line)
		default:
			for _, name := range patternNames(p, pattern) {
				p.export(name, name, line)
				p.ix.Exports[len(p.ix.Exports)-1].Opaque = true
			}
		}
	}
}

// exportValue adds the export of a local name that holds a value. A name
// that holds another name gives that name first, as handler for GET in
// export const GET = handler, so the export reaches its function. A value
// that is no literal, as the result of a call, is opaque.
func (p *projection) exportValue(name, local string, value *sitter.Node, line int) {
	value = unwrap(value)
	if value != nil && value.Type() == "identifier" && p.text(value) != local {
		p.export(name, p.text(value), line)
	}
	p.export(name, local, line)
	if value != nil && value.Type() != "identifier" && opaque(value) {
		p.ix.Exports[len(p.ix.Exports)-1].Opaque = true
	}
}

// typeWrappers hold a value with a type or a check of TypeScript, as
// x satisfies T, x as T and x!, or in parentheses. The value is the value
// of the wrapper.
var typeWrappers = map[string]bool{
	"satisfies_expression": true, "as_expression": true, "non_null_expression": true,
	"parenthesized_expression": true, "type_assertion": true,
}

// unwrap returns the value that type wrappers hold.
func unwrap(value *sitter.Node) *sitter.Node {
	for value != nil && typeWrappers[value.Type()] {
		value = value.NamedChild(0)
	}
	return value
}

// opaqueParts are the expressions that can give a function that no
// definition of the file names: a call, a new, an await, and a read of a
// member, which can hold a function of another object.
var opaqueParts = map[string]bool{
	"call_expression": true, "new_expression": true, "await_expression": true,
	"member_expression": true, "subscript_expression": true,
}

// opaque reports whether a value runs code that the index does not follow
// to a definition, as a call. A value with no call, new, await or read of
// a member, as 60 * 60 or 'x' as const, is a literal. A nil value is a
// name that no top-level declaration of the file binds, as an import.
func opaque(value *sitter.Node) bool {
	value = unwrap(value)
	if value == nil || value.Type() == "identifier" || functionValues[value.Type()] || literalValues[value.Type()] {
		return false
	}
	if opaqueParts[value.Type()] {
		return true
	}
	return slices.ContainsFunc(ts.NamedChildren(value), func(child *sitter.Node) bool {
		return opaqueParts[child.Type()] || opaque(child)
	})
}

// literalValues are the JavaScript values that run no code of their own
// when code reads them. A function of an object or an array is a
// definition of the file.
var literalValues = map[string]bool{
	"string": true, "template_string": true, "number": true, "true": true, "false": true, "null": true,
	"undefined": true, "regex": true, "object": true, "array": true, "class": true,
}

// topValue returns the value of a top-level variable of the file, or nil.
func (p *projection) topValue(name string) *sitter.Node {
	for _, statement := range ts.NamedChildren(p.tree.Tree().RootNode()) {
		if statement.Type() == "export_statement" {
			statement = statement.ChildByFieldName("declaration")
		}
		if statement == nil || statement.Type() != "lexical_declaration" && statement.Type() != "variable_declaration" {
			continue
		}
		for _, declarator := range ts.NamedChildren(statement) {
			if n := declarator.ChildByFieldName("name"); n != nil && n.Type() == "identifier" && p.text(n) == name {
				return declarator.ChildByFieldName("value")
			}
		}
	}
	return nil
}

// patternNames returns the names that a destructuring pattern binds.
func patternNames(p *projection, pattern *sitter.Node) []string {
	switch pattern.Type() {
	case "identifier", "shorthand_property_identifier_pattern":
		return []string{p.text(pattern)}
	case "assignment_pattern", "object_assignment_pattern":
		if left := pattern.ChildByFieldName("left"); left != nil {
			return patternNames(p, left)
		}
	case "pair_pattern":
		if value := pattern.ChildByFieldName("value"); value != nil {
			return patternNames(p, value)
		}
	case "object_pattern", "array_pattern", "rest_pattern":
		var names []string
		for _, child := range ts.NamedChildren(pattern) {
			names = append(names, patternNames(p, child)...)
		}
		return names
	}
	return nil
}

// exportPattern adds the exports of the names of a pattern whose value is
// the member at the path. A name of an object pattern takes the member of
// its own name, as GET in { GET }.
func (p *projection) exportPattern(pattern *sitter.Node, path []string, line int) {
	switch pattern.Type() {
	case "identifier":
		p.export(p.text(pattern), fileindex.Join(path[0], path[1:]...), line)
	case "shorthand_property_identifier_pattern":
		name := p.text(pattern)
		p.export(name, fileindex.Join(path[0], append(slices.Clip(path[1:]), name)...), line)
	case "assignment_pattern", "object_assignment_pattern":
		// A default holds the value when the member is undefined, as fb
		// for PUT in { PUT = fb } = h.
		left, right := pattern.ChildByFieldName("left"), pattern.ChildByFieldName("right")
		if left == nil {
			return
		}
		p.exportPattern(left, path, line)
		if right != nil && right.Type() == "identifier" {
			for _, name := range patternNames(p, left) {
				p.export(name, p.text(right), line)
			}
		}
	case "pair_pattern":
		key, value := pattern.ChildByFieldName("key"), pattern.ChildByFieldName("value")
		if key == nil || value == nil {
			return
		}
		if key.Type() == "property_identifier" {
			path = append(slices.Clip(path), p.text(key))
		}
		p.exportPattern(value, path, line)
	case "object_pattern", "array_pattern", "rest_pattern":
		for _, child := range ts.NamedChildren(pattern) {
			p.exportPattern(child, path, line)
		}
	}
}

// addExportAssignment adds the exports of a CommonJS assignment, as
// module.exports = f, module.exports = { f } and exports.f = f.
func (p *projection) addExportAssignment(statement, assignment *sitter.Node) {
	left, right := assignment.ChildByFieldName("left"), assignment.ChildByFieldName("right")
	if left == nil || right == nil {
		return
	}
	line := lineOf(statement)

	switch target := p.text(left); {
	case target == "module.exports":
		p.exportStatements = append(p.exportStatements, statement)
		p.addModuleExports(right, line)
	case left.Type() == "member_expression":
		object := p.text(left.ChildByFieldName("object"))
		if object != "exports" && object != "module.exports" {
			return
		}
		p.exportStatements = append(p.exportStatements, statement)
		name := p.text(left.ChildByFieldName("property"))
		switch {
		case right.Type() == "identifier":
			p.export(name, p.text(right), line)
		case functionValues[right.Type()] || right.Type() == "class":
			p.export(name, name, line)
		}
	}
}

// defaultExport is the name of the default export of a module.
const defaultExport = "default"

// addModuleExports adds the exports of module.exports = value. A name is the
// default export. A function or a class is the definition default, which
// is the default export. An object exports each name of it. A function
// that a property holds is the definition of the property name.
func (p *projection) addModuleExports(value *sitter.Node, line int) {
	switch {
	case functionValues[value.Type()] || value.Type() == "class":
		p.export(defaultExport, defaultExport, line)
		return
	}
	switch value.Type() {
	case "identifier":
		p.export(defaultExport, p.text(value), line)
	case "object":
		for _, property := range ts.NamedChildren(value) {
			switch property.Type() {
			case "shorthand_property_identifier":
				p.export(p.text(property), p.text(property), line)
			case "method_definition":
				name := p.text(property.ChildByFieldName("name"))
				p.export(name, name, line)
			case "pair":
				name, v := p.text(property.ChildByFieldName("key")), property.ChildByFieldName("value")
				switch {
				case v == nil:
				case v.Type() == "identifier":
					p.export(name, p.text(v), line)
				case functionValues[v.Type()]:
					p.export(name, name, line)
				}
			}
		}
	}
}

func (p *projection) text(node *sitter.Node) string {
	if node == nil {
		return ""
	}
	return node.Content(p.source)
}

func lineOf(node *sitter.Node) int {
	return int(node.StartPoint().Row) + 1
}
