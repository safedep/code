package projectgraph

import (
	"path"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/projectgraph/modules"
)

// kotlinProfile shares the class table and the rules of Java. Kotlin and
// Java files of one package are one unit, so each calls the other.
var kotlinProfile = profile{
	code:            langcode.Kotlin,
	family:          langcode.Java,
	resolver:        func(b *builder) modules.Resolver { return javaResolver{b} },
	link:            (*builder).linkKotlin,
	qualified:       (*builder).resolveJava,
	scopeFallback:   (*builder).javaScopeFallback,
	instance:        []string{"this"},
	thisCalls:       true,
	superCalls:      true,
	implicitBases:   true,
	implicitFields:  true,
	nestedTypes:     true,
	annotationsData: true,
	scopeWildcards:  true,
	exactWildcards:  true,
}

// kotlinExtensionReceiver is the name of the parameter that holds the
// receiver of an extension function in the index.
const kotlinExtensionReceiver = "this"

// kotlinOperators name the member functions that Kotlin code calls with no
// call of the name: an operator, as plus for a + b, a destructuring, as
// component1, a for loop, a delegated property, and the methods of Any that
// the runtime calls.
var kotlinOperators = map[string]bool{
	"plus": true, "minus": true, "times": true, "div": true, "rem": true, "mod": true,
	"rangeTo": true, "rangeUntil": true, "contains": true, "get": true, "set": true,
	"invoke": true, "compareTo": true, "equals": true, "hashCode": true, "toString": true,
	"iterator": true, "next": true, "hasNext": true, "getValue": true, "setValue": true,
	"provideDelegate": true, "inc": true, "dec": true, "not": true, "unaryPlus": true,
	"unaryMinus": true, "plusAssign": true, "minusAssign": true, "timesAssign": true,
	"divAssign": true, "remAssign": true, "component1": true, "component2": true,
	"component3": true, "component4": true, "component5": true,
}

// kotlinFacade returns the name of the class that holds the top-level
// functions of a Kotlin file for Java code, as UtilsKt for utils.kt.
func kotlinFacade(p string) string {
	base := strings.TrimSuffix(path.Base(p), path.Ext(p))
	r, size := utf8.DecodeRuneInString(base)
	return string(unicode.ToUpper(r)) + base[size:] + "Kt"
}

// linkKotlin adds the edges that Kotlin runs with no call in the source.
//
// A companion object runs its initializers when the class loads, as the
// static initializer <clinit> of Java, and the edges to it are the edges
// of Java.
//
// A method runs on an instance, so its class was made and the code of the
// class body ran: the property initializers, the init blocks and the
// getters, whose calls the class holds. So each method that is not static
// has an edge to its class. A member operator runs where an expression
// uses it, so it has a reference edge from its class, and an extension
// operator on a class of the project has one from that class.
//
// An extension function runs where a call names it on a receiver whose
// type has no member of the name. So a call with a receiver that runs no
// first-party member gets a name edge to each extension of its method name
// that the file sees: one of its package, one that it imports, or one of a
// package of a wildcard import. The receiver type is not checked, so the
// edge is a name edge and the record of the call stays.
func (b *builder) linkKotlin(ix *fileindex.Index) {
	resolver := func(rule string) string { return resolverName(ix.Language, rule) }
	b.linkClassInits(ix)
	b.linkFileInits(ix)
	for _, c := range ix.Classes {
		for _, m := range c.Methods {
			def := b.defs[m]
			if def == nil || !b.isMethod(m) {
				continue
			}
			n := b.g.Nodes[m]
			site := Site{Path: n.Path, Line: n.Line}
			if !def.Static {
				b.addEdge(Edge{From: m, To: c.Key, Site: site, Kind: EdgeDirect, Resolver: resolver("instance")})
			}
			if kotlinOperators[def.Name] {
				b.addEdge(Edge{From: c.Key, To: m, Site: site, Kind: EdgeReference, Resolver: resolver("operator")})
			}
		}
	}
	sc := b.scopes[ix.Path]
	for _, def := range ix.Definitions {
		receiver, ok := extensionReceiver(&def)
		if !ok || !kotlinOperators[def.Name] {
			continue
		}
		if class, ok := b.resolveType(sc, def.Key, receiver); ok {
			site := Site{Path: ix.Path, Line: def.StartLine}
			b.addEdge(Edge{From: class, To: def.Key, Site: site, Kind: EdgeReference, Resolver: resolver("operator")})
		}
	}
	link := func(caller string, line, column int, receiver, method string) {
		if receiver == "" || method == "" {
			return
		}
		site := Site{Path: ix.Path, Line: line, Column: column}
		for _, key := range b.extensions(sc, method) {
			b.addEdge(Edge{From: b.runner(caller), To: key, Site: site, Kind: EdgeName, Resolver: resolver("extension")})
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

// linkFileInits adds the edges to the module top level of a Kotlin file,
// which holds the initializers of its top-level properties. They run when
// the class of the file loads: before a top-level function of the file
// runs, and before code reads one of its properties. So each top-level
// function of the file has an edge to the module, and each call in the
// file on a top-level property of a file of the package or of an import
// has an edge to the module of the property.
func (b *builder) linkFileInits(ix *fileindex.Index) {
	for _, def := range ix.Definitions {
		if def.Kind == fileindex.DefFunction && def.Parent == "" && len(fileindex.Segments(def.Key)) == 1 {
			b.addEdge(Edge{From: def.Key, To: ix.Path, Site: Site{Path: ix.Path, Line: def.StartLine}, Kind: EdgeDirect, Resolver: resolverName(ix.Language, "file-init")})
		}
	}
	sc := b.scopes[ix.Path]
	for _, call := range ix.Calls {
		for _, t := range call.Targets {
			for _, key := range b.properties(sc, t) {
				if p := fileindex.Unit(key); p != ix.Path {
					site := Site{Path: ix.Path, Line: call.Line, Column: call.Column}
					b.addEdge(Edge{From: b.runner(call.Caller), To: p, Site: site, Kind: EdgeDirect, Resolver: resolverName(ix.Language, "file-init")})
				}
			}
		}
	}
}

// properties returns the top-level Kotlin properties that the first name of
// a per-file target names: one of the package of the file, or one that the
// file imports.
func (b *builder) properties(sc *scope, target string) []string {
	first, _ := splitKey(target)
	keys := slices.Clone(b.unitNames(sc.ix.Path, first))
	if imp, _, ok := sc.importFor(target); ok {
		if key, ok := b.resolveJava(imp.Module); ok {
			keys = append(keys, key)
		}
	}
	return slices.DeleteFunc(keys, func(k string) bool {
		return b.kindOf(k) != NodeVariable || b.languageOf(k) != langcode.Kotlin
	})
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

// extensions returns the extension functions of the name that a Kotlin
// file sees.
func (b *builder) extensions(sc *scope, name string) []string {
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

// extensionReceiver returns the receiver type of a Kotlin extension
// function.
func extensionReceiver(def *fileindex.Definition) (string, bool) {
	if def.Kind != fileindex.DefFunction || len(def.Params) == 0 || def.Params[0].Name != kotlinExtensionReceiver {
		return "", false
	}
	return def.Params[0].Type, true
}
