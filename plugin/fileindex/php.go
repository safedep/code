package fileindex

import (
	"strings"

	"github.com/safedep/code/core/ast"
	fidx "github.com/safedep/code/fileindex"
	"github.com/safedep/code/pkg/ts"
	"github.com/safedep/code/plugin/callgraph"
	sitter "github.com/smacker/go-tree-sitter"
)

// phpIncludes are the node types of an include or a require of a file.
var phpIncludes = map[string]bool{
	"include_expression": true, "include_once_expression": true,
	"require_expression": true, "require_once_expression": true,
}

// phpDirs are the expressions that name the directory of the file, as in
// require __DIR__ . '/lib/helpers.php'.
var phpDirs = map[string]bool{"__DIR__": true, "dirname(__FILE__)": true}

var phpRules = languageRules{
	packageOf:  phpNamespace,
	bases:      phpBases,
	fields:     phpFields,
	decorators: phpAttributes,
	signature:  phpSignature,
	visitors:   []func(p *projection) visitor{phpVisitor},
}

func phpNamespace(p *projection) string {
	if ns := childOfType(p.tree.Tree().RootNode(), "namespace_definition"); ns != nil {
		return p.text(ns.ChildByFieldName("name"))
	}
	return ""
}

// phpBases returns the class that a class extends, the interfaces that it
// implements and the traits that it uses, as written. A trait gives its
// methods to the class, so a method lookup reads it as a base.
func phpBases(p *projection, class *sitter.Node) []string {
	var bases []string
	for _, child := range ts.NamedChildren(class) {
		switch child.Type() {
		case "base_clause", "class_interface_clause":
			for _, name := range ts.NamedChildren(child) {
				bases = append(bases, nonEmpty(p.text(name))...)
			}
		}
	}
	if body := class.ChildByFieldName("body"); body != nil {
		for _, member := range ts.NamedChildren(body) {
			if member.Type() != "use_declaration" {
				continue
			}
			for _, name := range ts.NamedChildren(member) {
				if name.Type() == "name" || name.Type() == "qualified_name" {
					bases = append(bases, p.text(name))
				}
			}
		}
	}
	return bases
}

// phpTypeName returns the class that a type names, as Loader for Loader or
// ?Loader. A builtin type names no class.
func (p *projection) phpTypeName(node *sitter.Node) string {
	if node == nil {
		return ""
	}
	switch node.Type() {
	case "named_type":
		if name := childOfType(node, "name", "qualified_name"); name != nil {
			return p.text(name)
		}
	case "optional_type":
		return p.phpTypeName(childOfType(node, "named_type"))
	}
	return ""
}

// phpFields returns the typed properties of a class and the promoted
// parameters of its constructor, by the name that $this-> reads, with no $.
func phpFields(p *projection, class *sitter.Node) []fidx.Typed {
	body := class.ChildByFieldName("body")
	if body == nil {
		return nil
	}
	var fields []fidx.Typed
	for _, member := range ts.NamedChildren(body) {
		switch member.Type() {
		case "property_declaration":
			typ := p.phpTypeName(member.ChildByFieldName("type"))
			if typ == "" {
				continue
			}
			for _, element := range ts.NamedChildren(member) {
				if element.Type() == "property_element" {
					fields = append(fields, fidx.Typed{Name: strings.TrimPrefix(p.text(childOfType(element, "variable_name")), "$"), Type: typ})
				}
			}
		case "method_declaration":
			if p.text(member.ChildByFieldName("name")) != "__construct" {
				continue
			}
			for _, param := range ts.NamedChildren(member.ChildByFieldName("parameters")) {
				if typ := p.phpTypeName(param.ChildByFieldName("type")); param.Type() == "property_promotion_parameter" && typ != "" {
					fields = append(fields, fidx.Typed{Name: strings.TrimPrefix(p.text(param.ChildByFieldName("name")), "$"), Type: typ})
				}
			}
		}
	}
	return fields
}

// phpAttributes returns the attributes of a declaration as written, with
// no arguments, as @Route for #[Route('/a')].
func phpAttributes(p *projection, node *sitter.Node) []string {
	var attributes []string
	for _, list := range ts.NamedChildren(node) {
		if list.Type() != "attribute_list" {
			continue
		}
		for _, group := range ts.NamedChildren(list) {
			for _, a := range ts.NamedChildren(group) {
				if a.Type() == "attribute" && a.NamedChildCount() > 0 {
					attributes = append(attributes, "@"+p.text(a.NamedChild(0)))
				}
			}
		}
	}
	return attributes
}

// phpSignature sets the static mark, the typed parameters, by their names
// with the $, and the result of a function or a method.
func phpSignature(p *projection, def *fidx.Definition, node *sitter.Node) {
	if node.Type() != "method_declaration" && node.Type() != "function_definition" {
		return
	}
	def.Static = node.Type() == "function_definition" || childOfType(node, "static_modifier") != nil
	for _, param := range ts.NamedChildren(node.ChildByFieldName("parameters")) {
		if typ := p.phpTypeName(param.ChildByFieldName("type")); typ != "" {
			def.Params = append(def.Params, fidx.Typed{Name: p.text(param.ChildByFieldName("name")), Type: typ})
		}
	}
	def.Returns = p.phpTypeName(node.ChildByFieldName("return_type"))
}

// phpVisitor adds an import for each include and require of a file, a flag
// for one that computes its path, the references of the callables that a
// value names, and the flag of eval.
//
// A callable is an array of an object or a class and a method name, as
// [$this, 'fmt'] or [UserController::class, 'index'], or a string that
// names a function, as 'trim' in array_map('trim', $xs). Code that gets
// it can call it, as a router calls the action of a route.
func phpVisitor(p *projection) visitor {
	return func(node *sitter.Node, kind string) {
		switch {
		case phpIncludes[kind]:
			p.addPHPInclude(node)
		case kind == "array_creation_expression":
			p.addPHPCallable(node)
		case kind == "function_call_expression":
			name := p.text(node.ChildByFieldName("function"))
			if name == "eval" || name == "create_function" {
				p.flag(fidx.FlagEval, node)
			}
			if position, ok := callgraph.CallableArgument(p.ix.Language, name); ok {
				p.addPHPFunctionName(node, position)
			}
		}
	}
}

func (p *projection) addPHPInclude(node *sitter.Node) {
	if node.NamedChildCount() == 0 {
		return
	}
	path, ok := p.phpPath(node.NamedChild(0))
	imp := fidx.Import{Line: lineOf(node), Module: path, Kind: fidx.ImportStatic}
	if !ok {
		imp.Kind = fidx.ImportDynamicComputed
		p.flag(fidx.FlagComputedImport, node)
	}
	p.ix.Imports = append(p.ix.Imports, imp)
}

// phpPath returns the path that an include names: a string, relative to
// the file when the string follows __DIR__. A path with any other part is
// computed, and its static prefix is the path.
func (p *projection) phpPath(node *sitter.Node) (string, bool) {
	if node.Type() == "parenthesized_expression" && node.NamedChildCount() == 1 {
		node = node.NamedChild(0)
	}
	switch node.Type() {
	case "string":
		return p.stringText(node), true
	case "binary_expression":
		left, right := node.ChildByFieldName("left"), node.ChildByFieldName("right")
		if left == nil || right == nil || right.Type() != "string" || !phpDirs[p.text(left)] {
			return "", false
		}
		return "./" + strings.TrimPrefix(p.stringText(right), "/"), true
	}
	return "", false
}

func (p *projection) stringText(node *sitter.Node) string {
	if content := childOfType(node, "string_content"); content != nil {
		return p.text(content)
	}
	return ""
}

// addPHPCallable adds the reference of an array that names a method, as
// [$this, 'fmt'] or [Store::class, 'save'].
func (p *projection) addPHPCallable(node *sitter.Node) {
	elements := ts.NamedChildren(node)
	if len(elements) != 2 || elements[0].NamedChildCount() == 0 || elements[1].NamedChildCount() == 0 {
		return
	}
	holder, method := elements[0].NamedChild(0), elements[1].NamedChild(0)
	if method.Type() != "string" {
		return
	}
	caller := p.innermost(node.StartByte())
	var class string
	switch {
	case p.text(holder) == "$this":
		class = p.enclosingClassKey(caller)
	case holder.Type() == "class_constant_access_expression" && strings.HasSuffix(p.text(holder), "::class"):
		name := strings.TrimSuffix(p.text(holder), "::class")
		if name == "self" || name == "static" {
			class = p.enclosingClassKey(caller)
			break
		}
		class = p.phpName(caller, name)
	default:
		return
	}
	if class != p.cg.FileName {
		p.addPHPReference(node, caller, fidx.Join(class, p.stringText(method)))
	}
}

// addPHPFunctionName adds the reference of the string that names the
// function that a call of a callable calls, as my_filter in
// array_filter($xs, 'my_filter') or Report::run in
// call_user_func('Report::run').
func (p *projection) addPHPFunctionName(call *sitter.Node, position int) {
	args := call.ChildByFieldName("arguments")
	if args == nil || position >= int(args.NamedChildCount()) {
		return
	}
	arg := args.NamedChild(position)
	if arg.NamedChildCount() == 0 || arg.NamedChild(0).Type() != "string" {
		return
	}
	caller := p.innermost(call.StartByte())
	name := strings.ReplaceAll(p.stringText(arg.NamedChild(0)), "::", `\`)
	p.addPHPReference(arg, caller, p.phpName(caller, name))
}

// phpName returns the per-file target of a name as written: a definition
// of the file, the namespace of the use that binds it, or the name.
func (p *projection) phpName(caller, name string) string {
	name = strings.TrimPrefix(name, `\`)
	if targets := p.rebind(caller, name); len(targets) > 0 {
		return targets[0]
	}
	first, rest, _ := strings.Cut(name, `\`)
	for _, imp := range p.ix.Imports {
		if imp.Alias == first && imp.Namespace != "" {
			return fidx.JoinNonEmpty(imp.Namespace, fidx.Namespace(p.ix.Language, rest))
		}
	}
	return fidx.Namespace(p.ix.Language, name)
}

func (p *projection) addPHPReference(node *sitter.Node, caller, target string) {
	site := ast.GetNodePosition(node)
	p.ix.References = append(p.ix.References, fidx.Reference{
		Caller:  p.caller(caller, site),
		Line:    int(site.StartLine),
		Column:  int(site.StartColumn),
		Text:    p.text(node),
		Targets: []string{target},
	})
}
