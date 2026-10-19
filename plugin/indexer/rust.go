package indexer

import (
	"slices"
	"strings"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/pkg/ts"
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
	bases:       rustBases,
	fields:      rustFields,
	decorators:  rustAttributes,
	signature:   rustSignature,
	exported:    rustPublic,
	definitions: rustTypes,
	visitors:    []func(p *projection) visitor{referenceVisitor, rustVisitor},
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

// rustPointers hold a value whose methods a call reaches through them, as
// s.save() on a Box<dyn Store>.
var rustPointers = []string{"Box", "Rc", "Arc"}

// rustTypes adds each struct, enum and union that no impl block of its
// scope names, so that a path names the type. An impl block of the type
// is the class of the type, and its node gives the methods.
func rustTypes(p *projection) {
	var walk func(scope string, list *sitter.Node)
	walk = func(scope string, list *sitter.Node) {
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
				p.defs[key] = &fileindex.Definition{
					Key:       key,
					Kind:      fileindex.DefClass,
					Name:      fileindex.Last(key),
					StartLine: lineOf(item),
					EndLine:   int(item.EndPoint().Row) + 1,
				}
				p.addSyntax(key, item)
			}
		}
	}
	walk(p.cg.FileName, p.tree.Tree().RootNode())
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
			if typ := p.rustTypeName(field.ChildByFieldName("type"), nil); typ != "" {
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
		if args := ts.NamedChildren(node.ChildByFieldName("type_arguments")); slices.Contains(rustPointers, name) && len(args) > 0 {
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

// rustWrappers are the types of std whose value ?, unwrap and expect give,
// as T of Option<T> and of Result<T, E>.
var rustWrappers = []string{"Option", "Result"}

// rustResult returns the type that a result names, and the Option or the
// Result that holds it, as Thing and Option for Option<Thing<u8>>.
func (p *projection) rustResult(node *sitter.Node, bounds map[string]string) (string, string) {
	if node != nil && node.Type() == "generic_type" {
		name := p.rustTypeName(node.ChildByFieldName("type"), bounds)
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
// #[get("/")].
func rustAttributes(p *projection, node *sitter.Node) []string {
	var attributes []string
	for sibling := node.PrevNamedSibling(); sibling != nil && sibling.Type() == "attribute_item"; sibling = sibling.PrevNamedSibling() {
		if attribute := ts.ChildOfType(sibling, "attribute"); attribute != nil && attribute.NamedChildCount() > 0 {
			attributes = append(attributes, "@"+p.text(attribute.NamedChild(0)))
		}
	}
	slices.Reverse(attributes)
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
		name := param.ChildByFieldName("pattern")
		if name != nil && name.Type() == "mut_pattern" && name.NamedChildCount() > 0 {
			name = name.NamedChild(0)
		}
		if typ := p.rustTypeName(param.ChildByFieldName("type"), bounds); typ != "" && name != nil && name.Type() == "identifier" {
			def.Params = append(def.Params, fileindex.Typed{Name: p.text(name), Type: typ})
		}
	}
	def.Returns, def.Wraps = p.rustResult(node.ChildByFieldName("return_type"), bounds)
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
		name := node.ChildByFieldName("pattern")
		if name != nil && name.Type() == "mut_pattern" && name.NamedChildCount() > 0 {
			name = name.NamedChild(0)
		}
		if typ := p.rustTypeName(node.ChildByFieldName("type"), bounds); typ != "" && name != nil && name.Type() == "identifier" {
			locals = append(locals, fileindex.Typed{Name: p.text(name), Type: typ})
		}
	})
	return locals
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
			return item.Type() != "impl_item" && ts.ChildOfType(item, "visibility_modifier") != nil
		})
	}
	if ts.ChildOfType(node, "visibility_modifier") != nil {
		return true
	}
	if impl := rustImpl(node); impl != nil {
		return impl.ChildByFieldName("trait") != nil
	}
	parent := p.syntax[fileindex.Parent(def.Key)]
	return parent != nil && parent.Type() == "trait_item"
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
