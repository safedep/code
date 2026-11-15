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
		for _, name := range p.declaredNames(declaration) {
			p.export(named(name), name, line)
		}
		p.exportPatterns(declaration, line)
		return
	}
	if value := statement.ChildByFieldName("value"); value != nil {
		switch {
		case value.Type() == "identifier":
			p.export(defaultExport, p.text(value), line)
		case functionValues[value.Type()] || value.Type() == "class":
			p.export(defaultExport, defaultExport, line)
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
					p.export(name, local, line)
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

// exportPatterns adds the exports of the names that a destructuring
// declaration binds, as GET in export const { GET } = handlers. The
// target of a name is the member of the object that it takes, as
// handlers//GET. A framework calls the function of an exported name, and
// without the record no root names the function. A name of an array
// pattern or a rest element takes no member that a key names, so its
// target is the object that holds it. A value that is no name of the file
// gives no target, and the call of the value holds its code.
func (p *projection) exportPatterns(declaration *sitter.Node, line int) {
	if t := declaration.Type(); t != "lexical_declaration" && t != "variable_declaration" {
		return
	}
	for _, declarator := range ts.NamedChildren(declaration) {
		pattern, value := declarator.ChildByFieldName("name"), declarator.ChildByFieldName("value")
		if pattern == nil || pattern.Type() == "identifier" || value == nil || value.Type() != "identifier" {
			continue
		}
		p.exportPattern(pattern, []string{p.text(value)}, line)
	}
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
		if left := pattern.ChildByFieldName("left"); left != nil {
			p.exportPattern(left, path, line)
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
