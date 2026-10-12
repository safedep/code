// Package ecmascript answers the questions of a detection about one
// JavaScript or TypeScript file: what a name stands for, what a call calls,
// and what an assignment writes to.
//
// The analysis reads one file and does not track scopes. A name stands for
// each value that the file binds to it anywhere, so a detection sees a
// binding in one function from a call in another. A name with no binding
// stands for the global of that name.
//
// The analysis reads hostile input. Its walk is linear, it has a node
// limit, it stops when its context is done, and it bounds each chain of
// names and of members.
package ecmascript

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/safedep/code/core"
	"github.com/safedep/code/pkg/ts"
	sitter "github.com/smacker/go-tree-sitter"
)

const (
	// maxName is the longest name that the analysis reads. A longer name
	// reads as "", so it resolves to nothing. A copy of a long node for each
	// call makes a chain such as x()()() take minutes.
	maxName = 256

	// maxChain is the depth of a chain of names, as a = b, b = c.
	maxChain = 32

	// maxValues is the number of values that the analysis keeps for a name.
	maxValues = 16
)

// globalObjects are the names of the global object. A member of one is a
// global name, as eval in globalThis.eval.
var globalObjects = map[string]bool{"global": true, "globalThis": true, "window": true, "self": true}

// Value is what a name or an expression stands for. Global or Module is
// set. Member names a member of the global or of the module.
type Value struct {
	// Global is a global name, as require, eval or process.
	Global string

	// Module is the module of an import or of a require call, as written in
	// the source, as child_process or node:child_process.
	Module string

	// Member is a member of the global or of the module, as exec.
	Member string
}

// IsGlobal reports the global of the given name itself.
func (v Value) IsGlobal(name string) bool {
	return v.Module == "" && v.Member == "" && v.Global == name
}

// IsGlobalObject reports the global object, as globalThis.
func (v Value) IsGlobalObject() bool { return v.Member == "" && globalObjects[v.Global] }

// IsModule reports one of the given modules itself.
func (v Value) IsModule(modules ...string) bool {
	return v.Module != "" && v.Member == "" && slices.Contains(modules, v.Module)
}

// IsModuleMember reports a member of one of the given modules.
func (v Value) IsModuleMember(member string, modules ...string) bool {
	return v.Module != "" && v.Member == member && slices.Contains(modules, v.Module)
}

// Call is a call or a new expression.
type Call struct {
	// Node is the call_expression or the new_expression.
	Node *sitter.Node

	// Callee is the function of a call or the constructor of a new
	// expression, with no parentheses or type assertions around it.
	Callee *sitter.Node

	// Name is the name that the callee reads: eval in eval(x), (0, eval)(x)
	// and globalThis['eval'](x), and the property of a member call. It is
	// "" when the callee has no name, as in f()().
	Name string

	// New reports a new expression.
	New bool

	// Args are the arguments, with no comments.
	Args []*sitter.Node
}

// Assignment writes to a member of an object, as global.r = require.
type Assignment struct {
	// Node is the assignment_expression.
	Node *sitter.Node

	// Object is the object of the member that the assignment writes.
	Object *sitter.Node

	// Property is the member name. It is "" when Computed is true.
	Property string

	// Computed reports a member key that is not a plain string, as k in
	// global[k] = x.
	Computed bool

	// Right is the value.
	Right *sitter.Node
}

// Binding puts a value on a name.
type Binding struct {
	// Name is the bound name.
	Name string

	// Node is the declaration, the assignment or the import.
	Node *sitter.Node

	// Value is the expression of the value, or nil for an import.
	Value *sitter.Node
}

// Options bound an analysis.
type Options struct {
	// MaxNodes is the node limit of the walk. Zero selects
	// ts.DefaultMaxNodes. A negative value removes the limit.
	MaxNodes int
}

// source is one value of a name: an expression, a member of an
// expression, or a value that an import gives.
type source struct {
	node   *sitter.Node
	member string
	value  *Value
}

// File is the analysis of one parse tree.
type File struct {
	src         []byte
	typescript  bool
	sources     map[string][]source
	calls       []Call
	assignments []Assignment
	bindings    []Binding
	truncated   bool

	resolved  map[string][]Value
	evaluated map[*sitter.Node][]Value
	stack     map[string]bool
}

// Analyze reads a JavaScript or TypeScript parse tree. At the node limit it
// returns the analysis of the nodes before the limit, and Truncated reports
// it. It returns the error of ctx when ctx is done.
func Analyze(ctx context.Context, tree core.ParseTree, opts Options) (*File, error) {
	language, err := tree.Language()
	if err != nil {
		return nil, fmt.Errorf("failed to get language from parse tree: %w", err)
	}
	code := language.Meta().Code
	if code != core.LanguageCodeJavascript && code != core.LanguageCodeTypescript {
		return nil, fmt.Errorf("ecmascript analysis does not read %s", code)
	}
	data, err := tree.Data()
	if err != nil {
		return nil, fmt.Errorf("failed to get data from parse tree: %w", err)
	}

	f := &File{
		src:        *data,
		typescript: code == core.LanguageCodeTypescript,
		sources:    map[string][]source{},
		resolved:   map[string][]Value{},
		evaluated:  map[*sitter.Node][]Value{},
		stack:      map[string]bool{},
	}
	err = ts.Walk(ctx, tree.Tree().RootNode(), ts.WalkOptions{MaxNodes: opts.MaxNodes}, func(n *sitter.Node) bool {
		f.visit(n)
		return true
	})
	switch {
	case errors.Is(err, ts.ErrNodeLimit):
		f.truncated = true
	case err != nil:
		return nil, err
	}

	f.bindGlobalMembers()
	return f, nil
}

// bindImport binds the names of an import statement, as run in
// import { exec as run } from 'child_process'. A default import and a
// namespace import stand for the module.
func (f *File) bindImport(n *sitter.Node) {
	bindModule := func(src, alias *sitter.Node, member string) {
		if module, ok := f.StringValue(src); ok {
			f.bind(f.Name(alias), n, nil, moduleSource(module, member))
		}
	}
	from := n.ChildByFieldName("source")
	for _, clause := range ts.NamedChildren(n) {
		switch clause.Type() {
		case "import_require_clause":
			bindModule(clause.ChildByFieldName("source"), clause.NamedChild(0), "")
		case "import_clause":
			for _, c := range ts.NamedChildren(clause) {
				switch c.Type() {
				case "identifier":
					bindModule(from, c, "")
				case "namespace_import":
					bindModule(from, c.NamedChild(0), "")
				case "named_imports":
					for _, spec := range ts.NamedChildren(c) {
						name, alias := spec.ChildByFieldName("name"), spec.ChildByFieldName("alias")
						if alias == nil {
							alias = name
						}
						member := f.propertyName(name)
						if member == "default" {
							member = ""
						}
						bindModule(from, alias, member)
					}
				}
			}
		}
	}
}

func moduleSource(module, member string) source {
	return source{value: &Value{Module: module, Member: member}}
}

func (f *File) bind(name string, node, value *sitter.Node, s source) {
	if name == "" {
		return
	}
	f.sources[name] = append(f.sources[name], s)
	f.bindings = append(f.bindings, Binding{Name: name, Node: node, Value: value})
}

func (f *File) visit(n *sitter.Node) {
	switch n.Type() {
	case "import_statement":
		f.bindImport(n)
	case "variable_declarator":
		f.bindPattern(n, n.ChildByFieldName("name"), n.ChildByFieldName("value"))
	case "assignment_expression":
		left, right := f.Unparen(n.ChildByFieldName("left")), n.ChildByFieldName("right")
		if left == nil || right == nil {
			return
		}
		switch left.Type() {
		case "identifier":
			f.bind(f.Name(left), n, right, source{node: right})
		case "object_pattern":
			f.bindPattern(n, left, right)
		case "member_expression", "subscript_expression":
			a := Assignment{Node: n, Object: left.ChildByFieldName("object"), Right: right}
			a.Property, a.Computed = f.memberKey(left)
			f.assignments = append(f.assignments, a)
		}
	case "call_expression":
		f.calls = append(f.calls, f.call(n, n.ChildByFieldName("function"), false))
	case "new_expression":
		f.calls = append(f.calls, f.call(n, n.ChildByFieldName("constructor"), true))
	}
}

func (f *File) call(n, callee *sitter.Node, isNew bool) Call {
	callee = f.Unparen(callee)
	c := Call{Node: n, Callee: callee, Name: f.calleeName(callee), New: isNew}
	args := n.ChildByFieldName("arguments")
	if args == nil {
		return c
	}
	if args.Type() != "arguments" {
		c.Args = []*sitter.Node{args}
		return c
	}
	for _, a := range ts.NamedChildren(args) {
		if a.Type() != "comment" {
			c.Args = append(c.Args, a)
		}
	}
	return c
}

// bindPattern binds the name of a declaration, and the names of an object
// pattern to the members of the value, as run in { exec: run } = cp.
func (f *File) bindPattern(n, pattern, value *sitter.Node) {
	if pattern == nil || value == nil {
		return
	}
	switch pattern.Type() {
	case "identifier":
		f.bind(f.Name(pattern), n, value, source{node: value})
		return
	case "object_pattern":
	default:
		return
	}
	for _, p := range ts.NamedChildren(pattern) {
		var key, name string
		switch p.Type() {
		case "shorthand_property_identifier_pattern":
			key = f.Name(p)
			name = key
		case "object_assignment_pattern":
			left := p.ChildByFieldName("left")
			if left == nil || left.Type() != "shorthand_property_identifier_pattern" {
				continue
			}
			key = f.Name(left)
			name = key
		case "pair_pattern":
			k, v := p.ChildByFieldName("key"), p.ChildByFieldName("value")
			if v != nil && v.Type() == "assignment_pattern" {
				v = v.ChildByFieldName("left")
			}
			if v == nil || v.Type() != "identifier" {
				continue
			}
			key = f.propertyName(k)
			name = f.Name(v)
		}
		if key != "" && name != "" {
			f.bind(name, n, value, source{node: value, member: key})
		}
	}
}

// bindGlobalMembers binds the property of each assignment to a member of
// the global object, as r in global.r = require. The object of the
// assignment can be an alias of the global object, so the binding waits
// for the walk to end.
func (f *File) bindGlobalMembers() {
	for _, a := range f.assignments {
		if a.Computed || a.Property == "" || !anyValue(f.Eval(a.Object), Value.IsGlobalObject) {
			continue
		}
		f.bind(a.Property, a.Node, a.Right, source{node: a.Right})
	}
	clear(f.resolved)
	clear(f.evaluated)
}

// Truncated reports an analysis that stopped at the node limit.
func (f *File) Truncated() bool { return f.truncated }

// Calls returns the calls and the new expressions in source order.
func (f *File) Calls() []Call { return f.calls }

// Assignments returns the assignments to a member of an object in source
// order.
func (f *File) Assignments() []Assignment { return f.assignments }

// Bindings returns the bindings of names: the imports, the declarations
// and the assignments in source order, then the assignments to members of
// the global object.
func (f *File) Bindings() []Binding { return f.bindings }

// Resolve returns the values that a name stands for. A name with no
// binding stands for the global of that name. A name that the file binds
// only to values that the analysis cannot read, such as the result of a
// call, gives no value. A cycle of names, or a chain that is longer than
// the limit, gives a part of the values.
func (f *File) Resolve(name string) []Value {
	if name == "" || len(name) > maxName {
		return nil
	}
	if v, ok := f.resolved[name]; ok {
		return v
	}
	sources, bound := f.sources[name]
	if !bound {
		return []Value{{Global: name}}
	}
	if f.stack[name] || len(f.stack) >= maxChain {
		return nil
	}
	f.stack[name] = true
	var out []Value
	for _, s := range sources {
		switch {
		case s.value != nil:
			out = appendValues(out, *s.value)
		case s.member != "":
			out = appendValues(out, f.member(f.Eval(s.node), s.member)...)
		default:
			out = appendValues(out, f.Eval(s.node)...)
		}
	}
	delete(f.stack, name)
	f.resolved[name] = out
	return out
}

// Eval returns the values that an expression stands for: a name, a member
// of a value, a require call or an import of a module, as
// require('child_process').exec.
//
// Eval reads the chain of an expression, as the calls of x()()() or the
// members of a.b.c, from its end to its start with no recursion. It keeps
// the values of each node of the chain, so a detection that asks for each
// call of a chain reads each node once.
func (f *File) Eval(n *sitter.Node) []Value {
	var chain []*sitter.Node
	var values []Value
	for n = f.Unparen(n); n != nil; {
		if v, ok := f.evaluated[n]; ok {
			values = v
			break
		}
		chain = append(chain, n)
		n = f.inner(n)
	}
	for i := len(chain) - 1; i >= 0; i-- {
		values = f.step(chain[i], values)
		f.evaluated[chain[i]] = values
	}
	return values
}

// inner returns the expression that the value of n builds on: the object
// of a member, the function of a call, the operand of an await. It returns
// nil for the start of a chain.
func (f *File) inner(n *sitter.Node) *sitter.Node {
	switch n.Type() {
	case "member_expression", "subscript_expression":
		return f.Unparen(n.ChildByFieldName("object"))
	case "await_expression":
		return f.Unparen(n.NamedChild(0))
	case "call_expression":
		if fn := f.Unparen(n.ChildByFieldName("function")); fn != nil && fn.Type() != "import" {
			return fn
		}
	}
	return nil
}

// step returns the values of n from the values of its inner expression.
func (f *File) step(n *sitter.Node, inner []Value) []Value {
	switch n.Type() {
	case "identifier":
		return f.Resolve(f.Name(n))
	case "member_expression", "subscript_expression":
		key, computed := f.memberKey(n)
		if computed {
			return nil
		}
		return f.member(inner, key)
	case "await_expression":
		return inner
	case "call_expression":
		if fn := f.Unparen(n.ChildByFieldName("function")); fn != nil && fn.Type() == "import" {
			if module, ok := f.StringValue(firstArg(n)); ok {
				return []Value{{Module: module}}
			}
			return nil
		}
		var out []Value
		for _, v := range inner {
			switch {
			case v.IsGlobal("require"):
				if module, ok := f.StringValue(firstArg(n)); ok {
					out = appendValues(out, Value{Module: module})
				}
			case v.IsModuleMember("createRequire", "module", "node:module"):
				out = appendValues(out, Value{Global: "require"})
			}
		}
		return out
	}
	return nil
}

// member returns the values of a member of each value: a member of a
// module, a global name for a member of the global object, or a member of
// a global.
func (f *File) member(values []Value, key string) []Value {
	if key == "" {
		return nil
	}
	var out []Value
	for _, v := range values {
		switch {
		case v.Member != "":
		case v.Module != "":
			out = appendValues(out, Value{Module: v.Module, Member: key})
		case globalObjects[v.Global]:
			out = appendValues(out, f.Resolve(key)...)
		case v.Global != "":
			out = appendValues(out, Value{Global: v.Global, Member: key})
		}
	}
	return out
}

func appendValues(out []Value, values ...Value) []Value {
	for _, v := range values {
		if len(out) >= maxValues {
			return out
		}
		if !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}

func anyValue(values []Value, is func(Value) bool) bool {
	for _, v := range values {
		if is(v) {
			return true
		}
	}
	return false
}

// memberKey returns the key of a member or a subscript expression, and
// reports a key that is not a plain string.
func (f *File) memberKey(n *sitter.Node) (string, bool) {
	if n.Type() == "member_expression" {
		return f.Name(n.ChildByFieldName("property")), false
	}
	key, ok := f.StringValue(n.ChildByFieldName("index"))
	return key, !ok
}

// calleeName returns the name that a callee reads, through members,
// subscripts, parentheses and comma expressions.
func (f *File) calleeName(n *sitter.Node) string {
	for n != nil {
		switch n.Type() {
		case "identifier", "property_identifier", "import":
			return f.Name(n)
		case "member_expression":
			n = n.ChildByFieldName("property")
		case "subscript_expression":
			s, _ := f.StringValue(n.ChildByFieldName("index"))
			return s
		default:
			next := f.Unparen(n)
			if next == n {
				return ""
			}
			n = next
		}
	}
	return ""
}

func (f *File) propertyName(n *sitter.Node) string {
	if n == nil {
		return ""
	}
	switch n.Type() {
	case "property_identifier", "identifier":
		return f.Name(n)
	case "string":
		s, _ := f.StringValue(n)
		return s
	}
	return ""
}

// Name returns the text of a short node with its escapes decoded, as eval
// for eval, or "" for a nil node or a node over the name limit.
func (f *File) Name(n *sitter.Node) string {
	if n == nil || n.EndByte()-n.StartByte() > maxName {
		return ""
	}
	return unescape(n.Content(f.src))
}

// StringValue returns the value of a plain string: a string literal or a
// template with no substitution, through parentheses. Escapes are decoded,
// so 'child\x5fprocess' gives child_process.
func (f *File) StringValue(n *sitter.Node) (string, bool) {
	n = f.Unparen(n)
	if ClassifyArg(n) != ArgString {
		return "", false
	}
	text := n.Content(f.src)
	if len(text) < 2 {
		return "", false
	}
	return unescape(text[1 : len(text)-1]), true
}

// Unparen returns the expression inside parentheses, comma expressions and
// TypeScript assertions, as eval in (0, eval) and in (require as any). Each
// step goes to a child, so the loop ends.
func (f *File) Unparen(n *sitter.Node) *sitter.Node {
	for n != nil {
		switch n.Type() {
		case "parenthesized_expression", "sequence_expression":
			n = lastExpression(n)
		case "as_expression", "satisfies_expression", "non_null_expression":
			if !f.typescript {
				return n
			}
			n = n.NamedChild(0)
		case "type_assertion":
			if !f.typescript {
				return n
			}
			n = lastExpression(n)
		default:
			return n
		}
	}
	return nil
}

// lastExpression returns the last named child that is not a comment, as
// eval in (0, eval /* x */).
func lastExpression(n *sitter.Node) *sitter.Node {
	var last *sitter.Node
	for _, c := range ts.NamedChildren(n) {
		if c.Type() != "comment" {
			last = c
		}
	}
	return last
}

// firstArg returns the first argument of a call. It stops at the first, so
// a call with many arguments costs little.
func firstArg(call *sitter.Node) *sitter.Node {
	args := call.ChildByFieldName("arguments")
	if args == nil || args.Type() != "arguments" {
		return nil
	}
	c := sitter.NewTreeCursor(args)
	defer c.Close()
	for ok := c.GoToFirstChild(); ok; ok = c.GoToNextSibling() {
		if n := c.CurrentNode(); n.IsNamed() && n.Type() != "comment" {
			return n
		}
	}
	return nil
}
