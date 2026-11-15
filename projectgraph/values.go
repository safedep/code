package projectgraph

import (
	"cmp"
	"slices"
	"strings"

	"github.com/safedep/code/fileindex"
)

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

// recordValueCalls adds a function-value record at each call that can run
// a function value and that has only a record with its name: a call of a
// parameter or a local, as f(x), or a call of a member whose name a field
// of the graph has, as h.fn(). The name scopes the record to the
// definitions of the name, and a function value has another name. A search
// from the roots reaches the reference to the function, but a search from a
// source can miss the code that stores the value. A field of a type that
// no unit gives can also hold a function, and no definition shows it.
func (b *builder) recordValueCalls() {
	recorded := map[Site]bool{}
	for u := range b.unresolved {
		if u.Reason == fileindex.UnresolvedFunctionValue {
			recorded[u.Site] = true
		}
	}
	var add []UnresolvedSite
	for u := range b.unresolved {
		if u.Reason != fileindex.UnresolvedUnknownReceiver || u.Site.Path == "" || recorded[u.Site] || !b.callsValue(u) {
			continue
		}
		ix := b.byPath[u.Site.Path]
		if ix == nil || !profileOf(ix.Language).functionValues {
			continue
		}
		recorded[u.Site] = true
		add = append(add, UnresolvedSite{Caller: u.Caller, Site: u.Site, Text: u.Text, Receiver: u.Receiver, Reason: fileindex.UnresolvedFunctionValue})
	}
	for _, u := range add {
		b.addUnresolved(u)
	}
}

// callsValue reports whether a call with an unknown receiver can call a
// function value: a name with no receiver, or a member that a field of the
// graph names. Only a field of a language with functionValues counts, and
// not one whose type the index shows to hold data, as Name string.
func (b *builder) callsValue(u UnresolvedSite) bool {
	if u.Receiver == "" {
		return isIdentifier(u.Text)
	}
	if b.fieldNames == nil {
		b.fieldNames = map[string]bool{}
		for key, def := range b.defs {
			if def.Kind == fileindex.DefField && !def.Data && profileOf(b.languageOf(key)).functionValues {
				b.fieldNames[def.Name] = true
			}
		}
	}
	return b.fieldNames[u.Method]
}

// reflection is a package of a language that gives or runs a function
// that no reference names, and the methods of it that do it.
type reflection struct {
	module  string
	methods map[string]bool
}

// reflective reports whether a call or a reference of the method in a file
// that imports the package of reflection can run a function that it gives.
// The link step does not always know that the receiver is a value of the
// package, as when no unit gives the standard library. So any receiver
// counts.
func (r reflection) reflective(ix *fileindex.Index, method string) bool {
	return r.methods[method] && slices.ContainsFunc(ix.Imports, func(imp fileindex.Import) bool { return imp.Module == r.module })
}

// linkReflection adds a computed-name record at each call and each
// reference of a file that can run a function that reflection gives, as
// reflect.ValueOf(f).Call(nil). Such a call can run any function, and a
// function value that reflection gives has no reference edge. A site whose
// every target resolves to a definition outside the package of reflection,
// as stream.Method() on a type of the project, gets no record.
func (b *builder) linkReflection(ix *fileindex.Index) {
	if isDeclarationFile(ix.Path) {
		return
	}
	for _, refl := range profileOf(ix.Language).reflection {
		b.linkReflectionOf(ix, refl)
	}
}

func (b *builder) linkReflectionOf(ix *fileindex.Index, refl reflection) {
	sc := b.scopes[ix.Path]
	for _, call := range ix.Calls {
		if call.Kind == fileindex.CallImport || !refl.reflective(ix, cmp.Or(call.Method, lastWritten(call.Callee))) {
			continue
		}
		caller := call.Caller
		if b.g.Nodes[caller] == nil {
			caller = sc.enclosing(call.Line)
		}
		if b.resolvesOutside(refl.module, sc, caller, call.Targets) {
			continue
		}
		site := Site{Path: ix.Path, Line: call.Line, Column: call.Column}
		b.addUnresolved(UnresolvedSite{Caller: b.runner(caller), Site: site, Text: call.Callee, Receiver: call.Receiver, Reason: fileindex.UnresolvedComputedName})
	}
	for _, ref := range ix.References {
		if !refl.reflective(ix, lastWritten(ref.Text)) || b.resolvesOutside(refl.module, sc, ref.Caller, ref.Targets) {
			continue
		}
		site := Site{Path: ix.Path, Line: ref.Line, Column: ref.Column}
		b.addUnresolved(UnresolvedSite{Caller: b.runner(ref.Caller), Site: site, Text: ref.Text, Reason: fileindex.UnresolvedComputedName})
	}
}

// resolvesOutside reports whether each target of a site resolves to a key
// that is not in the module, in the graph or under its external key.
func (b *builder) resolvesOutside(module string, sc *scope, caller string, targets []string) bool {
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
		return fileindex.Unit(key) == module
	})
}
