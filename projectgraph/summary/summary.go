// Package summary gives the summary of one package version of dependency
// code: for each exported symbol of the package, the external symbols of
// its own dependencies that the symbol reaches, with the shortest path and
// the uncertainties of the search. One index and one link of the package
// alone give it, so a job can store one summary per package version, and a
// project can join a call of a symbol to the summary of its package.
package summary

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/projectgraph"
	"github.com/safedep/code/projectgraph/modules"
	"github.com/safedep/code/projectgraph/reach"
)

// Version is the version of the summary format and of the rules that fill
// it. Bump it when a field or a rule changes. A store keys a summary by
// Version, IndexVersion and LinkVersion.
const Version = "1"

// Summary is the summary of one package unit. Symbols are in key order. The
// JSON form of one input is one byte sequence. Incomplete lists the files of
// the unit with no complete index, in path order. Such a file can hold a
// symbol, so a symbol that Symbols does not hold is not known when
// Incomplete is not empty.
type Summary struct {
	Version      string            `json:"version"`
	IndexVersion string            `json:"index_version"`
	LinkVersion  string            `json:"link_version"`
	Unit         string            `json:"unit"`
	Symbols      []Symbol          `json:"symbols"`
	Incomplete   []string          `json:"incomplete,omitempty"`
	Problems     []modules.Problem `json:"problems,omitempty"`
}

// Symbol is one exported symbol of the package. Key is the external key
// that an importer writes, as lodash//template, and Definitions are the
// nodes of the package that the key links to. Reaches holds each external
// symbol of a dependency that the symbol reaches, in key order. A path
// starts at Key and goes through the package. Uncertainties holds each
// uncertainty of the search with its scope, so a symbol that Reaches does
// not hold is not proved unreached when an uncertainty concerns it.
type Symbol struct {
	Key           string              `json:"key"`
	Definitions   []string            `json:"definitions"`
	Reaches       []reach.Reached     `json:"reaches"`
	Uncertainties []reach.Uncertainty `json:"uncertainties,omitempty"`
}

// Summarize links the indexes of one package unit alone and summarizes it.
// Every index and every file of opts.Files must be of the one unit, and
// opts.Units gives its manifests.
func Summarize(ctx context.Context, indexes []*fileindex.Index, opts projectgraph.Options) (*Summary, error) {
	unit, err := unitOf(indexes, opts.Files)
	if err != nil {
		return nil, err
	}
	opts.Surface = true
	g, err := projectgraph.Build(ctx, indexes, opts)
	if err != nil {
		return nil, err
	}

	definitions := map[string][]string{}
	for _, e := range g.Edges {
		if e.Kind == projectgraph.EdgePackage && fileindex.PackageOf(e.To) == unit {
			definitions[e.From] = append(definitions[e.From], e.To)
		}
	}
	// A symbol that the package gives with no definition, as a name that
	// it makes at run time, has a record and no edge.
	for _, u := range g.Unresolved {
		if _, ok := definitions[u.Caller]; !ok && fileindex.PackageOf(u.Site.Path) == unit && g.IsExternal(u.Caller) {
			definitions[u.Caller] = nil
		}
	}
	languages := map[langcode.Code]bool{}
	for _, ix := range indexes {
		languages[ix.Language] = true
	}

	s := &Summary{Version: Version, IndexVersion: fileindex.Version, LinkVersion: projectgraph.Version, Unit: unit, Symbols: []Symbol{}, Problems: g.Problems}
	for _, p := range slices.Sorted(maps.Keys(g.Files)) {
		if !g.Files[p].Complete() {
			s.Incomplete = append(s.Incomplete, p)
		}
	}
	explorer := reach.NewExplorer(g)
	for _, key := range slices.Sorted(maps.Keys(definitions)) {
		e, err := explorer.Explore(ctx, []string{key}, 0)
		if err != nil {
			return nil, err
		}
		defs := append([]string{}, definitions[key]...)
		slices.Sort(defs)
		sym := Symbol{Key: key, Definitions: slices.Compact(defs), Reaches: []reach.Reached{}, Uncertainties: e.Uncertainties}
		for _, r := range e.Reached {
			if _, own := definitions[r.Target]; !own && !builtin(languages, r.Target) {
				sym.Reaches = append(sym.Reaches, r)
			}
		}
		s.Symbols = append(s.Symbols, sym)
	}
	return s, nil
}

// unitOf returns the one unit of the indexes and the files.
func unitOf(indexes []*fileindex.Index, files map[string]fileindex.FileStatus) (string, error) {
	units := map[string]bool{}
	for _, ix := range indexes {
		units[ix.Unit] = true
	}
	for p := range files {
		units[fileindex.PackageOf(p)] = true
	}
	switch {
	case units[""]:
		return "", errors.New("a summary takes only files of dependency code")
	case len(units) != 1:
		return "", fmt.Errorf("a summary takes the files of one unit, not %d", len(units))
	}
	return slices.Collect(maps.Keys(units))[0], nil
}

// builtin reports whether a key names a builtin of a language of the
// package, as console//log. A builtin is no symbol of a dependency.
func builtin(languages map[langcode.Code]bool, key string) bool {
	for lang := range languages {
		if langcode.IsBuiltin(lang, fileindex.Unit(key)) {
			return true
		}
	}
	return false
}
