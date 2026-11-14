package projectgraph

import (
	"cmp"
	"slices"
	"strings"

	"github.com/safedep/code/fileindex"
)

// functionValue gives a call of a function value the reason function-value
// in a language whose function values the link step joins to the code
// that holds them. The function that such a call runs is a function that
// some code uses as a value, with a reference edge from that code, or a
// function literal, whose calls belong to the code that holds it. The
// search reaches that code before the value exists, so the record names no
// target.
func functionValue(p *profile, reason fileindex.UnresolvedReason) fileindex.UnresolvedReason {
	if p.functionValues && reason == fileindex.UnresolvedComputedName {
		return fileindex.UnresolvedFunctionValue
	}
	return reason
}

// linkUnknownValue records a reference to a member that the link step
// cannot name, as the method value h.Handle on a value of an interface. The
// code that gets the value can call any method of the name, so the record
// keeps the method name. Without it, a call of the value would concern no
// target, because the reference has no edge.
func (b *builder) linkUnknownValue(sc *scope, ref fileindex.Reference, from string, site Site, r resolution) {
	if !sc.profile.functionValues || !strings.Contains(ref.Text, ".") {
		return
	}
	reason := fileindex.UnresolvedUnknownReceiver
	if r.fail != nil {
		reason = r.fail.reason
	}
	b.addUnresolved(UnresolvedSite{Caller: from, Site: site, Text: ref.Text, Method: lastWritten(ref.Text), Reason: reason})
}

// linkReflection adds a computed-name record at each call and each
// reference of a file that can run a function that reflection gives, as
// reflect.ValueOf(f).Call(nil). Such a call can run any function, and a
// function value that reflection gives has no reference edge. A site whose
// every target resolves to a definition outside package reflect, as
// stream.Method() on a type of the project, gets no record.
func (b *builder) linkReflection(ix *fileindex.Index) {
	reflective := profileOf(ix.Language).reflective
	if reflective == nil || isDeclarationFile(ix.Path) {
		return
	}
	sc := b.scopes[ix.Path]
	for _, call := range ix.Calls {
		if call.Kind == fileindex.CallImport || !reflective(ix, cmp.Or(call.Method, lastWritten(call.Callee))) {
			continue
		}
		caller := call.Caller
		if b.g.Nodes[caller] == nil {
			caller = sc.enclosing(call.Line)
		}
		if b.resolvesOutsideReflect(sc, caller, call.Targets) {
			continue
		}
		site := Site{Path: ix.Path, Line: call.Line, Column: call.Column}
		b.addUnresolved(UnresolvedSite{Caller: b.runner(caller), Site: site, Text: call.Callee, Receiver: call.Receiver, Reason: fileindex.UnresolvedComputedName})
	}
	for _, ref := range ix.References {
		if !reflective(ix, lastWritten(ref.Text)) || b.resolvesOutsideReflect(sc, ref.Caller, ref.Targets) {
			continue
		}
		site := Site{Path: ix.Path, Line: ref.Line, Column: ref.Column}
		b.addUnresolved(UnresolvedSite{Caller: b.runner(ref.Caller), Site: site, Text: ref.Text, Reason: fileindex.UnresolvedComputedName})
	}
}

// resolvesOutsideReflect reports whether each target of a site resolves to
// a key that is not in package reflect, in the graph or under its external
// key.
func (b *builder) resolvesOutsideReflect(sc *scope, caller string, targets []string) bool {
	rs := b.resolveTargets(sc, caller, targets)
	return len(rs) > 0 && !slices.ContainsFunc(rs, func(r resolution) bool {
		key := r.key
		switch {
		case r.unbound, r.fail != nil && r.class == "":
			return true
		case r.fail != nil:
			// The declared type of the receiver is known, so the call
			// is on a value of that type.
			key = r.class
		}
		if external, _, ok := b.unitSymbol(key); ok {
			key = external
		}
		return fileindex.Unit(key) == "reflect"
	})
}
