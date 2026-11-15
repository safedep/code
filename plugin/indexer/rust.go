package indexer

import (
	"context"
	"maps"
	"slices"
	"strings"
	"unicode"

	"github.com/safedep/code/core/ast"
	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/pkg/ts"
	"github.com/safedep/code/plugin/callgraph"
	sitter "github.com/smacker/go-tree-sitter"
)

var rustRules = languageRules{
	kinds: map[string]fileindex.DefKind{
		"function_item":    fileindex.DefFunction,
		"macro_definition": fileindex.DefFunction,
		"impl_item":        fileindex.DefClass,
		"mod_item":         fileindex.DefClass,
		"trait_item":       fileindex.DefInterface,
	},
	bases:          rustBases,
	fields:         rustFields,
	decorators:     rustAttributes,
	signature:      rustSignature,
	exported:       rustPublic,
	definitions:    rustTypes,
	markTries:      rustMarkTries,
	importExported: rustImportExported,
	shadowed:       rustShadowed,
	visitors:       []func(p *projection) visitor{referenceVisitor, rustVisitor},
	declaredAt:     rustDeclaredAt,
	rebound:        rustRebound,
	// A Rust path to a function, as handler in get(handler) or
	// Self::visit in items.iter().for_each(Self::visit), is a value that
	// the code that gets it calls.
	references: &referenceSyntax{
		holders: map[string]string{
			"arguments":             "",
			"let_declaration":       "value",
			"assignment_expression": "right",
			"return_expression":     "",
			"field_initializer":     "value",
			"array_expression":      "",
		},
		member:      "scoped_identifier",
		object:      "path",
		property:    "name",
		identifiers: map[string]bool{"identifier": true},
		instances:   map[string]bool{"Self": true},
		rawNames:    true,
	},
}

// rustTypeItems are the items that define a type, whose methods an impl
// block of the same name holds.
var rustTypeItems = []string{"struct_item", "enum_item", "union_item"}

// rustTypes adds each struct, enum and union that no impl block of its
// scope names, so that a path names the type, and each type alias. An impl
// block of the type is the class of the type, and its node gives the
// methods.
func rustTypes(p *projection) {
	var walk func(scope string, list *sitter.Node)
	walk = func(scope string, list *sitter.Node) {
		targets := p.rustAliasTargets(list)
		for _, item := range ts.NamedChildren(list) {
			switch {
			case item.Type() == "mod_item":
				if body := item.ChildByFieldName("body"); body != nil {
					walk(fileindex.Join(scope, p.text(item.ChildByFieldName("name"))), body)
				}
			case slices.Contains(rustTypeItems, item.Type()):
				key := fileindex.Join(scope, p.text(item.ChildByFieldName("name")))
				if _, ok := p.defs[key]; ok {
					continue
				}
				p.addDefinition(key, fileindex.DefClass, item)
				p.addSyntax(key, item)
			case item.Type() == "type_item":
				name := p.text(item.ChildByFieldName("name"))
				key := fileindex.Join(scope, name)
				p.addDefinition(key, fileindex.DefAlias, item).Alias = targets[name]
				p.addSyntax(key, item)
			}
		}
	}
	walk(p.cg.FileName, p.tree.Tree().RootNode())
}

// rustAliasTargets returns the type that each alias of a list of items
// names. Only a cfg attribute lets a scope declare a name more than once,
// as a struct under one cfg and an alias under the other, or two aliases.
// The index does not know which cfg the build sets, and one definition
// holds the key. So an alias with another meaning in its scope names no
// type, and a call through it gets the record unknown-receiver. A target
// of each meaning then stays uncertain and does not read not-found.
func (p *projection) rustAliasTargets(list *sitter.Node) map[string]string {
	aliases := map[string][]string{}
	types := map[string]bool{}
	for _, item := range ts.NamedChildren(list) {
		name := p.text(item.ChildByFieldName("name"))
		switch {
		case item.Type() == "type_item":
			if target := p.rustTypeName(item.ChildByFieldName("type"), nil); !slices.Contains(aliases[name], target) {
				aliases[name] = append(aliases[name], target)
			}
		case slices.Contains(rustTypeItems, item.Type()):
			types[name] = true
		}
	}
	targets := map[string]string{}
	for name, named := range aliases {
		if len(named) == 1 && !types[name] {
			targets[name] = named[0]
		}
	}
	return targets
}

// rustSiblings returns the items of the scope of an item that define or
// implement the type of the name, as the struct and each impl block of
// Store.
func (p *projection) rustSiblings(item *sitter.Node, name string) []*sitter.Node {
	parent := item.Parent()
	if parent == nil {
		return nil
	}
	var out []*sitter.Node
	for _, sibling := range ts.NamedChildren(parent) {
		switch {
		case sibling.Type() == "impl_item" && p.rustTypeName(sibling.ChildByFieldName("type"), nil) == name:
			out = append(out, sibling)
		case slices.Contains(rustTypeItems, sibling.Type()) && p.text(sibling.ChildByFieldName("name")) == name:
			out = append(out, sibling)
		}
	}
	return out
}

// rustItemName returns the name of the type that an item defines or
// implements.
func (p *projection) rustItemName(item *sitter.Node) string {
	if item.Type() == "impl_item" {
		return p.rustTypeName(item.ChildByFieldName("type"), nil)
	}
	return p.text(item.ChildByFieldName("name"))
}

// rustBases returns the traits that the impl blocks of a type implement, as
// Store for impl Store for FileStore, and the supertraits of a trait.
func rustBases(p *projection, item *sitter.Node) []string {
	if item.Type() == "trait_item" {
		var bases []string
		for _, bound := range ts.NamedChildren(item.ChildByFieldName("bounds")) {
			bases = append(bases, nonEmpty(p.rustTypeName(bound, nil))...)
		}
		return bases
	}
	if item.Type() != "impl_item" && !slices.Contains(rustTypeItems, item.Type()) {
		return nil
	}
	var bases []string
	for _, sibling := range p.rustSiblings(item, p.rustItemName(item)) {
		if trait := p.rustTypeName(sibling.ChildByFieldName("trait"), nil); trait != "" && !slices.Contains(bases, trait) {
			bases = append(bases, trait)
		}
	}
	return bases
}

// rustFields returns the fields of the struct of a type that declare a
// type.
func rustFields(p *projection, item *sitter.Node) []fileindex.Typed {
	var fields []fileindex.Typed
	for _, sibling := range p.rustSiblings(item, p.rustItemName(item)) {
		if sibling.Type() != "struct_item" {
			continue
		}
		for _, field := range ts.NamedChildren(sibling.ChildByFieldName("body")) {
			if field.Type() != "field_declaration" {
				continue
			}
			if typ := p.rustScopedType(field, p.rustTypeName(field.ChildByFieldName("type"), nil)); typ != "" {
				fields = append(fields, fileindex.Typed{Name: p.text(field.ChildByFieldName("name")), Type: typ})
			}
		}
	}
	return fields
}

// rustTypeName returns the name of the type that a type node names, as
// Store for Store, &Store, &mut dyn Store, impl Store, Box<dyn Store> or
// Store<T>, and a::Store for a path. A type parameter names its first
// bound, as Store for S in fn run<S: Store>(s: S). A primitive type, a
// tuple and a slice name no type.
func (p *projection) rustTypeName(node *sitter.Node, bounds map[string]string) string {
	if node == nil {
		return ""
	}
	switch node.Type() {
	case "type_identifier":
		name := p.text(node)
		if bound, ok := bounds[name]; ok {
			return bound
		}
		return name
	case "scoped_type_identifier", "scoped_identifier":
		return p.text(node)
	case "generic_type":
		name := p.rustTypeName(node.ChildByFieldName("type"), bounds)
		if args := ts.NamedChildren(node.ChildByFieldName("type_arguments")); langcode.IsPointer(langcode.Rust, name) && len(args) > 0 {
			return p.rustTypeName(args[0], bounds)
		}
		return name
	case "reference_type", "pointer_type":
		return p.rustTypeName(node.ChildByFieldName("type"), bounds)
	case "dynamic_type", "abstract_type":
		return p.rustTypeName(node.ChildByFieldName("trait"), bounds)
	case "trait_bounds":
		for _, bound := range ts.NamedChildren(node) {
			if name := p.rustTypeName(bound, bounds); name != "" {
				return name
			}
		}
	}
	return ""
}

// rustWrappers are the types whose value ?, unwrap and expect give, as T
// of Option<T> and of Result<T, E>. A wrapper matches by the last name of
// its path, so std::io::Result<T> and anyhow::Result<T> wrap T too.
var rustWrappers = []string{"Option", "Result"}

// rustResult returns the type that a result names, and the Option or the
// Result that holds it, as Thing and Option for Option<Thing<u8>>.
func (p *projection) rustResult(node *sitter.Node, bounds map[string]string) (string, string) {
	if node != nil && node.Type() == "generic_type" {
		name := rustLastName(p.rustTypeName(node.ChildByFieldName("type"), bounds))
		if args := ts.NamedChildren(node.ChildByFieldName("type_arguments")); slices.Contains(rustWrappers, name) && len(args) > 0 {
			return p.rustTypeName(args[0], bounds), name
		}
	}
	return p.rustTypeName(node, bounds), ""
}

// rustBounds returns the first trait bound of each type parameter of a
// function and of its where clause.
func (p *projection) rustBounds(fn *sitter.Node) map[string]string {
	bounds := map[string]string{}
	add := func(name, bound *sitter.Node) {
		if b := p.rustTypeName(bound, nil); b != "" {
			if _, ok := bounds[p.text(name)]; !ok {
				bounds[p.text(name)] = b
			}
		}
	}
	for _, param := range ts.NamedChildren(fn.ChildByFieldName("type_parameters")) {
		if param.Type() == "constrained_type_parameter" {
			add(param.ChildByFieldName("left"), param.ChildByFieldName("bounds"))
		}
	}
	if where := ts.ChildOfType(fn, "where_clause"); where != nil {
		for _, predicate := range ts.NamedChildren(where) {
			add(predicate.ChildByFieldName("left"), predicate.ChildByFieldName("bounds"))
		}
	}
	return bounds
}

// rustAttributes returns the attributes before an item as written, with no
// arguments, as @tokio::main for #[tokio::main] or @get for
// #[get("/")]. A macro_rules! gets the mark macro_rules! too, so the link
// step can tell a macro from a function of the name.
func rustAttributes(p *projection, node *sitter.Node) []string {
	var attributes []string
	for sibling := node.PrevNamedSibling(); sibling != nil && sibling.Type() == "attribute_item"; sibling = sibling.PrevNamedSibling() {
		if attribute := ts.ChildOfType(sibling, "attribute"); attribute != nil && attribute.NamedChildCount() > 0 {
			attributes = append(attributes, "@"+p.text(attribute.NamedChild(0)))
		}
	}
	slices.Reverse(attributes)
	if node.Type() == "macro_definition" {
		attributes = append(attributes, fileindex.RustMacroRules)
	}
	return attributes
}

// rustSignature sets the static mark, the parameters, the typed locals and
// the result of a function. A function of an impl block or a trait with no
// self parameter is static. Self names the type of the impl block.
func rustSignature(p *projection, def *fileindex.Definition, node *sitter.Node) {
	if node.Type() != "function_item" {
		return
	}
	params := node.ChildByFieldName("parameters")
	if parent := p.defs[fileindex.Parent(def.Key)]; parent != nil && parent.Kind != fileindex.DefModule {
		def.Static = ts.ChildOfType(params, "self_parameter") == nil
	}
	bounds := p.rustBounds(node)
	if impl := p.syntax[fileindex.Parent(def.Key)]; impl != nil && impl.Type() == "impl_item" {
		bounds["Self"] = p.rustItemName(impl)
	}
	for _, param := range ts.NamedChildren(params) {
		if param.Type() != "parameter" {
			continue
		}
		name := rustBoundName(param.ChildByFieldName("pattern"))
		if typ := p.rustDeclaredType(param, bounds); typ != "" && name != nil {
			def.Params = append(def.Params, fileindex.Typed{Name: p.text(name), Type: typ})
		}
	}
	result := node.ChildByFieldName("return_type")
	if def.Returns, def.Wraps = p.rustResult(result, bounds); def.Returns != "" {
		if def.Returns = p.rustScopedType(result, def.Returns); def.Returns == "" {
			def.Wraps = ""
		}
	}
	def.Locals = p.rustLocals(node.ChildByFieldName("body"), bounds)
	if impl := rustImpl(node); impl != nil && !def.Static && !p.rustDefinesType(impl) {
		def.Params = append([]fileindex.Typed{{Name: extensionParam, Type: p.rustItemName(impl)}}, def.Params...)
	}
}

// rustImpl returns the impl block that holds a function.
func rustImpl(fn *sitter.Node) *sitter.Node {
	if list := fn.Parent(); list != nil && list.Type() == "declaration_list" {
		if impl := list.Parent(); impl != nil && impl.Type() == "impl_item" {
			return impl
		}
	}
	return nil
}

// rustDefinesType reports whether the scope of an impl block defines its
// type. An impl of a trait for a type of another crate, as impl Shout for
// String, or for any type, as impl<T: Display> Shout for T, adds methods
// that a call on a value of that type runs, as an extension function does.
// So its methods take the receiver as the parameter this.
func (p *projection) rustDefinesType(impl *sitter.Node) bool {
	return slices.ContainsFunc(p.rustSiblings(impl, p.rustItemName(impl)), func(item *sitter.Node) bool {
		return item.Type() != "impl_item"
	})
}

// rustLocals returns the local variables of a body that declare a type.
func (p *projection) rustLocals(body *sitter.Node, bounds map[string]string) []fileindex.Typed {
	var locals []fileindex.Typed
	walkBody(body, map[string]bool{"function_item": true}, func(node *sitter.Node) {
		if node.Type() != "let_declaration" {
			return
		}
		name := rustBoundName(node.ChildByFieldName("pattern"))
		if typ := p.rustDeclaredType(node, bounds); typ != "" && name != nil {
			locals = append(locals, fileindex.Typed{Name: p.text(name), Type: typ})
		}
	})
	return locals
}

// rustDeclaredType returns the type that a let or a parameter declares, or
// "" when a scope that holds the binder declares the first name of the
// type.
func (p *projection) rustDeclaredType(binder *sitter.Node, bounds map[string]string) string {
	return p.rustScopedType(binder, p.rustTypeName(binder.ChildByFieldName("type"), bounds))
}

// rustScopedType returns a type that a node names, as the link step reads
// it. The link step resolves Local at the root. A local type alias of the
// name, as in fn run() { type Local = dep::Thing<u8>; let t: Local }, must
// not take a struct Local of the root. So a type whose first name a scope
// of the node declares is "", or the path of a use that declares the name.
// docs/projectgraph.md has the rule.
func (p *projection) rustScopedType(node *sitter.Node, typ string) string {
	first, rest, nested := strings.Cut(typ, "::")
	if typ == "" || !p.rustScopeDeclares(node, nil, first) {
		return typ
	}
	use, isUse := p.rustRebinder(node, nil, first)
	if !isUse || p.rustScopeDeclares(node, use.Parent(), first) {
		return ""
	}
	namespace, ok := p.rustUseNamespace(use, first)
	if !ok {
		return ""
	}
	path := strings.ReplaceAll(namespace, fileindex.Separator, "::")
	if nested {
		path += "::" + rest
	}
	return path
}

// rustScopeDeclares reports whether a scope between a node and the root of
// the file, or a stop node, can give a name another meaning than the root gives it: an item
// or a use of a block or of an inline module, or a type parameter of a
// function, an impl, a trait or a type. A glob use can declare any name,
// except use super::* in a module of the root, which reads the names of the
// root. In an inline module, self and super name other modules than the
// root. A struct, an enum, a union or a trait of the inline module that
// declares the name first does not count, because the link step looks for
// a type of the name in the inline modules that hold the node.
func (p *projection) rustScopeDeclares(node, stop *sitter.Node, name string) bool {
	for scope := node.Parent(); scope != nil && (stop == nil || !ts.SameNode(scope, stop)); scope = scope.Parent() {
		switch scope.Type() {
		case "block":
			if p.rustItemsDeclare(scope, name, false) {
				return true
			}
		case "declaration_list":
			mod := scope.Parent()
			if mod == nil || mod.Type() != "mod_item" {
				continue
			}
			ofRoot := mod.Parent() != nil && mod.Parent().Type() == "source_file"
			if name == "self" || name == "super" {
				return true
			}
			if p.rustItemsDefineType(scope, name) {
				return false
			}
			if p.rustItemsDeclare(scope, name, ofRoot) {
				return true
			}
		case "function_item", "impl_item", "trait_item", "struct_item", "enum_item", "union_item":
			if slices.ContainsFunc(ts.NamedChildren(scope.ChildByFieldName("type_parameters")), func(param *sitter.Node) bool {
				return p.rustTypeParameter(param) == name
			}) {
				return true
			}
		}
	}
	return false
}

// rustItemsDefineType reports whether the items of a module define a
// struct, an enum, a union or a trait of the name. The index gives each one
// a key under the module.
func (p *projection) rustItemsDefineType(list *sitter.Node, name string) bool {
	return slices.ContainsFunc(ts.NamedChildren(list), func(item *sitter.Node) bool {
		switch item.Type() {
		case "struct_item", "enum_item", "union_item", "trait_item":
			return p.text(item.ChildByFieldName("name")) == name
		}
		return false
	})
}

// rustItemsDeclare reports whether the items of a block or of a module
// declare a type, a trait, a module or a use of the name. ofRoot tells
// that the module is a module of the root, whose use super::* reads the
// names of the root.
func (p *projection) rustItemsDeclare(list *sitter.Node, name string, ofRoot bool) bool {
	for _, item := range ts.NamedChildren(list) {
		switch item.Type() {
		case "struct_item", "enum_item", "union_item", "type_item", "trait_item", "mod_item":
			if p.text(item.ChildByFieldName("name")) == name {
				return true
			}
		case "extern_crate_declaration":
			bound := item.ChildByFieldName("alias")
			if bound == nil {
				bound = item.ChildByFieldName("name")
			}
			if p.text(bound) == name {
				return true
			}
		case "use_declaration":
			tree := item.ChildByFieldName("argument")
			names, glob := p.rustUseNames(tree, nil)
			readsRoot := ofRoot && p.text(tree) == "super::*"
			if slices.Contains(names, name) || glob && !readsRoot {
				return true
			}
		}
	}
	return false
}

// rustUseNames returns the names that a use tree binds, and whether it
// holds a glob. The prefix is the path of the use list that holds the
// tree, whose last name self binds, as io for std::io::{self}.
func (p *projection) rustUseNames(tree, prefix *sitter.Node) ([]string, bool) {
	if tree == nil {
		return nil, false
	}
	switch tree.Type() {
	case "use_as_clause":
		return []string{p.text(tree.ChildByFieldName("alias"))}, false
	case "use_wildcard":
		return nil, true
	case "scoped_use_list":
		return p.rustUseNames(tree.ChildByFieldName("list"), tree.ChildByFieldName("path"))
	case "use_list":
		var names []string
		glob := false
		for _, child := range ts.NamedChildren(tree) {
			n, g := p.rustUseNames(child, prefix)
			names, glob = append(names, n...), glob || g
		}
		return names, glob
	case "self":
		if prefix == nil {
			return nil, false
		}
		return []string{rustLastName(p.text(prefix))}, false
	case "identifier", "scoped_identifier":
		return []string{rustLastName(p.text(tree))}, false
	}
	return nil, false
}

// rustTypeParameter returns the name of a type parameter, as T for T,
// T: Store or T = u8, and "" for a lifetime or a const parameter.
func (p *projection) rustTypeParameter(param *sitter.Node) string {
	switch param.Type() {
	case "optional_type_parameter":
		return p.rustTypeParameter(param.ChildByFieldName("name"))
	case "constrained_type_parameter":
		return p.text(param.ChildByFieldName("left"))
	case "type_identifier":
		return p.text(param)
	}
	return ""
}

// rustLastName returns the last name of a path, as Result for
// std::io::Result.
func rustLastName(path string) string {
	if i := strings.LastIndex(path, "::"); i >= 0 {
		return path[i+len("::"):]
	}
	return path
}

// rustDeclaredAt reports whether the declared type of a name holds at a
// call: the function binds the name once, as a parameter or a let with a
// type, before the call. A let, a match arm, an if let, a for, a closure, a
// static, a const or a macro can bind the name again with another type.
func rustDeclaredAt(ctx context.Context, p *projection, call fileindex.Call, name string) bool {
	binders, _ := p.rustBinders(ctx, call.Caller)
	if len(binders[name]) != 1 {
		return false
	}
	binder := binders[name][0]
	if !startsBefore(binder, call) || !rustPlainBinder(binder) {
		return false
	}
	return binder.Type() == "parameter" || binder.ChildByFieldName("type") != nil
}

// rustRebound reports whether the function binds a name before a call by a
// binder that the call graph does not read, or by a second let of another
// type. A static or a const binds its name in the whole block, also before
// it. The call graph binds a name to the value of its first let, so it sends
// t.run() after let t = B to A::run when let t = A comes first. A let counts
// once its statement ends, because the call in its value, as c.with_retry()
// in let c = c.with_retry(), still sees the binding before it. Lets of one
// type, as let c = c.with_retry() after let c = Client::new(), keep the
// target. A function whose walk stopped early can bind any name.
func rustRebound(ctx context.Context, p *projection, call fileindex.Call, name string) bool {
	binders, ok := p.rustBinders(ctx, call.Caller)
	if !ok {
		return true
	}
	var lets []*sitter.Node
	for _, binder := range binders[name] {
		item := binder.Type() == "static_item" || binder.Type() == "const_item"
		switch {
		case !item && !startsBefore(binder, call):
			continue
		case !rustPlainBinder(binder):
			return true
		case binder.Type() == "let_declaration" && endsBefore(binder, call):
			lets = append(lets, binder)
		}
	}
	if len(lets) < 2 {
		return false
	}
	first := p.rustLetType(binders, lets[0], 0)
	return first == "" || slices.ContainsFunc(lets[1:], func(let *sitter.Node) bool {
		return p.rustLetType(binders, let, 0) != first
	})
}

// endsBefore reports whether a node ends before the site of a call.
func endsBefore(node *sitter.Node, call fileindex.Call) bool {
	at := node.EndPoint()
	line, column := int(at.Row)+1, int(at.Column)+1
	return line < call.Line || line == call.Line && column <= call.Column
}

// rustMaxLetDepth bounds the lets that the type of one let reads, as c of
// let c = c.with_retry().
const rustMaxLetDepth = 4

// rustLetType returns the name of the type of the value of a let, as the
// file writes it, or "" when the index does not know it: the declared type,
// a struct, a function or a method of the file with a result, or the let of
// a name that the value reads.
func (p *projection) rustLetType(binders map[string][]*sitter.Node, let *sitter.Node, depth int) string {
	if typ := p.rustTypeName(let.ChildByFieldName("type"), nil); typ != "" {
		return typ
	}
	return p.rustValueType(binders, let.ChildByFieldName("value"), depth)
}

func (p *projection) rustValueType(binders map[string][]*sitter.Node, value *sitter.Node, depth int) string {
	if value == nil || depth > rustMaxLetDepth {
		return ""
	}
	switch value.Type() {
	case "struct_expression":
		return p.rustTypeName(value.ChildByFieldName("name"), nil)
	case "identifier":
		if name := p.text(value); name != "" && unicode.IsUpper(rune(name[0])) {
			return name
		}
		if let := rustLetOf(binders[p.text(value)], value); let != nil {
			return p.rustLetType(binders, let, depth+1)
		}
	case "call_expression":
		fn := value.ChildByFieldName("function")
		if method := rustMethod(fn); method != nil {
			if typ := p.rustValueType(binders, method.ChildByFieldName("value"), depth+1); typ != "" {
				return p.rustResultOf(typ, p.text(method.ChildByFieldName("field")))
			}
			return ""
		}
		if fn != nil && fn.Type() == "scoped_identifier" {
			path := p.text(fn.ChildByFieldName("path"))
			if typ := p.rustResultOf(path, p.text(fn.ChildByFieldName("name"))); typ != "" {
				return typ
			}
			return rustConstructorType(path, p.text(fn))
		}
	}
	return ""
}

// rustGenericTraits are the traits of std whose associated function gives
// whatever type the caller infers, as Default::default().
var rustGenericTraits = map[string]bool{"Default": true, "From": true, "Into": true, "FromStr": true, "TryFrom": true, "FromIterator": true}

// rustConstructorType returns a function of a type as a type of its own,
// as A::new for let t = A::new() when A is a type of another file. Two lets
// whose values call the same function of the same type hold one type. A
// trait of std that gives an inferred type names no type.
func rustConstructorType(typ, fn string) string {
	last := rustLastName(typ)
	if last == "" || !unicode.IsUpper(rune(last[0])) || rustGenericTraits[last] {
		return ""
	}
	return fn + "()"
}

// rustResultOf returns the result type of a function of a type of the
// file, with no Option or Result around it.
func (p *projection) rustResultOf(typ, name string) string {
	def := p.defs[fileindex.Join(p.cg.FileName, append(strings.Split(typ, "::"), name)...)]
	if def == nil || def.Wraps != "" {
		return ""
	}
	return def.Returns
}

// startsBefore reports whether a node starts before the site of a call.
func startsBefore(node *sitter.Node, call fileindex.Call) bool {
	at := node.StartPoint()
	line, column := int(at.Row)+1, int(at.Column)+1
	return line < call.Line || line == call.Line && column < call.Column
}

// rustPlainBinder reports whether the call graph reads a binder: a
// parameter of the function or a let, that binds one name.
func rustPlainBinder(binder *sitter.Node) bool {
	switch binder.Type() {
	case "parameter":
		if parent := binder.Parent(); parent == nil || parent.Type() != "parameters" {
			return false
		}
	case "let_declaration":
	default:
		return false
	}
	return rustBoundName(binder.ChildByFieldName("pattern")) != nil
}

// rustBinders returns the binders of each name of the function of a caller,
// in source order. A nested function binds its own names. It returns false
// when the walk of the function stops early, so its binders are not known.
// A caller that is no function binds no name.
func (p *projection) rustBinders(ctx context.Context, caller string) (map[string][]*sitter.Node, bool) {
	fn := p.syntax[caller]
	if fn == nil || fn.Type() != "function_item" {
		return nil, true
	}
	if binders, ok := p.binders[fn.StartByte()]; ok {
		return binders, binders != nil
	}
	binders := map[string][]*sitter.Node{}
	err := ts.Walk(ctx, fn, ts.WalkOptions{}, func(node *sitter.Node) bool {
		if node.Type() == "function_item" && !ts.SameNode(node, fn) {
			return false
		}
		if binder := p.rustBinder(node); binder != nil {
			binders[p.text(node)] = append(binders[p.text(node)], binder)
		}
		return true
	})
	if err != nil {
		p.truncateWalk(err)
		binders = nil
	}
	if p.binders == nil {
		p.binders = map[uint32]map[string][]*sitter.Node{}
	}
	p.binders[fn.StartByte()] = binders
	return binders, binders != nil
}

// rustBinder returns the node that binds an identifier: the let, the
// parameter, the match pattern or another pattern, the static or the
// const, or the call of a macro whose tokens hold it. It returns nil for an
// identifier that binds nothing and for any other node.
func (p *projection) rustBinder(id *sitter.Node) *sitter.Node {
	if id.Type() != "identifier" && id.Type() != "shorthand_field_identifier" {
		return nil
	}
	child, parent := id, id.Parent()
	for parent != nil && strings.HasSuffix(parent.Type(), "_pattern") && parent.Type() != "match_pattern" {
		if typ := parent.ChildByFieldName("type"); typ != nil && ts.SameNode(typ, child) {
			return nil
		}
		child, parent = parent, parent.Parent()
	}
	if parent == nil {
		return nil
	}
	switch parent.Type() {
	case "match_pattern", "closure_parameters":
		return parent
	case "let_declaration", "for_expression", "let_condition", "parameter":
		if pattern := parent.ChildByFieldName("pattern"); pattern != nil && ts.SameNode(pattern, child) {
			return parent
		}
	case "static_item", "const_item":
		if name := parent.ChildByFieldName("name"); name != nil && ts.SameNode(name, child) {
			return parent
		}
	case "token_tree":
		return p.rustBindingMacro(parent)
	}
	return nil
}

// rustBindingMacro returns the call of the macro that holds a token tree,
// when the macro can bind a name of its caller, as bind!(t) with the rule
// ($n:ident) => { let $n = dep::new_thing(); }. A macro of std binds no
// name of its caller, except thread_local!, which makes a static. A
// macro_rules! or a use of the file can give a name of std to another
// macro, which can bind a name.
func (p *projection) rustBindingMacro(tree *sitter.Node) *sitter.Node {
	for tree != nil && tree.Type() == "token_tree" {
		tree = tree.Parent()
	}
	if tree == nil || tree.Type() != "macro_invocation" {
		return nil
	}
	name := rustLastName(p.text(tree.ChildByFieldName("macro")))
	if langcode.IsBuiltin(langcode.Rust, name) && name != "thread_local" && !p.rustOwnMacro(name) {
		return nil
	}
	return tree
}

// rustOwnMacro reports whether the file defines a macro of the name with
// macro_rules! or binds the name with a use.
func (p *projection) rustOwnMacro(name string) bool {
	if p.rustMacros == nil {
		p.rustMacros = maps.Clone(p.bindings)
		for key, node := range p.syntax {
			if node.Type() == "macro_definition" {
				p.rustMacros[fileindex.Last(key)] = true
			}
		}
	}
	return p.rustMacros[name]
}

// rustBoundName returns the name that a pattern binds, as t for t or for
// mut t, and nil for any other pattern.
func rustBoundName(pattern *sitter.Node) *sitter.Node {
	if pattern != nil && pattern.Type() == "mut_pattern" && pattern.NamedChildCount() > 0 {
		pattern = pattern.NamedChild(0)
	}
	if pattern == nil || pattern.Type() != "identifier" {
		return nil
	}
	return pattern
}

// rustShadowed fixes a target that the call graph finds for the path of a
// call in an outer scope, when a scope between the call and that scope
// binds the first name of the path again, as util::g() after use
// crate::deep::util in a block. The call graph reads no use, so it gives
// util::g() the module util of the file. A use gives the target through
// the path of the use. A type parameter names a type that the call does not
// know, so the call is a record.
func rustShadowed(p *projection, call *fileindex.Call, callee ast.NodePosition) bool {
	node := p.descendant(callee)
	if node != nil && rustMethod(node) != nil {
		return p.rustShadowedMethod(call, node)
	}
	if node == nil || node.Type() != "identifier" && node.Type() != "scoped_identifier" {
		return true
	}
	names := strings.Split(p.text(node), "::")
	switch names[0] {
	case "crate", "self", "super", "Self", "":
		return true
	}
	written := strings.Join(names, fileindex.Separator)
	for i, target := range call.Targets {
		scope, ok := strings.CutSuffix(target, fileindex.Separator+written)
		if !ok || !fileindex.HasKeyPrefix(scope, p.cg.FileName) {
			continue
		}
		rebound, ok := p.rustUseTarget(node, p.syntax[scope], names[0], names[1:]...)
		if !ok {
			return false
		}
		if rebound != "" {
			call.Targets[i] = rebound
		}
	}
	return true
}

// rustShadowedMethod fixes a target of a method call that the call graph
// finds as a method of a type of an outer scope, when a scope between the
// call and that scope binds the name of the type again, as x.save() after
// let x = Item::new() and use crate::store::Item in mod deep. The call graph
// reads no use, so it gives x.save() the Item of the parent of deep.
func (p *projection) rustShadowedMethod(call *fileindex.Call, node *sitter.Node) bool {
	for i, target := range call.Targets {
		class := fileindex.Parent(target)
		if def := p.defs[class]; def == nil || def.Kind != fileindex.DefClass || !fileindex.HasKeyPrefix(class, p.cg.FileName) {
			continue
		}
		rebound, ok := p.rustUseTarget(node, p.syntax[fileindex.Parent(class)], fileindex.Last(class), fileindex.Last(target))
		if !ok {
			return false
		}
		if rebound != "" {
			call.Targets[i] = rebound
		}
	}
	return true
}

// rustUseTarget returns the target of the names under a name that a use
// between a node and a scope binds again, or "" when no scope between them
// binds the name. It reports false when an item or a type parameter binds
// the name, because the call then names a type that it does not know.
func (p *projection) rustUseTarget(node, scope *sitter.Node, name string, rest ...string) (string, bool) {
	binder, use := p.rustRebinder(node, scope, name)
	switch {
	case binder == nil:
		return "", true
	case !use:
		return "", false
	}
	namespace, ok := p.rustUseNamespace(binder, name)
	if !ok {
		return "", false
	}
	return fileindex.Join(namespace, rest...), true
}

// rustRebinder returns the innermost use or item with type parameters
// between a node and the definition of a scope that binds a name, and
// whether it is a use. A nil scope is the file.
func (p *projection) rustRebinder(node, scope *sitter.Node, name string) (*sitter.Node, bool) {
	for n := node.Parent(); n != nil && (scope == nil || !ts.SameNode(n, scope)); n = n.Parent() {
		switch n.Type() {
		case "block", "declaration_list":
			var globbed *sitter.Node
			for _, item := range ts.NamedChildren(n) {
				if item.Type() != "use_declaration" {
					continue
				}
				names, glob := p.rustUseNames(item.ChildByFieldName("argument"), nil)
				if slices.Contains(names, name) {
					return item, true
				}
				if glob && !p.rustOuterGlob(item) {
					globbed = item
				}
			}
			if globbed != nil {
				return globbed, false
			}
		case "function_item", "impl_item", "trait_item":
			if slices.ContainsFunc(ts.NamedChildren(n.ChildByFieldName("type_parameters")), func(param *sitter.Node) bool {
				return p.rustTypeParameter(param) == name
			}) {
				return n, false
			}
		}
	}
	return nil, false
}

// rustOuterGlob reports whether a glob use brings no name that the call
// graph would find in an outer scope instead: a glob of super or self, which
// brings the names of the scopes that hold it, or of std, core or alloc,
// which define no item of the crate. Any other glob use can bring the name,
// as A of use crate::other::* in mod n inside mod m with a struct A.
func (p *projection) rustOuterGlob(use *sitter.Node) bool {
	first := rustFirstName(p.text(use.ChildByFieldName("argument")))
	return first == "super" || first == "self" || first == "std" || first == "core" || first == "alloc"
}

// rustFirstName returns the first name of a path, as std for ::std::io.
func rustFirstName(path string) string {
	first, _, _ := strings.Cut(strings.TrimPrefix(path, "::"), "::")
	return first
}

// rustUseNamespace returns the namespace of the import of a use that binds
// a name.
func (p *projection) rustUseNamespace(use *sitter.Node, name string) (string, bool) {
	first, last := lineOf(use), int(use.EndPoint().Row)+1
	for _, imp := range p.ix.Imports {
		if imp.Line >= first && imp.Line <= last && imp.Alias == name && imp.Namespace != "" {
			return imp.Namespace, true
		}
	}
	return "", false
}

// rustMarkTries marks each ? in the receiver of a method call in the
// targets of the call, as dep//load//?//run for dep::load()?.run() and for
// t.run() after let t = dep::load()?. The call graph reads through a ?, so
// without the mark the link step cannot tell a method of the value inside
// an Option or a Result from a method of the wrapper, as inspect of
// Option.
func rustMarkTries(ctx context.Context, p *projection, call *fileindex.Call, callee ast.NodePosition) {
	node := p.descendant(callee)
	if node == nil || node.StartByte() != callee.StartByte || node.EndByte() != callee.EndByte {
		return
	}
	method := rustMethod(node)
	if method == nil {
		return
	}
	binders, _ := p.rustBinders(ctx, call.Caller)
	marks := rustAfter(p.rustTries(binders, method.ChildByFieldName("value")))
	for i, target := range call.Targets {
		call.Targets[i] = withTries(target, marks)
	}
}

// rustTries returns a mark for each ? of a receiver: the count of method
// names after it. A name reads the value of its let.
func (p *projection) rustTries(binders map[string][]*sitter.Node, node *sitter.Node) []int {
	if node == nil {
		return nil
	}
	switch node.Type() {
	case "try_expression":
		return append(p.rustTries(binders, node.NamedChild(0)), 0)
	case "parenthesized_expression", "await_expression", "reference_expression":
		return p.rustTries(binders, node.NamedChild(0))
	case "identifier":
		if let := rustLetOf(binders[p.text(node)], node); let != nil {
			return p.rustTries(binders, let.ChildByFieldName("value"))
		}
	case "call_expression":
		if method := rustMethod(node.ChildByFieldName("function")); method != nil {
			return rustAfter(p.rustTries(binders, method.ChildByFieldName("value")))
		}
	}
	return nil
}

// rustLetOf returns the let of one name, among the binders of the name,
// that binds it at a use: the last one that ends before the use, in a
// block that holds the use.
func rustLetOf(binders []*sitter.Node, use *sitter.Node) *sitter.Node {
	var out *sitter.Node
	for _, let := range binders {
		if let.Type() != "let_declaration" || rustBoundName(let.ChildByFieldName("pattern")) == nil {
			continue
		}
		if block := let.Parent(); let.EndByte() <= use.StartByte() && block != nil && use.EndByte() <= block.EndByte() {
			out = let
		}
	}
	return out
}

// rustMethod returns the field expression of a method call, as
// t.run of t.run() or of t.run::<u8>(). Each method call adds one name to
// a target. A field that no call reads, as t.store in t.store.run(), keeps
// no ? of its value, so the walk of the link step gives no type there.
func rustMethod(fn *sitter.Node) *sitter.Node {
	if fn != nil && fn.Type() == "generic_function" {
		fn = fn.ChildByFieldName("function")
	}
	if fn == nil || fn.Type() != "field_expression" || fn.ChildByFieldName("value") == nil {
		return nil
	}
	return fn
}

// rustAfter moves the marks of a receiver past the name of the method that
// a call on it adds.
func rustAfter(marks []int) []int {
	for i := range marks {
		marks[i]++
	}
	return marks
}

// withTries puts a ? into a target before the count of names of each mark,
// counted from the end.
func withTries(target string, marks []int) string {
	if len(marks) == 0 {
		return target
	}
	names := strings.Split(target, fileindex.Separator)
	n := len(names)
	marks = slices.Clone(marks)
	slices.Sort(marks)
	for _, m := range slices.Compact(marks) {
		if at := n - m; at > 0 {
			names = slices.Insert(names, at, fileindex.TryName)
		}
	}
	return strings.Join(names, fileindex.Separator)
}

// rustPublic reports whether an item is pub. A method of a trait impl is
// as public as the trait.
func rustPublic(p *projection, def *fileindex.Definition) bool {
	node := p.syntax[def.Key]
	if node == nil {
		return false
	}
	if node.Type() == "impl_item" {
		return slices.ContainsFunc(p.rustSiblings(node, p.rustItemName(node)), func(item *sitter.Node) bool {
			return item.Type() != "impl_item" && p.rustPub(item)
		})
	}
	if p.rustPub(node) {
		return true
	}
	if impl := rustImpl(node); impl != nil {
		return impl.ChildByFieldName("trait") != nil
	}
	parent := p.syntax[fileindex.Parent(def.Key)]
	return parent != nil && parent.Type() == "trait_item"
}

// rustPub reports whether an item is pub for every crate. pub(crate),
// pub(super) and pub(in path) keep the item in its crate, so another crate
// cannot name it, and a glob use of another crate does not bring it.
func (p *projection) rustPub(item *sitter.Node) bool {
	return p.text(ts.ChildOfType(item, "visibility_modifier")) == "pub"
}

// rustImportExported reports whether a use is pub, as pub use
// inner::render, so another crate can name what it binds.
func rustImportExported(p *projection, imp callgraph.Import) bool {
	for node := p.descendant(imp.Position); node != nil; node = node.Parent() {
		if node.Type() == "use_declaration" {
			return p.rustPub(node)
		}
	}
	return false
}

// rustVisitor marks an include! of a source file, whose code the index
// does not read, as a computed import. It adds a reference to the type of a
// let and to the error type of the function of a ?, which converts an
// error with the From of that type. Rust runs the Drop, the Display and the
// From of a type with no call of their names.
func rustVisitor(p *projection) visitor {
	return func(node *sitter.Node, kind string) {
		switch kind {
		case "macro_invocation":
			if p.text(node.ChildByFieldName("macro")) == "include" {
				p.flag(fileindex.FlagComputedImport, node)
			}
		case "let_declaration":
			p.addRustTypeReference(node, node.ChildByFieldName("type"))
		case "try_expression":
			for fn := node.Parent(); fn != nil; fn = fn.Parent() {
				if fn.Type() == "function_item" || fn.Type() == "closure_expression" {
					if result := fn.ChildByFieldName("return_type"); result != nil && result.Type() == "generic_type" {
						if args := ts.NamedChildren(result.ChildByFieldName("type_arguments")); len(args) > 0 {
							p.addRustTypeReference(node, args[len(args)-1])
						}
					}
					break
				}
			}
		}
	}
}

// addRustTypeReference adds a reference from the code of the node to the
// type that a type node names.
func (p *projection) addRustTypeReference(node, typ *sitter.Node) {
	name := strings.ReplaceAll(p.rustTypeName(typ, nil), "::", fileindex.Separator)
	first, rest, _ := strings.Cut(name, fileindex.Separator)
	if name == "" || name == "Self" || langcode.IsBuiltin(langcode.Rust, first) {
		return
	}
	caller := p.innermost(node.StartByte())
	target := name
	if targets := p.rebind(caller, first); len(targets) > 0 {
		target = fileindex.Join(targets[0], nonEmpty(rest)...)
	}
	for _, imp := range p.ix.Imports {
		if imp.Alias == first && imp.Namespace != "" {
			target = fileindex.Join(imp.Namespace, nonEmpty(rest)...)
		}
	}
	p.addReference(typ, caller, []string{target})
}
