package projectgraph

import (
	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/fileindex/roots"
	pgroots "github.com/safedep/code/projectgraph/roots"
)

// markRoots adds the roots that the rules mark. With no root of kind main,
// manifest or framework, every exported definition and every module top
// level of a file that is not a test becomes a root of kind
// library-surface.
func (b *builder) markRoots() {
	indexes := make([]*fileindex.Index, 0, len(b.byPath))
	for _, p := range b.paths {
		if ix := b.byPath[p]; ix != nil {
			indexes = append(indexes, ix)
		}
	}
	found, problems := pgroots.Find(pgroots.Project{
		Indexes: indexes,
		IsTest:  func(p string) bool { return b.test[p] },
		Root:    b.opts.Root,
		Files:   b.files,
		Python:  b.python,
		JS:      b.js,
		Defined: func(key string) bool { return b.g.Nodes[key] != nil },
	}, b.rules)
	b.problems = append(b.problems, problems...)

	entry := false
	for _, r := range found {
		b.addRoot(r.Key, r.Kind, r.Rule)
		entry = entry || r.Kind != RootTest
	}
	if entry {
		return
	}

	b.g.Stats.RootFallback = true
	for _, ix := range indexes {
		if b.test[ix.Path] || isDeclarationFile(ix.Path) {
			continue
		}
		b.addRoot(ix.Path, RootLibrarySurface, string(RootLibrarySurface))
		for _, def := range ix.Definitions {
			if def.Exported && def.Key != ix.Path {
				b.addRoot(def.Key, RootLibrarySurface, string(RootLibrarySurface))
			}
		}
	}
}

func (b *builder) addRoot(key string, kind roots.Kind, rule string) {
	n := b.g.Nodes[key]
	b.g.Roots = append(b.g.Roots, Root{Key: key, Kind: kind, Rule: rule, Site: Site{Path: n.Path, Line: n.Line}})
}
