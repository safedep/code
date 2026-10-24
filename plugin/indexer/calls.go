package indexer

import (
	"regexp"
	"slices"
	"strings"

	"github.com/safedep/code/core/ast"
	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/pkg/ts"
	"github.com/safedep/code/plugin/callgraph"
)

// retargetTyped gives a call on a receiver that a declared type of the
// scope names the key of the name in its scope as its target, in a language
// with typedReceivers. The call graph bound the receiver to the value of an
// assignment, as dep//new_thing for t = dep.new_thing(), and a call on that
// value, as dep//new_thing//do, names no method. The declared type or the
// result type of the name does. A call that the call graph bound to a
// method of a class of the file, as Parser//render for p = Parser(), keeps
// that target.
func (p *projection) retargetTyped(call *fileindex.Call) {
	for _, t := range call.Targets {
		if def, ok := p.defs[fileindex.Parent(t)]; ok && def.Kind == fileindex.DefClass {
			return
		}
	}
	if target, ok := p.typedTarget(call.Caller, call.Receiver, call.Method); ok {
		call.Targets = []string{target}
	}
}

func (p *projection) addCalls() {
	for key, node := range p.cg.Nodes {
		for _, ref := range node.CallsTo {
			kind, ok := callKind(ref.Kind)
			if !ok {
				continue
			}
			if p.isRequire(ref) {
				continue
			}
			targets := p.targets(ref.Targets)
			if ref.Unbound {
				targets = p.rebind(key, ref.Method)
				if len(targets) == 0 && p.rules().rawUnbound {
					targets = []string{ref.CalleeNamespace}
				}
			}
			targets = p.throughRequire(ref.Receiver, targets)
			if local, ok := p.objectMember(key, targets); ok {
				targets = local
				p.members[[2]uint32{ref.Site.StartLine, ref.Site.StartColumn}] = true
			}
			call := fileindex.Call{
				Caller:   p.caller(key, ref.Site),
				Line:     int(ref.Site.StartLine),
				Column:   int(ref.Site.StartColumn),
				Callee:   ref.Text,
				Targets:  targets,
				Receiver: ref.Receiver,
				Method:   ref.Method,
				Kind:     kind,
				Args:     p.arguments(key, ref.Site),
			}
			p.retargetTyped(&call)
			p.ix.Calls = append(p.ix.Calls, call)
		}
	}
}

// objectMember returns the definition that a JavaScript call of a member of
// an object of the file names, as api//run for api.run() after
// const api = { run() {} }. The call graph keeps no value of an object
// literal, so its target has no unit. Only a target whose whole key is a
// definition changes.
func (p *projection) objectMember(caller string, targets []string) ([]string, bool) {
	if !p.ix.Language.IsECMAScript() || len(targets) != 1 || fileindex.HasKeyPrefix(targets[0], p.cg.FileName) {
		return nil, false
	}
	bound := p.rebind(caller, fileindex.Unit(targets[0]))
	if len(bound) != 1 || len(fileindex.Segments(targets[0])) == 0 {
		return nil, false
	}
	local := fileindex.Join(bound[0], fileindex.Segments(targets[0])...)
	if def := p.defs[local]; def == nil || def.Kind != fileindex.DefMethod && def.Kind != fileindex.DefFunction {
		return nil, false
	}
	return []string{local}, true
}

// caller returns the definition that runs the call site that the
// definition key holds. A function that a JavaScript property or argument
// holds, and the static initializer of a Java class, are definitions that
// the call graph does not know, so they take the calls of their nodes. A
// JavaScript static field or static block runs when the class is defined,
// so its calls belong to the scope that defines the class. A function
// value in a static field runs only when a call runs it. A flag in
// a static block stays on the class.
func (p *projection) caller(key string, site ast.NodePosition) string {
	def, ok := p.defs[key]
	class := ok && def.Kind == fileindex.DefClass && p.ix.Language.IsECMAScript()
	if !class && len(p.bodies) == 0 {
		return key
	}
	var own span
	if node := p.syntax[key]; node != nil {
		own = spanOf(node)
	}
	for node := p.descendant(site); node != nil && spanOf(node) != own; node = node.Parent() {
		if body, ok := p.bodies[spanOf(node)]; ok {
			return body
		}
		if !class {
			continue
		}
		switch node.Type() {
		case "class_body":
			return key
		case "arrow_function", "function_expression", "function", "generator_function":
			return key
		case "class_static_block":
			return fileindex.Parent(key)
		case "field_definition", "public_field_definition":
			if ts.HasKeyword(node, "static") {
				return fileindex.Parent(key)
			}
		}
	}
	return key
}

// isRequire reports whether a call is a CommonJS require of the builtin
// require. The import record of the call names the module, so the call
// itself makes no call record.
func (p *projection) isRequire(ref callgraph.CallReference) bool {
	return p.ix.Language.IsECMAScript() && ref.Text == requireFunction && slices.Equal(ref.Targets, []string{requireFunction})
}

// requireCall matches a receiver that starts with a require of a string, as
// require('js-yaml') in require('js-yaml').load.
var requireCall = regexp.MustCompile("^require\\(\\s*['\"`]([^'\"`]+)['\"`]\\s*\\)")

// throughRequire gives a member of a require call the namespace of the
// module, as js-yaml//load for require('js-yaml').load. The call graph keys
// the member under require.
func (p *projection) throughRequire(receiver string, targets []string) []string {
	if !p.ix.Language.IsECMAScript() {
		return targets
	}
	m := requireCall.FindStringSubmatch(receiver)
	if m == nil {
		return targets
	}
	out := make([]string, 0, len(targets))
	for _, t := range targets {
		if rest, ok := strings.CutPrefix(t, "require"+fileindex.Separator); ok {
			t = fileindex.Join(fileindex.Namespace(p.ix.Language, m[1]), rest)
		}
		out = append(out, t)
	}
	return out
}

// callKind maps the kind of a call graph edge to the kind of a call. An
// edge that no call site makes has no call.
func callKind(kind callgraph.CallKind) (fileindex.CallKind, bool) {
	switch kind {
	case callgraph.CallCall:
		return fileindex.CallCall, true
	case callgraph.CallNew:
		return fileindex.CallNew, true
	case callgraph.CallImport:
		return fileindex.CallImport, true
	default:
		return "", false
	}
}

// targets replaces an instance key, as Parser//this//parse, with the method
// that its synthetic edge reaches, so a target is a definition key.
func (p *projection) targets(targets []string) []string {
	var resolved []string
	for _, target := range targets {
		resolved = append(resolved, p.throughInstance(target, 0)...)
	}
	slices.Sort(resolved)
	return slices.Compact(resolved)
}

func (p *projection) throughInstance(target string, hops int) []string {
	node, ok := p.cg.Nodes[target]
	if !ok || node.Kind != "" || len(node.CallsTo) == 0 || hops >= maxInstanceHops {
		return []string{target}
	}
	var methods []string
	for _, ref := range node.CallsTo {
		if ref.Kind != callgraph.CallSynthetic {
			return []string{target}
		}
		methods = append(methods, p.throughInstance(ref.CalleeNamespace, hops+1)...)
	}
	return methods
}

// rebind binds a name that the scope chain of the build did not bind to a
// definition of the file, as a function that the file declares after the
// call. It looks from the scope of the caller out to the module, and skips
// a class scope, as the languages do. A Java class scope holds the methods
// and the nested classes that the class calls with no receiver. A name
// with no definition has no target.
func (p *projection) rebind(caller, name string) []string {
	if name == "" || !fileindex.HasKeyPrefix(caller, p.cg.FileName) {
		return nil
	}
	for scope := caller; ; {
		if def, ok := p.defs[scope]; ok && (def.Kind != fileindex.DefClass || scope == caller || p.rules().memberScopes) {
			if _, bound := p.defs[fileindex.Join(scope, name)]; bound {
				return []string{fileindex.Join(scope, name)}
			}
		}
		if scope == p.cg.FileName {
			return nil
		}
		scope = fileindex.Parent(scope)
	}
}

func (p *projection) addUnresolved() {
	for _, u := range p.cg.Unresolved() {
		if p.members[[2]uint32{u.Site.StartLine, u.Site.StartColumn}] && u.Reason == callgraph.UnresolvedUnknownReceiver {
			continue
		}
		caller := p.caller(u.Caller, u.Site)
		if target, ok := p.typedTarget(caller, u.Receiver, u.Method); ok && u.Reason == callgraph.UnresolvedUnknownReceiver {
			p.ix.Calls = append(p.ix.Calls, fileindex.Call{
				Caller:   caller,
				Line:     int(u.Site.StartLine),
				Column:   int(u.Site.StartColumn),
				Callee:   u.Text,
				Targets:  []string{target},
				Receiver: u.Receiver,
				Method:   u.Method,
				Kind:     fileindex.CallCall,
			})
			continue
		}
		p.ix.Unresolved = append(p.ix.Unresolved, fileindex.Unresolved{
			Caller:   caller,
			Line:     int(u.Site.StartLine),
			Column:   int(u.Site.StartColumn),
			Text:     u.Text,
			Receiver: u.Receiver,
			Method:   u.Method,
			Reason:   fileindex.UnresolvedReason(u.Reason),
		})
	}
}

// typedTarget returns the target of a call on a receiver whose first name a
// declared type of the scope chain of the caller types, as main.py//run//t//do
// for t.do() in run(t: Thing), in a language with typedReceivers. The link
// step reads the type of the name from the scope, as it does for Go.
func (p *projection) typedTarget(caller, receiver, method string) (string, bool) {
	if !p.rules().typedReceivers || receiver == "" || method == "" {
		return "", false
	}
	names := strings.Split(receiver, ".")
	for _, name := range names {
		if !isIdentifier(name) {
			return "", false
		}
	}
	scope, ok := p.typedScope(caller, names[0])
	if !ok {
		return "", false
	}
	return fileindex.Join(scope, append(names, method)...), true
}

func (p *projection) addTruncations() {
	for _, t := range p.cg.Truncations() {
		p.truncate(fileindex.TruncationKind(t.Kind), t.Namespace, t.Line)
	}
}

// truncate records the first hit of a kind of limit.
func (p *projection) truncate(kind fileindex.TruncationKind, namespace string, line int) {
	if slices.ContainsFunc(p.ix.Truncated, func(t fileindex.Truncation) bool { return t.Kind == kind }) {
		return
	}
	p.ix.Truncated = append(p.ix.Truncated, fileindex.Truncation{Kind: kind, Namespace: namespace, Line: line})
}
