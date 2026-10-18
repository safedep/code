package projectgraph

import (
	"context"
	"slices"
	"strings"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/fileindex/roots"
	"github.com/safedep/code/projectgraph/rootmatch"
)

// markRoots adds the roots that the rules mark. With no root of kind main,
// manifest or framework, every exported definition and every module top
// level of a file that is not a test becomes a root of kind
// library-surface. Dependency code runs only when the project calls it, so
// it holds no root.
func (b *builder) markRoots(ctx context.Context) error {
	indexes := make([]*fileindex.Index, 0, len(b.byPath))
	for _, p := range b.paths {
		if ix := b.byPath[p]; ix != nil && ix.Unit == "" {
			indexes = append(indexes, ix)
		}
	}
	found, problems, err := rootmatch.Find(ctx, rootmatch.Project{
		Indexes:     indexes,
		IsTest:      func(p string) bool { return b.test[p] },
		Root:        b.opts.Root,
		Files:       b.project.files,
		Python:      b.project.python,
		JS:          b.project.js,
		Defined:     func(key string) bool { return b.g.Nodes[key] != nil && b.kindOf(key) != NodeExternal },
		Exports:     func(p string) []string { return b.exportKeys(p, map[string]bool{}) },
		Export:      b.exportKey,
		Resolve:     b.resolveKeys,
		ResolveName: b.resolveRuleName,
	}, b.rules)
	if err != nil {
		return err
	}
	b.problems = append(b.problems, problems...)

	entry := false
	for _, r := range found {
		b.addRootAt(r.Key, r.Kind, r.Rule, Site(r.Site))
		entry = entry || r.Kind != RootTest
		// A Go program initializes its package before main runs, so the
		// module top level of each file of the package is a root too.
		if u := b.unitOf[fileindex.Unit(r.Key)]; r.Kind == RootMain && u != nil && b.profileAt(r.Key).mainLoadsPackage {
			for _, p := range u.files {
				b.addRoot(p, r.Kind, r.Rule)
			}
		}
	}
	if entry {
		return nil
	}

	b.g.Stats.RootFallback = true
	for _, ix := range indexes {
		if b.test[ix.Path] || isDeclarationFile(ix.Path) {
			continue
		}
		b.addRoot(ix.Path, RootLibrarySurface, string(RootLibrarySurface))
		for _, def := range ix.Definitions {
			if def.Exported && def.Key != ix.Path && def.Kind != fileindex.DefVariable && def.Kind != fileindex.DefInterface {
				b.addRoot(def.Key, RootLibrarySurface, string(RootLibrarySurface))
			}
		}
	}
	return nil
}

func (b *builder) addRoot(key string, kind roots.Kind, rule string) {
	b.addRootAt(key, kind, rule, Site{})
}

// addRootAt adds a root at the site that registers it. With no site, the
// root is at its own node.
func (b *builder) addRootAt(key string, kind roots.Kind, rule string, site Site) {
	if site.Path == "" {
		n := b.g.Nodes[key]
		site = Site{Path: n.Path, Line: n.Line}
	}
	b.g.Roots = append(b.g.Roots, Root{Key: key, Kind: kind, Rule: rule, Site: site})
}

// exportKey returns the definition that a file exports by the name.
func (b *builder) exportKey(p, name string) (string, bool) {
	if b.scopes[p] == nil {
		return "", false
	}
	key, fail := b.resolveName(p, name, 0)
	return key, fail == nil
}

// resolveKeys returns the keys that the per-file targets of a site resolve
// to, past the export hops.
func (b *builder) resolveKeys(ix *fileindex.Index, caller string, targets []string) []string {
	var keys []string
	for _, r := range b.resolveTargets(b.scopes[ix.Path], caller, targets) {
		if r.fail == nil {
			keys = append(keys, b.deref(r.key))
		}
	}
	slices.Sort(keys)
	return slices.Compact(keys)
}

// resolveRuleName resolves a dotted name as a file writes it, as app.route
// in a decorator. A name under a first-party function that the walk cannot
// follow, as cli.command, keeps its key under the function, so a rule can
// read the decorators of the function.
func (b *builder) resolveRuleName(p, name string) (string, bool) {
	sc := b.scopes[p]
	if sc == nil {
		return "", false
	}
	if key, ok := b.resolveScopeName(sc, name); ok {
		return key, true
	}
	first, rest, ok := strings.Cut(name, ".")
	if !ok {
		return "", false
	}
	key, ok := b.resolveScopeName(sc, first)
	if !ok || b.kindOf(key) == "" {
		return "", false
	}
	return fileindex.Join(key, strings.Split(rest, ".")...), true
}

// exportKeys returns the keys of the names that a file exports. A name that
// the file gives on from another file is an export node, so a path from the
// root keeps the hop. An export * gives the names of its module. A file of
// a language with no export statement exports its exported definitions. An
// interface runs no code, so it is no root.
func (b *builder) exportKeys(p string, seen map[string]bool) []string {
	sc := b.scopes[p]
	if sc == nil || seen[p] || len(seen) > b.opts.maxExportHops {
		return nil
	}
	seen[p] = true
	var keys []string
	if sc.profile.definitionExports {
		for _, def := range sc.ix.Definitions {
			if def.Exported && def.Key != p && def.Kind != fileindex.DefVariable && def.Kind != fileindex.DefInterface {
				keys = append(keys, def.Key)
			}
		}
	}
	for name := range sc.exports {
		if key, fail := b.resolveName(p, name, 0); fail == nil && b.kindOf(b.deref(key)) != NodeInterface {
			keys = append(keys, key)
		}
	}
	for _, st := range sc.stars {
		if mod, _, ok := b.resolveModule(sc.ix, st.spec); ok {
			keys = append(keys, b.exportKeys(mod, seen)...)
		}
	}
	slices.Sort(keys)
	return slices.Compact(keys)
}
