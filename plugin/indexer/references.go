package indexer

import (
	"slices"

	"github.com/safedep/code/core/ast"
	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/pkg/ts"
	"github.com/safedep/code/plugin/callgraph"
	sitter "github.com/smacker/go-tree-sitter"
)

// maxReferenceSegments bounds the member chain of a reference, as a.b.c.
const maxReferenceSegments = 8

// referenceSyntax names the node types of one language that hold a value.
// A holder maps to the field of its value, or to "" when each named child
// is a value.
type referenceSyntax struct {
	holders          map[string]string
	member           string
	object, property string
	identifiers      map[string]bool
	instances        map[string]bool
	// logical holds the operators of a binary node whose operands are
	// values, as a || b.
	logical map[string]bool
	// bases maps the type of a class to the field that names its bases, as
	// the argument list of class A(Base). A base is no value.
	bases map[string]string
	// lists hold values with no name of their own, as the expression list
	// of a, b := f, g. The last child of each list holds the value, so the
	// key of a keyed element is no value.
	lists map[string]bool
	// spreads maps a holder to the node type of a spread in it, as
	// {...handlers} in <button {...handlers}>. The spread passes each
	// property of an object, so each function of the object is a value.
	spreads map[string]string
	// intrinsic reports whether a value of the holder names no code of the
	// file by a rule of the language, as the tag div of a JSX element.
	intrinsic func(p *projection, holder, value *sitter.Node) bool
	// rawNames marks a language whose files name the classes of other
	// files of a unit with no import, as C#. A name that the file does not
	// bind keeps its text, and the link step resolves it or drops it.
	rawNames bool
	// namedOnly holds the holders whose value counts only when the index
	// names its function, as the expression of a Go range. A range over a
	// member that the index cannot name, as n.Attr, mostly reads a slice or
	// a map, and a record with the method name would give it a name edge
	// to each method of the name.
	namedOnly map[string]bool
	// bareValues marks a language whose record of a method value keeps no
	// receiver, as Go. The link step then tells it from a call, which can
	// run a function value of a field.
	bareValues bool
	// value replaces reference for a language whose function values take
	// more forms than a name or a member chain, as Go. It gives the member
	// node of a method value whose receiver the index cannot name, and the
	// visitor records the method name of that member.
	value func(p *projection, caller string, value *sitter.Node) (target string, member *sitter.Node)
}

// referenceVisitor adds each function, method, class or import that the
// file uses as a value and does not call, as yaml.load in run(yaml.load).
// The code that gets the value can call it. A name in an export statement
// or a CommonJS export is an export record, not a reference.
func referenceVisitor(p *projection) visitor {
	syntax := p.rules().references
	if syntax == nil {
		return nil
	}
	kinds := p.rules().kinds
	type open struct {
		key string
		end uint32
	}
	stack := []open{{key: p.cg.FileName, end: ^uint32(0)}}
	type site struct {
		line, column int
		caller, text string
	}
	seen := map[site]bool{}
	// bases holds the start of each list of class bases. The walk visits a
	// class before its bases, and a parent lookup costs a walk from the root.
	bases := map[uint32]bool{}
	return func(node *sitter.Node, kind string) {
		start := node.StartByte()
		for len(stack) > 1 && start >= stack[len(stack)-1].end {
			stack = stack[:len(stack)-1]
		}
		if _, definition := kinds[kind]; definition {
			if key, ok := p.spans[span{start, node.EndByte(), kind}]; ok {
				stack = append(stack, open{key: key, end: node.EndByte()})
			}
		}
		if field, ok := syntax.bases[kind]; ok {
			if list := node.ChildByFieldName(field); list != nil {
				bases[list.StartByte()] = true
			}
		}
		field, ok := syntax.holders[kind]
		switch {
		case !ok || bases[start]:
			return
		case kind == "binary_expression" && !syntax.logical[p.text(node.ChildByFieldName("operator"))]:
			return
		}
		values := ts.NamedChildren(node)
		if field != "" {
			values = []*sitter.Node{node.ChildByFieldName(field)}
		}
		caller := stack[len(stack)-1].key
		for _, value := range syntax.unlist(values) {
			if spread, ok := syntax.spreads[kind]; ok && value.Type() == spread {
				p.addSpread(*syntax, caller, value)
				continue
			}
			if syntax.intrinsic != nil && syntax.intrinsic(p, node, value) {
				continue
			}
			// A read of a field, a constant or a variable of data runs
			// no code and gives the reader no function.
			target, member := p.valueTarget(*syntax, caller, value)
			if def := p.defs[target]; target == "" && member == nil || def != nil && def.HoldsData() {
				continue
			}
			at := ast.GetNodePosition(value)
			k := site{int(at.StartLine), int(at.StartColumn), caller, p.text(value)}
			switch {
			case seen[k]:
			case target != "":
				p.addReference(value, caller, p.targets([]string{target}))
			case !syntax.namedOnly[kind]:
				p.addValueRecord(member, caller)
			}
			seen[k] = true
		}
	}
}

// addReference adds a reference from the definition that runs the code at
// the node to the targets. The caller is the definition that holds the
// node, and a static initializer or a property function that the call
// graph does not know takes it from there, as a call does. The text is
// bounded as the text of a record is.
func (p *projection) addReference(node *sitter.Node, caller string, targets []string) {
	site := ast.GetNodePosition(node)
	p.ix.References = append(p.ix.References, fileindex.Reference{
		Caller:  p.caller(caller, site),
		Line:    int(site.StartLine),
		Column:  int(site.StartColumn),
		Text:    callgraph.ClipText(p.text(node)),
		Targets: targets,
	})
}

// addSpread adds the functions of the object that a spread passes, as
// handlers in <button {...handlers}>. React makes each property of the
// object a prop of the element and can call each function. The index names
// the functions that a definition of the file holds, as handlers//onClick.
// The index cannot name the functions of any other object, as a parameter,
// the result of a call or an object with no function definition. A
// function of such an object can have no reference, so the site gets a
// computed-name record, which concerns every target.
func (p *projection) addSpread(syntax referenceSyntax, caller string, spread *sitter.Node) {
	operand := spread.NamedChild(0)
	if target, ok := p.reference(syntax, caller, operand); ok {
		if functions := p.functionsOf(target); len(functions) > 0 {
			p.addReference(operand, caller, p.targets(functions))
			return
		}
	}
	site := ast.GetNodePosition(spread)
	p.ix.Unresolved = append(p.ix.Unresolved, fileindex.Unresolved{
		Caller: p.caller(caller, site),
		Line:   int(site.StartLine),
		Column: int(site.StartColumn),
		Text:   callgraph.ClipText(p.text(spread)),
		Reason: fileindex.UnresolvedComputedName,
	})
}

// functionsOf returns the functions and the methods that the definition of
// the key holds directly, as handlers//onClick of handlers.
func (p *projection) functionsOf(key string) []string {
	var functions []string
	for k, def := range p.defs {
		if fileindex.Parent(k) == key && (def.Kind == fileindex.DefFunction || def.Kind == fileindex.DefMethod) {
			functions = append(functions, k)
		}
	}
	slices.Sort(functions)
	return functions
}

// valueTarget returns the per-file target of a value, or the member node
// of a method value whose receiver the index cannot name.
func (p *projection) valueTarget(syntax referenceSyntax, caller string, value *sitter.Node) (string, *sitter.Node) {
	if syntax.value != nil {
		return syntax.value(p, caller, value)
	}
	target, _ := p.reference(syntax, caller, value)
	return target, nil
}

// addValueRecord records a method value whose receiver the index cannot
// name, as ts[0].Close. The code that gets the value can call any method of
// the name, so the record keeps the method name, as the record of a call
// on such a receiver does. Its site is the method name, so it never shares
// the site of a call that the receiver makes.
func (p *projection) addValueRecord(member *sitter.Node, caller string) {
	syntax := p.rules().references
	name := member.ChildByFieldName(syntax.property)
	site := ast.GetNodePosition(name)
	receiver := callgraph.ClipText(p.text(member.ChildByFieldName(syntax.object)))
	if syntax.bareValues {
		receiver = ""
	}
	p.ix.Unresolved = append(p.ix.Unresolved, fileindex.Unresolved{
		Caller:   p.caller(caller, site),
		Line:     int(site.StartLine),
		Column:   int(site.StartColumn),
		Text:     callgraph.ClipText(p.text(member)),
		Receiver: receiver,
		Method:   p.text(name),
		Reason:   fileindex.UnresolvedUnknownReceiver,
	})
}

// unlist replaces each list of the values with the value that it holds.
func (syntax referenceSyntax) unlist(values []*sitter.Node) []*sitter.Node {
	var out []*sitter.Node
	for _, value := range values {
		for depth := 0; value != nil && syntax.lists[value.Type()] && depth < maxReferenceSegments; depth++ {
			children := ts.NamedChildren(value)
			if value.Type() == "expression_list" {
				out = append(out, syntax.unlist(children)...)
				value = nil
				break
			}
			value = nil
			if len(children) > 0 {
				value = children[len(children)-1]
			}
		}
		if value != nil {
			out = append(out, value)
		}
	}
	return out
}

// reference returns the per-file target of a value. The value is a name
// or a member chain, and a definition, an import or the instance of a
// class binds its first name.
func (p *projection) reference(syntax referenceSyntax, caller string, value *sitter.Node) (string, bool) {
	// A name in the body of a function that an export statement declares
	// is a value of that function, not the export.
	if value == nil || caller == p.cg.FileName && p.inExport(value) && syntax.identifiers[value.Type()] {
		return "", false
	}
	var names []string
	root := value
	for root != nil && root.Type() == syntax.member && len(names) < maxReferenceSegments {
		property := root.ChildByFieldName(syntax.property)
		if property == nil {
			return "", false
		}
		names = append(names, p.text(property))
		root = root.ChildByFieldName(syntax.object)
	}
	if root == nil || !syntax.identifiers[root.Type()] && !syntax.instances[p.text(root)] {
		return "", false
	}
	slices.Reverse(names)

	unit, ok := p.valueOf(syntax, caller, p.text(root))
	switch {
	case !ok && syntax.rawNames && syntax.identifiers[root.Type()]:
		unit = p.text(root)
	case !ok:
		return "", false
	}
	// A Go parameter or local of a declared type holds a value of the
	// type, and a value is no function.
	if _, typed := p.typedScope(caller, p.text(root)); typed && len(names) == 0 {
		return "", false
	}
	return fileindex.Join(unit, names...), true
}

// valueOf returns the per-file target of a name: the definition that the
// scope chain binds, the namespace of an import, or the class of self or
// this. A typed parameter or local variable of a Go function is a name
// under the function, as the call graph keys it. Any other local variable
// or parameter has no target.
func (p *projection) valueOf(syntax referenceSyntax, caller, name string) (string, bool) {
	if syntax.instances[name] {
		for scope := caller; scope != p.cg.FileName; scope = fileindex.Parent(scope) {
			if def := p.defs[scope]; def != nil && def.Parent != "" {
				return def.Parent, true
			}
		}
		return "", false
	}
	if targets := p.rebind(caller, name); len(targets) > 0 {
		return targets[0], true
	}
	if scope, ok := p.typedScope(caller, name); ok {
		return fileindex.Join(scope, name), true
	}
	for _, imp := range p.ix.Imports {
		if imp.Alias == name && imp.Namespace != "" && imp.Kind != fileindex.ImportTypeOnly {
			return imp.Namespace, true
		}
	}
	return "", false
}

// inExport reports whether the node is part of an export statement or a
// CommonJS export assignment.
func (p *projection) inExport(node *sitter.Node) bool {
	return slices.ContainsFunc(p.exportStatements, func(statement *sitter.Node) bool {
		return node.StartByte() >= statement.StartByte() && node.EndByte() <= statement.EndByte()
	})
}
