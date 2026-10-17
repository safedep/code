package projectgraph

import (
	"github.com/safedep/code/fileindex"
)

// paramType returns the class or interface that the declared type of a
// parameter names. The parameter belongs to the caller or to a function
// that holds the caller, as the parameter of a method in an arrow function
// of the method.
func (b *builder) paramType(sc *scope, caller, name string) (string, bool) {
	for key := caller; fileindex.HasKeyPrefix(key, sc.ix.Path) && key != sc.ix.Path; key = fileindex.Parent(key) {
		for _, p := range b.params[key] {
			if p.Name == name {
				return b.resolveType(sc, p.Type)
			}
		}
	}
	return "", false
}

// fieldType returns the class or interface that the declared type of a
// field of the class, or of a class in its MRO, names.
func (b *builder) fieldType(class, name string) (string, bool) {
	for _, c := range b.mro(class) {
		rec, ok := b.classes[c]
		if !ok {
			continue
		}
		for _, f := range rec.class.Fields {
			if f.Name == name {
				return b.resolveType(b.scopes[rec.ix.Path], f.Type)
			}
		}
	}
	return "", false
}

// resolveType resolves a type name as written in the file to a first-party
// class or interface. A type of a dependency narrows nothing.
func (b *builder) resolveType(sc *scope, text string) (string, bool) {
	key, ok := b.resolveScopeName(sc, text)
	if !ok || !b.isType(key) {
		return "", false
	}
	return key, true
}
