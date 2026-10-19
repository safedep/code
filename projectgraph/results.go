package projectgraph

import (
	"slices"
	"strings"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/projectgraph/modules"
)

// resultType returns the class or interface of the result at a position of
// a function, as a file of the unit of the importer sees it, "" for the
// project. A function of a package
// that the project imports is an external key, and the unit of dependency
// code that gives the package gives its definition. A function of a
// package that no unit gives has no known result, so a call on the result
// is uncertain with the reason no-dependency-source.
func (b *builder) resultType(unit, fn string, pos int) (string, fileindex.UnresolvedReason, bool) {
	defs := []string{fn}
	if b.defs[fn] == nil && b.g.IsExternal(fn) {
		var provided bool
		defs, provided = b.unitDefinitions(b.origins[fn])
		if !provided {
			return "", fileindex.UnresolvedNoDependencySource, false
		}
	}
	for _, d := range defs {
		text := resultText(b.defs[d], pos)
		if text == "" {
			continue
		}
		if class, ok := b.resolveType(b.scopes[fileindex.Unit(d)], d, text); ok {
			return b.typeFor(unit, class), "", true
		}
	}
	return "", fileindex.UnresolvedUnknownReceiver, false
}

// resultText returns the type name of the result at a position of a
// function, as written.
func resultText(def *fileindex.Definition, pos int) string {
	switch {
	case def == nil:
		return ""
	case pos < len(def.Results):
		return def.Results[pos]
	case pos == 0:
		return def.Returns
	}
	return ""
}

// unitDefinitions returns the definitions that the units of dependency
// code give for the origins of an external key, and whether a unit gives
// the package of an origin at all. A walk of a unit can ask for the
// definitions of another external key, so a key that a walk reads now
// gives none.
func (b *builder) unitDefinitions(origins []origin) ([]string, bool) {
	var defs []string
	provided := false
	for _, o := range origins {
		for _, pv := range b.providers(o) {
			provided = true
			if pv.module == "" || len(o.names) == 0 {
				continue
			}
			k := nameKey{module: pv.module, name: fileindex.Join("", o.names...)}
			if b.stepping[k] {
				continue
			}
			b.stepping[k] = true
			r := b.walk(pv.module, o.names)
			delete(b.stepping, k)
			if r.fail == nil && b.defs[r.key] != nil && !slices.Contains(defs, r.key) {
				defs = append(defs, r.key)
			}
		}
	}
	return defs, provided
}

// externalResult returns the type of the result of the function or the
// method of the name under an external key, as golang.org//x//net//html//Tokenizer
// for NewTokenizer under golang.org//x//net//html. A unit of dependency
// code gives the function, and the importer of the key sees the type.
func (b *builder) externalResult(parent, name string) (string, bool) {
	origins := derivedOrigins(b.origins[parent], name)
	if len(origins) == 0 {
		return "", false
	}
	defs, _ := b.unitDefinitions(origins)
	for _, d := range defs {
		if !b.isFunction(d) {
			continue
		}
		if class, _, ok := b.resultType(origins[0].unit, d, 0); ok {
			return class, true
		}
	}
	return "", false
}

// derivedOrigins returns the origins of the name under a key with the
// origins.
func derivedOrigins(origins []origin, names ...string) []origin {
	out := make([]origin, 0, len(origins))
	for _, o := range origins {
		o.names = append(slices.Clone(o.names), names...)
		out = append(out, o)
	}
	return out
}

// typeFor returns the key of a class as a file of a unit names it, "" for
// the project.
// A class of a unit of dependency code that the importer does not belong
// to is its external key, as golang.org//x//net//html//Tokenizer, so a call
// of its method gets the external key that a target names, and a package
// edge then joins the key to the definition. An interface stays as it is,
// because the class hierarchy analysis reads its implementations.
func (b *builder) typeFor(unit, class string) string {
	if fileindex.PackageOf(class) == unit || b.kindOf(class) == NodeInterface || !b.isType(class) {
		return class
	}
	if key, ok := b.symbolKey(unit, class); ok {
		return key
	}
	return class
}

// entryKey returns the key of a definition that a caller runs: the
// definition itself, or for a definition of a unit of dependency code that
// the caller does not belong to, its external key with a node. The
// package edge of the key then joins it to the definition, and a target
// of the key finds the path.
func (b *builder) entryKey(from, def string) string {
	if sameUnit(from, def) {
		return def
	}
	if key, ok := b.symbolKey(fileindex.PackageOf(from), def); ok {
		return b.external(key)
	}
	return def
}

// symbolKey returns the external key of a definition of a unit of
// dependency code, as an import of its package names it, and records the
// import as an origin of the key for a file of the importer unit.
func (b *builder) symbolKey(importer, def string) (string, bool) {
	unit := fileindex.PackageOf(def)
	sp := b.spaces[unit]
	if unit == "" || sp == nil {
		return "", false
	}
	lang := b.languageOf(def)
	symbol := profileOf(lang.Family()).symbol
	if symbol == nil {
		return "", false
	}
	module, names, ok := symbol(b, sp, def)
	if !ok {
		return "", false
	}
	key := fileindex.Join(fileindex.Namespace(lang, module), names...)
	o := origin{lang: lang, unit: importer, module: module, form: modules.FormImport, names: names}
	if !slices.ContainsFunc(b.origins[key], o.equal) {
		b.origins[key] = append(b.origins[key], o)
	}
	return key, true
}

// importedWalk resolves the names under an import that no file of the
// space of the importer gives, as html//NewTokenizer//Next for
// html.NewTokenizer(r).Next(). The walk reads the names one at a time from
// the key of the module, so the result type of a function that a unit of
// dependency code gives types the next name.
func (b *builder) importedWalk(ix *fileindex.Index, target, module string, form modules.Form, names []string) resolution {
	base, ok := strings.CutSuffix(target, fileindex.Join("", names...))
	if len(names) < 2 || !ok {
		return resolution{key: b.importedExternal(ix, target, module, form, names), kind: EdgeDirect, rule: ruleExternal}
	}
	b.recordOrigin(ix, base, module, form, nil)
	return b.walkFrom(resolution{key: base, kind: EdgeDirect, rule: ruleExternal}, names)
}
