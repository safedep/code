package projectgraph

import (
	"strings"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
)

// typedName returns the class or interface that the declared type of a
// parameter or a typed local variable of the name names. The name belongs
// to the scope or to a function that holds the scope, as the parameter of a
// method in an arrow function of the method. A Go local that holds a
// result of a call has the type of the result at its position. A name
// with no known type gives the reason of the record of a call on it.
func (b *builder) typedName(sc *scope, scopeKey, name string) (string, fileindex.UnresolvedReason, bool) {
	for key := scopeKey; fileindex.HasKeyPrefix(key, sc.ix.Path) && key != sc.ix.Path; key = fileindex.Parent(key) {
		def := b.defs[key]
		if def == nil {
			continue
		}
		for _, typed := range [][]fileindex.Typed{def.Params, def.Locals} {
			for _, t := range typed {
				if t.Name != name {
					continue
				}
				if !t.Result {
					class, ok := b.resolveType(sc, key, t.Type)
					return class, fileindex.UnresolvedUnknownReceiver, ok
				}
				fn, ok := b.resolveScopeName(sc, t.Type)
				if !ok {
					return "", fileindex.UnresolvedUnknownReceiver, false
				}
				return b.resultType(fileindex.PackageOf(sc.ix.Path), fn, t.Index)
			}
		}
	}
	return "", fileindex.UnresolvedUnknownReceiver, false
}

// memberType returns the class of a name in a scope: a typed parameter or
// local variable, or in Java a field of a class that holds the scope, which
// a method names with no this. A name with no known type gives the reason
// of the record of a call on it.
func (b *builder) memberType(sc *scope, scopeKey, name string) (string, fileindex.UnresolvedReason, bool) {
	class, reason, ok := b.typedName(sc, scopeKey, name)
	if ok || !sc.profile.implicitFields {
		return class, reason, ok
	}
	for class, ok := b.enclosingClass(scopeKey); ok; class, ok = b.enclosingClass(fileindex.Parent(class)) {
		if t, ok := b.fieldType(class, name); ok {
			return t, "", true
		}
	}
	return "", reason, false
}

// fieldType returns the class or interface that the declared type of a
// field of the class, or of a class in its MRO, names.
func (b *builder) fieldType(class, name string) (string, bool) {
	for _, c := range b.mro(class) {
		for _, rec := range b.records(c) {
			for _, f := range rec.class.Fields {
				if f.Name == name {
					return b.resolveType(b.scopes[rec.ix.Path], rec.class.Key, f.Type)
				}
			}
		}
	}
	return "", false
}

// resolveType resolves a type name as written at a key of the file to a
// first-party class or interface. A type of a dependency narrows nothing, as an
// instance of a project class can extend it. A Go value holds its declared
// type or, through an interface, a type of the project, so a Go type of a
// dependency gives the external key of its methods.
func (b *builder) resolveType(sc *scope, from, text string) (string, bool) {
	if sc == nil {
		return "", false
	}
	key, ok := b.resolveNameIn(sc, from, text)
	switch {
	case !ok:
		return "", false
	case b.isType(key):
		return key, true
	case sc.profile.implicitInterfaces && b.kindOf(key) == "" && b.g.IsExternal(key) && strings.Contains(key, fileindex.Separator):
		return key, true
	}
	return "", false
}

// dispatches reports whether a receiver of the declared type can hold an
// instance of another type, so a call on it in a file of the language can
// run the method of that type. A Go value of a named type that is not an
// interface holds that type only.
func (b *builder) dispatches(lang langcode.Code, class string) bool {
	if profileOf(lang).implicitInterfaces {
		return b.kindOf(class) == NodeInterface
	}
	return true
}

func (b *builder) languageOf(key string) langcode.Code {
	if ix := b.byPath[fileindex.Unit(key)]; ix != nil {
		return ix.Language
	}
	return ""
}
