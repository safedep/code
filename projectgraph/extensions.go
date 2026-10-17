package projectgraph

import (
	"slices"

	"github.com/safedep/code/fileindex"
)

// extensionParam names the parameter that holds the receiver of an
// extension function in the index, as s in fun String.shout() of Kotlin or
// this string s of C#. No parameter can have the name, so the name marks an
// extension.
const extensionParam = "this"

// linkExtensions adds a name edge from each call with a receiver that runs
// no first-party member to each extension function of its method name that
// the file sees. An extension runs where a call names it on a receiver
// whose type has no member of the name. The link step does not check the
// receiver type, so the edge is a name edge and the record of the call
// stays.
func (b *builder) linkExtensions(ix *fileindex.Index, visible func(sc *scope, name string) []string) {
	sc := b.scopes[ix.Path]
	link := func(caller string, line, column int, receiver, method string) {
		if receiver == "" || method == "" {
			return
		}
		site := Site{Path: ix.Path, Line: line, Column: column}
		for _, key := range visible(sc, method) {
			b.addEdge(Edge{From: b.runner(caller), To: key, Site: site, Kind: EdgeName, Resolver: resolverName(ix.Language, "extension")})
		}
	}
	for _, call := range ix.Calls {
		if call.Kind == fileindex.CallCall && !b.runsMember(sc, call) {
			link(call.Caller, call.Line, call.Column, call.Receiver, call.Method)
		}
	}
	// A call on a value that the index does not know, as a literal, has
	// only its record.
	for _, u := range ix.Unresolved {
		if u.Reason == fileindex.UnresolvedUnknownReceiver {
			link(u.Caller, u.Line, u.Column, u.Receiver, u.Method)
		}
	}
}

// extensionsNamed returns the extension functions of the family of the file
// with the name, in path order.
func (b *builder) extensionsNamed(sc *scope, name string) []string {
	if b.extensionDefs == nil {
		b.extensionDefs = map[familyName][]string{}
		for _, p := range b.paths {
			ix := b.byPath[p]
			if ix == nil {
				continue
			}
			for i := range ix.Definitions {
				if _, ok := extensionReceiver(&ix.Definitions[i]); ok {
					k := familyName{family: languageFamily(ix.Language), name: ix.Definitions[i].Name}
					b.extensionDefs[k] = append(b.extensionDefs[k], ix.Definitions[i].Key)
				}
			}
		}
	}
	return b.extensionDefs[familyName{family: languageFamily(sc.ix.Language), name: name}]
}

// linkImplicitMethods adds a reference edge from each class to each of its
// methods that the language calls with no call of the name, as an
// operator, Dispose at the end of a using block, or ToString in a string.
func (b *builder) linkImplicitMethods(ix *fileindex.Index) {
	names := profileOf(ix.Language).implicitMethods
	for _, c := range ix.Classes {
		for _, m := range c.Methods {
			if def := b.defs[m]; def != nil && b.isMethod(m) && names[def.Name] {
				n := b.g.Nodes[m]
				b.addEdge(Edge{From: c.Key, To: m, Site: Site{Path: n.Path, Line: n.Line}, Kind: EdgeReference, Resolver: resolverName(ix.Language, "implicit")})
			}
		}
	}
}

// runsMember reports whether a call resolves to a first-party method, which
// wins over an extension of the name.
func (b *builder) runsMember(sc *scope, call fileindex.Call) bool {
	for _, r := range b.resolveTargets(sc, call.Caller, call.Targets) {
		if r.fail == nil && b.isMethod(r.key) {
			return true
		}
	}
	return false
}

// kotlinExtensions returns the extension functions of the name that a
// Kotlin file sees: one of its package, one that it imports, or one of a
// package of a wildcard import.
func (b *builder) kotlinExtensions(sc *scope, name string) []string {
	var keys []string
	add := func(key string) {
		if def := b.defs[b.deref(key)]; def != nil && !slices.Contains(keys, def.Key) {
			if _, ok := extensionReceiver(def); ok {
				keys = append(keys, def.Key)
			}
		}
	}
	for _, key := range b.unitNames(sc.ix.Path, name) {
		add(key)
	}
	if imp, ok := sc.bindings[name]; ok {
		if key, ok := b.resolveScopeName(sc, imp.Alias); ok {
			add(key)
		}
	}
	for _, key := range b.wildcardImported(sc, name) {
		add(key)
	}
	slices.Sort(keys)
	return keys
}

// extensionReceiver returns the receiver type of an extension function of
// Kotlin or of an extension method of C#.
func extensionReceiver(def *fileindex.Definition) (string, bool) {
	if def.Kind != fileindex.DefFunction && def.Kind != fileindex.DefMethod || len(def.Params) == 0 || def.Params[0].Name != extensionParam {
		return "", false
	}
	return def.Params[0].Type, true
}
