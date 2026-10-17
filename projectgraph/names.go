package projectgraph

import (
	"cmp"
	"strings"

	"github.com/safedep/code/fileindex"
)

// resolveName returns the key that a name of a module stands for. A name
// that the module re-exports gets a node of kind export in the module, with
// an export edge to what it names.
func (b *builder) resolveName(module, name string, hops int) (string, *failure) {
	k := nameKey{module: module, name: name}
	if r, ok := b.names[k]; ok {
		return r.key, r.fail
	}
	if b.visiting[k] {
		return "", &failure{reason: fileindex.UnresolvedNoDefinition, name: name, cut: true}
	}
	if hops > b.opts.maxExportHops {
		path := strings.TrimSuffix(module, "/")
		b.truncations[Truncation{Kind: TruncationExportHops, Key: fileindex.Join(path, name), Site: Site{Path: path}}] = true
		return "", &failure{reason: fileindex.UnresolvedLimit, name: name, cut: true}
	}
	b.visiting[k] = true
	key, fail := b.lookupName(module, name, hops)
	delete(b.visiting, k)
	if fail == nil || !fail.cut {
		b.names[k] = resolution{key: key, fail: fail}
	}
	return key, fail
}

func (b *builder) lookupName(module, name string, hops int) (string, *failure) {
	if isNamespacePackage(module) {
		if sub, ok := b.submodule(module, name); ok {
			return sub, nil
		}
		return "", noDefinition(name)
	}
	sc := b.scopes[module]
	if sc == nil {
		return module, nil
	}
	if n := b.g.Nodes[fileindex.Join(module, name)]; n != nil && n.Kind != NodeExport {
		return n.Key, nil
	}

	miss := noDefinition(name)
	try := func(_ string, fail *failure) bool {
		if fail == nil {
			return true
		}
		miss.cut = miss.cut || fail.cut
		if fail.reason == fileindex.UnresolvedLimit {
			miss.reason = fail.reason
		}
		return false
	}
	for _, ex := range sc.exports[name] {
		if key, fail := b.followExport(sc, ex, hops); try(key, fail) {
			return key, nil
		}
	}
	moduleNames := sc.profile.moduleNames
	if imp, ok := sc.bindings[name]; ok && moduleNames && !sc.hasExports {
		if key, fail := b.followBinding(sc, imp, name, hops); try(key, fail) {
			return key, nil
		}
	}
	if !moduleNames || !sc.hasExports || sc.exported(name) {
		for _, st := range sc.stars {
			if key, fail := b.followStar(sc, st, name, hops); try(key, fail) {
				return key, nil
			}
		}
	}
	if moduleNames {
		if sub, ok := b.submodule(module, name); ok {
			return sub, nil
		}
	}
	return "", miss
}

// followExport resolves an export record of the file. A record with no
// target and no module exports a name that the file imports, and Item holds
// that local name when it differs from Name.
func (b *builder) followExport(sc *scope, ex fileindex.Export, hops int) (string, *failure) {
	switch {
	case ex.Target != "":
		if b.g.Nodes[ex.Target] != nil {
			return ex.Target, nil
		}
		return "", noDefinition(ex.Name)
	case ex.From != "":
		mod, _, ok := b.resolveModule(sc.ix, ex.From)
		var key string
		switch {
		case !ok:
			var names []string
			if ex.Item != "" {
				names = []string{ex.Item}
			}
			key = b.importedExternal(sc.ix, fileindex.JoinNonEmpty(fileindex.Namespace(sc.ix.Language, ex.From), ex.Item), ex.From, names)
		case ex.Item == "":
			key = mod
		default:
			var fail *failure
			if key, fail = b.resolveName(mod, ex.Item, hops+1); fail != nil {
				return "", fail
			}
		}
		return b.bind(sc, ex.Name, ex.Line, key, "reexport"), nil
	}
	imp, ok := sc.bindings[cmp.Or(ex.Item, ex.Name)]
	if !ok {
		return "", noDefinition(ex.Name)
	}
	return b.followBinding(sc, imp, ex.Name, hops)
}

// followBinding resolves a name that the file imports and gives on.
func (b *builder) followBinding(sc *scope, imp fileindex.Import, name string, hops int) (string, *failure) {
	lang := sc.ix.Language
	module, lead := importedModule(lang, imp)
	mod, _, ok := b.resolveModule(sc.ix, module)
	var key string
	switch {
	case !ok:
		key = b.importedExternal(sc.ix, boundNamespace(lang, imp), module, lead)
	case len(lead) > 0:
		var fail *failure
		if key, fail = b.resolveName(mod, lead[0], hops+1); fail != nil {
			return "", fail
		}
	case sc.profile.defaultExport != "":
		key = mod
		if def, fail := b.resolveName(mod, sc.profile.defaultExport, hops+1); fail == nil {
			key = def
		}
	default:
		key = mod
	}
	return b.bind(sc, name, imp.Line, key, "reexport"), nil
}

// followStar resolves a name through an export * from or a Python
// from m import *. Python gives on only the names of __all__, or every name
// with no leading underscore.
func (b *builder) followStar(sc *scope, st star, name string, hops int) (string, *failure) {
	mod, _, ok := b.resolveModule(sc.ix, st.spec)
	if !ok || !b.starVisible(mod, name) {
		return "", noDefinition(name)
	}
	key, fail := b.resolveName(mod, name, hops+1)
	if fail != nil {
		return "", fail
	}
	return b.bind(sc, name, st.line, key, "reexport-star"), nil
}

func (b *builder) bind(sc *scope, name string, line int, target, rule string) string {
	if isNamespacePackage(target) {
		return target
	}
	key := fileindex.Join(sc.ix.Path, name)
	b.addNode(&Node{Key: key, Kind: NodeExport, Path: sc.ix.Path, Line: line, Exported: true, Test: b.test[sc.ix.Path]})
	b.bindings[key] = target
	b.addEdge(Edge{
		From:     key,
		To:       target,
		Site:     Site{Path: sc.ix.Path, Line: line},
		Kind:     EdgeExport,
		Resolver: resolverName(sc.ix.Language, rule),
	})
	return key
}

func (b *builder) deref(key string) string {
	for b.kindOf(key) == NodeExport {
		key = b.bindings[key]
	}
	return key
}
