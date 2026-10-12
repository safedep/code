// Package ecmascript answers the questions of a detection about one
// JavaScript or TypeScript file: what a name stands for, what a call calls,
// and what an assignment writes to.
//
// The analysis reads one file and does not track scopes. A name stands for
// each value that the file binds to it anywhere. A name also stands for the
// global of that name, unless the file declares it at its top level. A
// declaration in a function hides the global only in that function, so a
// detection that asks for require sees require in a file that declares a
// local named require in a function.
//
// The analysis reads hostile input. Its walk is linear, it has a node
// limit, and it stops when its context is done. After the walk it solves
// the names to a fixed point, so a result does not depend on the order of
// the calls of a detection, and a cycle of names ends.
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
	// reads as "", so it stands for nothing. A copy of a long node for each
	// call makes a chain such as x()()() take minutes.
	maxName = 256

	// maxValues is the number of values that the analysis keeps for a name
	// or an expression. The fixed point ends because a set only grows and
	// has this limit.
	maxValues = 16

	// maxBranch is the depth of nested branches that the analysis reads, as
	// a || (b || c). A chain of members or calls has no depth limit.
	maxBranch = 32

	// maxRounds is the number of rounds that bind the members of the global
	// object. A round can find a new alias of the global object.
	maxRounds = 4

	// minWork is the least number of source reads that the fixed point can
	// do. The budget is the larger of minWork and workPerSource for each
	// source. A name with many sources that grow one at a time, as in a
	// chain of aliases in reverse order, can need a read of all its
	// sources for each change, and the budget stops it.
	minWork       = 1_000_000
	workPerSource = 32

	// ctxCheckInterval is the number of source reads between two checks of
	// the context.
	ctxCheckInterval = 4096
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

// IsGlobal reports the global of one of the given names itself.
func (v Value) IsGlobal(names ...string) bool {
	return v.Module == "" && v.Member == "" && slices.Contains(names, v.Global)
}

// IsGlobalObject reports the global object, as globalThis.
func (v Value) IsGlobalObject() bool {
	return v.Module == "" && v.Member == "" && globalObjects[v.Global]
}

// IsModuleMember reports the given member of one of the given modules.
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

	// Args are the arguments, with no comments. A tagged template, as
	// f`x`, has the template as its one argument. In f.call(t, x) the
	// first argument is t.
	Args []*sitter.Node
}

// Arg returns argument i, or nil.
func (c Call) Arg(i int) *sitter.Node {
	if i < 0 || i >= len(c.Args) {
		return nil
	}
	return c.Args[i]
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

	// Global reports an object that stands for the global object, also
	// through an alias, as g in const g = globalThis; g.r = require.
	Global bool

	// Right is the value.
	Right *sitter.Node
}

// Binding puts a value on a name.
type Binding struct {
	// Name is the bound name.
	Name string

	// Node is the declaration, the assignment, the parameter or the import.
	Node *sitter.Node

	// Value is the expression on the right side, or nil for an import.
	Value *sitter.Node

	// Member is the member of Value that the name takes, as exec in
	// const { exec: run } = cp. It is "" when the name takes all of Value.
	Member string
}

// Options bound an analysis.
type Options struct {
	// MaxNodes is the node limit of the walk, as ts.WalkOptions sets it.
	MaxNodes int
}

// source is one value of a name: an expression, a member of an
// expression, or a value that an import gives.
type source struct {
	node   *sitter.Node
	member string
	value  *Value
}

// Analysis is the analysis of one parse tree.
type Analysis struct {
	src         []byte
	typescript  bool
	sources     map[string][]source
	names       []string
	sourceCount int
	work        int
	calls       []Call
	assignments []Assignment
	bindings    []Binding
	truncated   bool

	// topLevel holds the names that a declaration or an import at the top
	// level of the file binds. Such a name does not stand for its global.
	topLevel map[string]bool
	// scopes holds the functions and the blocks that the walk is in, and
	// functions counts the functions among them.
	scopes    []scope
	functions int

	resolved   map[string][]Value
	dependents map[string]map[string]bool

	// evaluated keeps the values of each node that Eval reads. The values
	// of names are final only after Analyze, so it is empty before.
	final     bool
	evaluated map[*sitter.Node][]Value
}

// Analyze reads a JavaScript or TypeScript parse tree. At the node limit it
// returns the analysis of the nodes before the limit, and Truncated reports
// it. It returns the error of ctx when ctx is done.
func Analyze(ctx context.Context, tree core.ParseTree, opts Options) (*Analysis, error) {
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

	a := &Analysis{
		src:        *data,
		typescript: code == core.LanguageCodeTypescript,
		sources:    map[string][]source{},
		topLevel:   map[string]bool{},
		resolved:   map[string][]Value{},
		dependents: map[string]map[string]bool{},
		evaluated:  map[*sitter.Node][]Value{},
	}
	err = ts.Walk(ctx, tree.Tree().RootNode(), ts.WalkOptions{MaxNodes: opts.MaxNodes}, func(n *sitter.Node) bool {
		a.visit(n)
		return true
	})
	switch {
	case errors.Is(err, ts.ErrNodeLimit):
		a.truncated = true
	case err != nil:
		return nil, err
	}

	if err := a.solve(ctx, a.names); err != nil {
		return nil, err
	}
	bound := make([]bool, len(a.assignments))
	for round := 0; ; round++ {
		added := a.bindGlobalMembers(bound)
		if len(added) == 0 {
			break
		}
		if round == maxRounds {
			// A longer chain of aliases of the global object can bind more.
			a.truncated = true
			break
		}
		if err := a.solve(ctx, added); err != nil {
			return nil, err
		}
	}
	for i := range a.assignments {
		a.assignments[i].Global = slices.ContainsFunc(a.value(a.assignments[i].Object, nil, 0), Value.IsGlobalObject)
	}
	a.final = true
	return a, nil
}

// scope is a function or a block that the walk is in.
type scope struct {
	end      uint32
	function bool
}

// functionTypes are the nodes that open a function scope. var declares a
// name for the whole function.
var functionTypes = map[string]bool{
	"function_declaration": true, "function_expression": true, "function": true, "arrow_function": true,
	"method_definition": true, "generator_function_declaration": true, "generator_function": true,
}

// blockTypes are the nodes that open a block scope. let and const declare a
// name for the block only.
var blockTypes = map[string]bool{
	"statement_block": true, "class_body": true, "switch_body": true, "for_statement": true,
	"for_in_statement": true, "catch_clause": true,
}

// enter keeps the scopes of the walk. The walk visits a parent before its
// children, so the scopes that end before n are behind it.
func (a *Analysis) enter(n *sitter.Node) {
	for len(a.scopes) > 0 && n.StartByte() >= a.scopes[len(a.scopes)-1].end {
		if a.scopes[len(a.scopes)-1].function {
			a.functions--
		}
		a.scopes = a.scopes[:len(a.scopes)-1]
	}
	switch t := n.Type(); {
	case functionTypes[t]:
		a.scopes = append(a.scopes, scope{end: n.EndByte(), function: true})
		a.functions++
	case blockTypes[t]:
		a.scopes = append(a.scopes, scope{end: n.EndByte()})
	}
}

func (a *Analysis) visit(n *sitter.Node) {
	a.enter(n)
	switch n.Type() {
	case "import_statement":
		a.bindImport(n)
	case "lexical_declaration", "variable_declaration":
		// let and const at the top level are outside each block, and var is
		// outside each function.
		top := len(a.scopes) == 0 || (n.Type() == "variable_declaration" && a.functions == 0)
		if top {
			for _, d := range ts.NamedChildren(n) {
				if d.Type() == "variable_declarator" {
					a.declareTopLevel(d.ChildByFieldName("name"))
				}
			}
		}
	case "variable_declarator":
		a.bindPattern(n, n.ChildByFieldName("name"), n.ChildByFieldName("value"))
	case "assignment_pattern":
		// A default parameter, as r in (r = require) => r(x).
		if left := n.ChildByFieldName("left"); left != nil && left.Type() == "identifier" {
			a.bind(a.Name(left), n, n.ChildByFieldName("right"), "")
		}
	case "augmented_assignment_expression":
		// e ||= eval, e ??= eval and e &&= eval can put the value on e.
		left := n.ChildByFieldName("left")
		if op := n.ChildByFieldName("operator"); left != nil && left.Type() == "identifier" && op != nil && logicalAssign[op.Type()] {
			a.bind(a.Name(left), n, n.ChildByFieldName("right"), "")
		}
	case "assignment_expression":
		left, right := a.Unparen(n.ChildByFieldName("left")), n.ChildByFieldName("right")
		if left == nil || right == nil {
			return
		}
		switch left.Type() {
		case "identifier":
			a.bind(a.Name(left), n, right, "")
		case "object_pattern":
			a.bindPattern(n, left, right)
		case "member_expression", "subscript_expression":
			as := Assignment{Node: n, Object: left.ChildByFieldName("object"), Right: right}
			as.Property, as.Computed = a.memberKey(left)
			a.assignments = append(a.assignments, as)
		}
	case "call_expression":
		a.calls = append(a.calls, a.call(n, n.ChildByFieldName("function"), false))
	case "new_expression":
		a.calls = append(a.calls, a.call(n, n.ChildByFieldName("constructor"), true))
	}
}

var logicalAssign = map[string]bool{"||=": true, "&&=": true, "??=": true}

func (a *Analysis) call(n, callee *sitter.Node, isNew bool) Call {
	callee = a.Unparen(callee)
	return Call{Node: n, Callee: callee, Name: a.calleeName(callee), New: isNew, Args: Args(n)}
}

// bindImport binds the names of an import statement, as run in
// import { exec as run } from 'child_process'. A default import and a
// namespace import stand for the module.
func (a *Analysis) bindImport(n *sitter.Node) {
	bindModule := func(src, alias *sitter.Node, member string) {
		if module, ok := a.StringValue(src); ok {
			a.bindValue(a.Name(alias), n, Value{Module: module, Member: member})
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
						member := a.propertyName(name)
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

func (a *Analysis) bind(name string, node, value *sitter.Node, member string) {
	if name == "" || value == nil {
		return
	}
	a.addSource(name, source{node: value, member: member})
	a.bindings = append(a.bindings, Binding{Name: name, Node: node, Value: value, Member: member})
}

// declareTopLevel records the names that a pattern at the top level of the
// file declares.
func (a *Analysis) declareTopLevel(pattern *sitter.Node) {
	if pattern == nil {
		return
	}
	switch pattern.Type() {
	case "identifier":
		a.topLevel[a.Name(pattern)] = true
	case "object_pattern":
		for _, p := range ts.NamedChildren(pattern) {
			switch p.Type() {
			case "shorthand_property_identifier_pattern":
				a.topLevel[a.Name(p)] = true
			case "object_assignment_pattern":
				a.topLevel[a.Name(p.ChildByFieldName("left"))] = true
			case "pair_pattern":
				v := p.ChildByFieldName("value")
				if v != nil && v.Type() == "assignment_pattern" {
					v = v.ChildByFieldName("left")
				}
				if v != nil && v.Type() == "identifier" {
					a.topLevel[a.Name(v)] = true
				}
			}
		}
	}
}

func (a *Analysis) bindValue(name string, node *sitter.Node, v Value) {
	if name == "" {
		return
	}
	a.topLevel[name] = true
	a.addSource(name, source{value: &v})
	a.bindings = append(a.bindings, Binding{Name: name, Node: node, Member: v.Member})
}

func (a *Analysis) addSource(name string, s source) {
	if _, ok := a.sources[name]; !ok {
		a.names = append(a.names, name)
	}
	a.sources[name] = append(a.sources[name], s)
	a.sourceCount++
}

// bindPattern binds the name of a declaration, and the names of an object
// pattern to the members of the value, as run in { exec: run } = cp.
func (a *Analysis) bindPattern(n, pattern, value *sitter.Node) {
	if pattern == nil || value == nil {
		return
	}
	if pattern.Type() == "identifier" {
		a.bind(a.Name(pattern), n, value, "")
		return
	}
	if pattern.Type() != "object_pattern" {
		return
	}
	for _, p := range ts.NamedChildren(pattern) {
		var key, name string
		switch p.Type() {
		case "shorthand_property_identifier_pattern":
			key = a.Name(p)
			name = key
		case "object_assignment_pattern":
			left := p.ChildByFieldName("left")
			if left == nil || left.Type() != "shorthand_property_identifier_pattern" {
				continue
			}
			key = a.Name(left)
			name = key
			// The default is a value too, as eval in { run = eval } = cfg.
			a.bind(name, n, p.ChildByFieldName("right"), "")
		case "pair_pattern":
			k, v := p.ChildByFieldName("key"), p.ChildByFieldName("value")
			if v != nil && v.Type() == "assignment_pattern" {
				v = v.ChildByFieldName("left")
			}
			if v == nil || v.Type() != "identifier" {
				continue
			}
			key = a.propertyName(k)
			name = a.Name(v)
		}
		if key != "" {
			a.bind(name, n, value, key)
		}
	}
}

// solve computes the values of the names, and of each name that reads
// them, to a fixed point. A set of values only grows and has a limit, so a
// name changes a bounded number of times. At the work budget it stops and
// marks the analysis as truncated.
func (a *Analysis) solve(ctx context.Context, names []string) error {
	budget := max(minWork, workPerSource*a.sourceCount)
	queued := map[string]bool{}
	var queue []string
	for _, name := range names {
		if !queued[name] {
			queued[name] = true
			queue = append(queue, name)
		}
	}
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		delete(queued, name)

		read := func(dep string) {
			if dep == name {
				return
			}
			if a.dependents[dep] == nil {
				a.dependents[dep] = map[string]bool{}
			}
			a.dependents[dep][name] = true
		}
		values := a.resolved[name]
		before := len(values)
		if before >= maxValues {
			continue
		}
		for _, s := range a.sources[name] {
			a.work++
			if a.work > budget {
				a.truncated = true
				return nil
			}
			if a.work%ctxCheckInterval == 0 {
				if err := ctx.Err(); err != nil {
					return err
				}
			}
			values = appendValues(values, a.sourceValues(s, read)...)
		}
		if len(values) == before {
			continue
		}
		a.resolved[name] = values
		for dep := range a.dependents[name] {
			if !queued[dep] {
				queued[dep] = true
				queue = append(queue, dep)
			}
		}
	}
	return nil
}

func (a *Analysis) sourceValues(s source, read func(string)) []Value {
	switch {
	case s.value != nil:
		return []Value{*s.value}
	case s.member != "":
		return a.member(a.value(s.node, read, 0), s.member, read)
	default:
		return a.value(s.node, read, 0)
	}
}

// bindGlobalMembers binds the property of each assignment to a member of
// the global object, as r in global.r = require. The object can be an alias
// of the global object, so the binding waits for the names to be solved. It
// returns the names that it bound.
func (a *Analysis) bindGlobalMembers(bound []bool) []string {
	var added []string
	for i, as := range a.assignments {
		if bound[i] || as.Computed || as.Property == "" {
			continue
		}
		if !slices.ContainsFunc(a.value(as.Object, nil, 0), Value.IsGlobalObject) {
			continue
		}
		bound[i] = true
		a.bind(as.Property, as.Node, as.Right, "")
		added = append(added, as.Property)
	}
	return added
}

// Truncated reports an analysis that stopped at the node limit, at the work
// budget of the names, or at the round limit of the aliases of the global
// object. The values of some names can then be missing.
func (a *Analysis) Truncated() bool { return a.truncated }

// Calls returns the calls and the new expressions in source order, an outer
// call before the calls inside it.
func (a *Analysis) Calls() []Call { return a.calls }

// Assignments returns the assignments to a member of an object in source
// order.
func (a *Analysis) Assignments() []Assignment { return a.assignments }

// Bindings returns the bindings of names in source order, then the
// assignments to members of the global object.
func (a *Analysis) Bindings() []Binding { return a.bindings }

// Resolve returns the values that a name stands for: the global of that
// name, unless the file declares it at its top level, then each value that
// the file binds to it. A name that the file binds only to values that the
// analysis cannot read, as the result of a call, can give no value.
func (a *Analysis) Resolve(name string) []Value { return a.lookup(name, nil) }

func (a *Analysis) lookup(name string, read func(string)) []Value {
	if a.topLevel[name] {
		return a.read(name, read)
	}
	return a.global(name, read)
}

// global returns the global of a name and each value that the file binds
// to it. A member of the global object reads it, because globalThis.eval
// is eval also in a file that declares a local eval.
func (a *Analysis) global(name string, read func(string)) []Value {
	if name == "" || len(name) > maxName {
		return nil
	}
	return appendValues([]Value{{Global: name}}, a.read(name, read)...)
}

func (a *Analysis) read(name string, read func(string)) []Value {
	if name == "" || len(name) > maxName {
		return nil
	}
	if read != nil {
		read(name)
	}
	return a.resolved[name]
}

// Eval returns the values that an expression stands for: a name, a member
// of a value, a require call or an import of a module, as
// require('child_process').exec.
func (a *Analysis) Eval(n *sitter.Node) []Value { return a.value(n, nil, 0) }

// value reads the chain of an expression, as the calls of x()()() or the
// members of a.b.c, from its end to its start with no recursion. After
// Analyze it keeps the values of each node of the chain, so a detection
// that asks for each call of a chain reads each node once.
func (a *Analysis) value(n *sitter.Node, read func(string), depth int) []Value {
	var chain []*sitter.Node
	var values []Value
	for n = a.Unparen(n); n != nil; n = a.inner(n) {
		if v, ok := a.evaluated[n]; ok {
			values = v
			break
		}
		chain = append(chain, n)
	}
	for i := len(chain) - 1; i >= 0; i-- {
		values = a.step(chain[i], values, read, depth)
		if a.final {
			a.evaluated[chain[i]] = values
		}
	}
	return values
}

// inner returns the expression that the value of n builds on: the object
// of a member, the function of a call, the operand of an await. It returns
// nil for the start of a chain.
func (a *Analysis) inner(n *sitter.Node) *sitter.Node {
	switch n.Type() {
	case "member_expression", "subscript_expression":
		return a.Unparen(n.ChildByFieldName("object"))
	case "await_expression":
		return a.Unparen(n.NamedChild(0))
	case "call_expression":
		if fn := a.Unparen(n.ChildByFieldName("function")); fn != nil && fn.Type() != "import" {
			return fn
		}
	}
	return nil
}

// step returns the values of n from the values of its inner expression.
func (a *Analysis) step(n *sitter.Node, inner []Value, read func(string), depth int) []Value {
	switch n.Type() {
	case "identifier":
		return a.lookup(a.Name(n), read)
	case "member_expression", "subscript_expression":
		key, computed := a.memberKey(n)
		if computed {
			return nil
		}
		return a.member(inner, key, read)
	case "await_expression":
		return inner
	case "call_expression":
		return a.callValues(n, inner)
	case "binary_expression":
		if op := n.ChildByFieldName("operator"); op != nil && logical[op.Type()] {
			return a.either(n.ChildByFieldName("left"), n.ChildByFieldName("right"), read, depth)
		}
	case "ternary_expression":
		return a.either(n.ChildByFieldName("consequence"), n.ChildByFieldName("alternative"), read, depth)
	}
	return nil
}

var logical = map[string]bool{"||": true, "&&": true, "??": true}

// either returns the values of both branches, as eval and require in
// c ? eval : require.
func (a *Analysis) either(x, y *sitter.Node, read func(string), depth int) []Value {
	if depth >= maxBranch {
		return nil
	}
	return appendValues(a.value(x, read, depth+1), a.value(y, read, depth+1)...)
}

// callValues returns the values of a call: the module of require('m') or of
// import('m') with a plain string, and require for createRequire(url).
func (a *Analysis) callValues(n *sitter.Node, inner []Value) []Value {
	module := func() (string, bool) {
		args := expressions(n.ChildByFieldName("arguments"), 1)
		if len(args) == 0 {
			return "", false
		}
		return a.StringValue(args[0])
	}
	if fn := a.Unparen(n.ChildByFieldName("function")); fn != nil && fn.Type() == "import" {
		if m, ok := module(); ok {
			return []Value{{Module: m}}
		}
		return nil
	}
	if fn := a.Unparen(n.ChildByFieldName("function")); fn != nil && fn.Type() == "member_expression" && a.Name(fn.ChildByFieldName("property")) == "bind" {
		// f.bind(t) returns f with a fixed this.
		return inner
	}
	var out []Value
	for _, v := range inner {
		switch {
		case v.IsGlobal("require"), v == moduleRequire:
			if m, ok := module(); ok {
				out = appendValues(out, Value{Module: m})
			}
		case v.IsModuleMember("createRequire", "module", "node:module"):
			out = appendValues(out, Value{Global: "require"})
		}
	}
	return out
}

// moduleRequire is module.require, which loads a module as require does.
var moduleRequire = Value{Global: "module", Member: "require"}

// functionMethods run or bind the function that they are a member of, so
// eval.call(null, x) runs eval.
var functionMethods = map[string]bool{"call": true, "apply": true, "bind": true}

// member returns the values of a member of each value: a member of a
// module, a global name for a member of the global object, or a member of
// a global.
func (a *Analysis) member(values []Value, key string, read func(string)) []Value {
	if key == "" {
		return nil
	}
	var out []Value
	for _, v := range values {
		switch {
		case functionMethods[key]:
			out = appendValues(out, v)
		case v.Module != "" && v.Member == "" && key == "default":
			out = appendValues(out, v)
		case v.Module != "" && v.Member == "":
			out = appendValues(out, Value{Module: v.Module, Member: key})
		case v.Member != "" || v.Module != "":
		case globalObjects[v.Global]:
			out = appendValues(out, a.global(key, read)...)
		case v.Global == "process" && key == "mainModule", v.Global == "require" && key == "main":
			// process.mainModule.require and require.main.require are
			// module.require.
			out = appendValues(out, Value{Global: "module"})
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

// memberKey returns the key of a member or a subscript expression, and
// reports a key that is not a plain string.
func (a *Analysis) memberKey(n *sitter.Node) (string, bool) {
	if n.Type() == "member_expression" {
		return a.Name(n.ChildByFieldName("property")), false
	}
	key, ok := a.StringValue(n.ChildByFieldName("index"))
	return key, !ok
}

// calleeName returns the name that a callee reads, through members,
// subscripts, parentheses and comma expressions.
func (a *Analysis) calleeName(n *sitter.Node) string {
	for n != nil {
		switch n.Type() {
		case "identifier", "property_identifier", "import":
			return a.Name(n)
		case "member_expression":
			n = n.ChildByFieldName("property")
		case "subscript_expression":
			s, _ := a.StringValue(n.ChildByFieldName("index"))
			return s
		default:
			next := a.Unparen(n)
			if next == n {
				return ""
			}
			n = next
		}
	}
	return ""
}

func (a *Analysis) propertyName(n *sitter.Node) string {
	if n == nil {
		return ""
	}
	switch n.Type() {
	case "property_identifier", "identifier":
		return a.Name(n)
	case "string":
		s, _ := a.StringValue(n)
		return s
	}
	return ""
}

// Name returns the text of a short node with its escapes decoded, or "" for
// a nil node or a node over the name limit.
func (a *Analysis) Name(n *sitter.Node) string {
	if n == nil || n.EndByte()-n.StartByte() > maxName {
		return ""
	}
	return unescape(n.Content(a.src))
}

// Unparen returns the expression inside parentheses, comma expressions,
// assignments and TypeScript assertions, as eval in (0, eval), in
// (e = eval) and in (require as any). Each step goes to a child, so the
// loop ends.
func (a *Analysis) Unparen(n *sitter.Node) *sitter.Node {
	for n != nil {
		switch n.Type() {
		case "parenthesized_expression", "sequence_expression":
			n = lastExpression(n)
		case "assignment_expression":
			n = n.ChildByFieldName("right")
		case "as_expression", "satisfies_expression", "non_null_expression":
			if !a.typescript {
				return n
			}
			n = n.NamedChild(0)
		case "type_assertion":
			if !a.typescript {
				return n
			}
			n = lastExpression(n)
		default:
			return n
		}
	}
	return nil
}

// Args returns the arguments of a call or a new expression, with no
// comments. A tagged template gives the template.
func Args(call *sitter.Node) []*sitter.Node {
	if call == nil {
		return nil
	}
	args := call.ChildByFieldName("arguments")
	if args == nil {
		return nil
	}
	if args.Type() != "arguments" {
		return []*sitter.Node{args}
	}
	return expressions(args, -1)
}

// expressions returns up to limit named children of n that are not
// comments, or all of them for a negative limit.
func expressions(n *sitter.Node, limit int) []*sitter.Node {
	if n == nil {
		return nil
	}
	var out []*sitter.Node
	c := sitter.NewTreeCursor(n)
	defer c.Close()
	for ok := c.GoToFirstChild(); ok && len(out) != limit; ok = c.GoToNextSibling() {
		if child := c.CurrentNode(); child.IsNamed() && child.Type() != "comment" {
			out = append(out, child)
		}
	}
	return out
}

// lastExpression returns the last named child that is not a comment, as
// eval in (0, eval /* x */).
func lastExpression(n *sitter.Node) *sitter.Node {
	all := expressions(n, -1)
	if len(all) == 0 {
		return nil
	}
	return all[len(all)-1]
}
