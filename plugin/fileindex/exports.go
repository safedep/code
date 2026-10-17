package fileindex

import (
	"unicode"
	"unicode/utf8"

	"github.com/safedep/code/core"
	fidx "github.com/safedep/code/fileindex"
	"github.com/safedep/code/pkg/ts"
	sitter "github.com/smacker/go-tree-sitter"
)

// addExports adds the export records of the file. The exports of a
// JavaScript or TypeScript file are its export statements and its CommonJS
// assignments. The exports of a Python file are the names of its __all__.
func (p *projection) addExports() {
	if !p.ix.Language.IsECMAScript() && p.ix.Language != core.LanguageCodePython {
		return
	}
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

// exported reports that the rule of the language makes the definition
// visible outside the file. A JavaScript or TypeScript definition is
// exported when an export names it. A Go name is exported when it starts
// with an upper case letter, a method too. Elsewhere a top-level definition
// is exported, and a method is exported with its class.
func (p *projection) exported(def *fidx.Definition) bool {
	switch {
	case def.Kind == fidx.DefModule:
		return true
	case p.ix.Language == core.LanguageCodeGo:
		r, _ := utf8.DecodeRuneInString(def.Name)
		return unicode.IsUpper(r)
	case def.Parent != "":
		return p.exported(p.defs[def.Parent])
	case p.ix.Language.IsECMAScript():
		return p.exports[def.Key]
	default:
		return def.Key == p.local(def.Name)
	}
}

func (p *projection) local(name string) string {
	return fidx.Join(p.cg.FileName, name)
}

// export adds the export of a local name as name. A name that an import
// binds is given on, and Item holds the local name when it is not name.
func (p *projection) export(name, local string, line int) {
	if p.bindings[local] {
		item := local
		if item == name {
			item = ""
		}
		p.ix.Exports = append(p.ix.Exports, fidx.Export{Name: name, Item: item, Line: line})
		return
	}
	target := p.local(local)
	p.exports[target] = true
	p.ix.Exports = append(p.ix.Exports, fidx.Export{Name: name, Target: target, Line: line})
}

// reexport adds an export of item of the module from as name. An empty
// item exports the whole module, and "*" each name of it.
func (p *projection) reexport(name, from, item string, line int) {
	p.ix.Exports = append(p.ix.Exports, fidx.Export{Name: name, From: from, Item: item, Line: line})
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
		return
	}
	if value := statement.ChildByFieldName("value"); value != nil {
		if value.Type() == "identifier" {
			p.export("default", p.text(value), line)
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

// addExportAssignment adds the exports of a CommonJS assignment, as
// module.exports = f, module.exports = { f } and exports.f = f, and the
// names of a Python __all__.
func (p *projection) addExportAssignment(statement, assignment *sitter.Node) {
	left, right := assignment.ChildByFieldName("left"), assignment.ChildByFieldName("right")
	if left == nil || right == nil {
		return
	}
	line := lineOf(statement)

	if p.ix.Language == core.LanguageCodePython {
		if left.Type() == "identifier" && p.text(left) == "__all__" {
			p.addAll(right, line)
		}
		return
	}

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
		case functionValues[right.Type()]:
			p.export(name, name, line)
		}
	}
}

// addModuleExports adds the exports of module.exports = value. A name is the
// default export. An object exports each name of it. A function that a
// property holds is the definition of the property name.
func (p *projection) addModuleExports(value *sitter.Node, line int) {
	switch value.Type() {
	case "identifier":
		p.export("default", p.text(value), line)
	case "object":
		for _, property := range ts.NamedChildren(value) {
			switch property.Type() {
			case "shorthand_property_identifier":
				p.export(p.text(property), p.text(property), line)
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

// addAll adds the names of __all__ when it is a list or a tuple of string
// literals. A computed __all__ gives no export.
func (p *projection) addAll(value *sitter.Node, line int) {
	if value.Type() != "list" && value.Type() != "tuple" {
		return
	}
	var names []string
	for _, item := range ts.NamedChildren(value) {
		name, whole := p.stringPrefix(item)
		if !whole {
			return
		}
		names = append(names, name)
	}
	for _, name := range names {
		p.export(name, name, line)
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
