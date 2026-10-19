package projectgraph

import (
	"slices"
	"strings"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/projectgraph/modules"
)

// result is the class or the interface of a result of a function, as a
// file of an importer names it, and the Option or the Result of std that
// holds a Rust result.
type result struct {
	class string
	wraps string
}

// resultType returns the result at a position of a function, as a file of
// the unit of the importer sees it, "" for the project. A function of a
// package that the project imports is an external key, and the unit of
// dependency code that gives the package gives its definition. A function
// of a package that no unit gives has no known result, so a call on the
// result is uncertain with the reason no-dependency-source.
func (b *builder) resultType(unit, fn string, pos int) (result, fileindex.UnresolvedReason, bool) {
	defs := []string{fn}
	if b.defs[fn] == nil && b.g.IsExternal(fn) {
		var provided bool
		defs, provided = b.unitDefinitions(b.origins[fn])
		if !provided {
			return result{}, fileindex.UnresolvedNoDependencySource, false
		}
	}
	for _, d := range defs {
		def := b.defs[d]
		text := resultText(def, pos)
		if text == "" {
			continue
		}
		if class, ok := b.resolveType(b.scopes[fileindex.Unit(d)], d, text); ok {
			return result{class: b.typeFor(unit, class), wraps: def.Wraps}, "", true
		}
	}
	return result{}, fileindex.UnresolvedUnknownReceiver, false
}

// unwrap returns how many of the names after a call a wrapped result
// takes: one for unwrap or expect, which give the value inside, and none
// for a name that the value inside takes after a ?, which the index does
// not record. A method of the wrapper itself, as map of Option, gives no
// type, so it reports false. A result with no wrapper takes no name. Only
// a Rust result has a wrapper.
func unwrap(wraps string, next []string) (int, bool) {
	if wraps == "" || len(next) == 0 {
		return 0, true
	}
	return rustUnwrap(wraps, next[0])
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
			switch {
			case pv.module == "":
				continue
			case len(o.names) == 0:
				// A Rust use can name the item itself, as use dep::Thing.
				if b.defs[pv.module] != nil && !slices.Contains(defs, pv.module) {
					defs = append(defs, pv.module)
				}
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

// externalResult returns the result of the function or the method of the
// name under an external key, as golang.org//x//net//html//Tokenizer for
// NewTokenizer under golang.org//x//net//html. A unit of dependency code
// gives the function, and the type is the external key of a class. A method
// of the same-type methods of the language, as clone in Rust, on a type of
// a unit keeps the type, also when the unit derives it and defines none.
func (b *builder) externalResult(parent, name string) (result, bool) {
	origins := derivedOrigins(b.origins[parent], name)
	if len(origins) == 0 {
		return result{}, false
	}
	defs, _ := b.unitDefinitions(origins)
	for _, d := range defs {
		if !b.isFunction(d) {
			continue
		}
		// The walk reads an external key, so the code that names it is
		// in another space than the unit, and the type is its key.
		if res, _, ok := b.resultType("", d, 0); ok {
			return res, true
		}
	}
	if profileOf(origins[0].lang).sameTypeMethods[name] {
		types, _ := b.unitDefinitions(b.origins[parent])
		if slices.ContainsFunc(types, b.isType) {
			return result{class: parent}, true
		}
	}
	return result{}, false
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
	key, o, ok := b.unitSymbol(def)
	if !ok {
		return "", false
	}
	o.unit = importer
	if !slices.ContainsFunc(b.origins[key], o.equal) {
		b.origins[key] = append(b.origins[key], o)
	}
	return key, true
}

// unitSymbol returns the external key of a definition of a unit of
// dependency code, and the origin of an import that names it, with no
// importer unit.
func (b *builder) unitSymbol(def string) (string, origin, bool) {
	unit := fileindex.PackageOf(def)
	sp := b.spaces[unit]
	if unit == "" || sp == nil {
		return "", origin{}, false
	}
	lang := b.languageOf(def)
	symbol := profileOf(lang.Family()).symbol
	if symbol == nil {
		return "", origin{}, false
	}
	module, names, ok := symbol(b, sp, def)
	if !ok {
		return "", origin{}, false
	}
	return fileindex.Join(fileindex.Namespace(lang, module), names...), origin{lang: lang, module: module, form: modules.FormImport, names: names}, true
}

// markExternals sets the external key of each function, method and
// constructor of a unit of dependency code. A call that the unit makes to
// its own code goes to the definition and not to the external key, so
// reach matches a target against the external key of the definition too.
// Without it, a target that only the code of its unit calls reads as
// not-found, as the methods of the HTTP/2 server of x/net.
func (b *builder) markExternals() {
	for _, n := range b.g.Nodes {
		switch n.Kind {
		case NodeFunction, NodeMethod, NodeConstructor:
		default:
			continue
		}
		if key, _, ok := b.unitSymbol(n.Key); ok {
			n.External = key
		}
	}
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
