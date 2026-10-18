// Package summary gives the summary of one package version of dependency
// code: for each exported symbol of the package, the external symbols of
// its own dependencies that the symbol reaches, and the uncertainties of
// the search. One index and one link of the package alone give it, so a
// job can store one summary per package version, and a project can join a
// call of a symbol to the summary of its package. The summary stores reach
// sets and no path. A caller that needs the path of one reach asks Result
// for it. The package owns no search rule: reach computes the sets and
// scopes the uncertainties, and the summary only filters and records them.
package summary

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/projectgraph"
	"github.com/safedep/code/projectgraph/modules"
	"github.com/safedep/code/projectgraph/reach"
)

// Version is the version of the summary format and of the rules that fill
// it. Bump it when a field or a rule changes. A store keys a summary by
// Version, IndexVersion and LinkVersion.
const Version = "2"

// Summary is the summary of one package unit. Symbols are in key order.
// Targets holds every external symbol that the code of the unit calls,
// other than the symbols of the unit and the builtins of its languages, in
// key order. Uncertainties is the table that each symbol indexes.
// Incomplete lists the files of the unit with no complete index, in path
// order. Such a file can hold a symbol, so a symbol that Symbols does not
// hold is not known when Incomplete is not empty. The JSON form of one
// input is one byte sequence.
type Summary struct {
	Version       string            `json:"version"`
	IndexVersion  string            `json:"index_version"`
	LinkVersion   string            `json:"link_version"`
	Unit          string            `json:"unit"`
	Symbols       []Symbol          `json:"symbols"`
	Targets       []string          `json:"targets"`
	Uncertainties []reach.Scoped    `json:"uncertainties,omitempty"`
	Incomplete    []string          `json:"incomplete,omitempty"`
	Problems      []modules.Problem `json:"problems,omitempty"`
}

// Symbol is one exported symbol of the package. Key is the external key
// that an importer writes, as lodash//template, and Definitions are the
// nodes of the package that the key links to. Reaches holds each target of
// Summary.Targets that the symbol reaches, in key order, with no depth cap.
// Uncertainties holds the indexes into Summary.Uncertainties of each
// uncertainty of the search, ascending, so a target that Reaches does not
// hold is not proved unreached when one of them concerns it. Status
// applies that rule.
type Symbol struct {
	Key           string   `json:"key"`
	Definitions   []string `json:"definitions"`
	Reaches       []string `json:"reaches"`
	Uncertainties []int    `json:"uncertainties,omitempty"`
}

// Result is a summary with the graph of the unit that it comes from, so a
// caller can ask for the path of one reach.
type Result struct {
	Summary  *Summary
	graph    *projectgraph.Graph
	explorer *reach.Explorer
}

// Build links the indexes of one package unit alone and summarizes it.
// Every index and every file of opts.Files must be of the one unit, and
// opts.Units gives its manifests.
func Build(ctx context.Context, indexes []*fileindex.Index, opts projectgraph.Options) (*Result, error) {
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
	// A target is an external node that is no symbol of the package and
	// no builtin.
	isTarget := func(key string) bool {
		_, own := definitions[key]
		return !own && !builtin(languages, key)
	}

	s := &Summary{Version: Version, IndexVersion: fileindex.Version, LinkVersion: projectgraph.Version, Unit: unit, Symbols: []Symbol{}, Targets: []string{}, Problems: g.Problems}
	for _, p := range slices.Sorted(maps.Keys(g.Files)) {
		if !g.Files[p].Complete() {
			s.Incomplete = append(s.Incomplete, p)
		}
	}
	for _, key := range slices.Sorted(maps.Keys(g.Nodes)) {
		if g.Nodes[key].Kind == projectgraph.NodeExternal && isTarget(key) {
			s.Targets = append(s.Targets, key)
		}
	}

	keys := slices.Sorted(maps.Keys(definitions))
	explorer := reach.NewExplorer(g)
	sets, err := explorer.ExploreAll(ctx, keys)
	if err != nil {
		return nil, err
	}
	// A walk that the context stopped leaves the sets partial, so the
	// summary is no summary.
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.Uncertainties = sets.Uncertainties
	for i := range s.Uncertainties {
		s.Uncertainties[i].Targets = slices.DeleteFunc(s.Uncertainties[i].Targets, func(key string) bool { return !isTarget(key) })
	}
	// Two symbols that reach one set share its reached slice, so the
	// filter runs once per set.
	filtered := map[*string][]string{}
	for i, key := range keys {
		src := sets.Sources[i]
		reaches := []string{}
		if len(src.Reached) > 0 {
			if r, ok := filtered[&src.Reached[0]]; ok {
				reaches = r
			} else {
				reaches = slices.DeleteFunc(slices.Clone(src.Reached), func(key string) bool { return !isTarget(key) })
				filtered[&src.Reached[0]] = reaches
			}
		}
		defs := slices.Compact(slices.Sorted(slices.Values(definitions[key])))
		if defs == nil {
			defs = []string{}
		}
		s.Symbols = append(s.Symbols, Symbol{Key: key, Definitions: defs, Reaches: reaches, Uncertainties: src.Uncertainties})
	}
	return &Result{Summary: s, graph: g, explorer: explorer}, nil
}

// Summarize links the indexes of one package unit alone and returns its
// summary, as Build does.
func Summarize(ctx context.Context, indexes []*fileindex.Index, opts projectgraph.Options) (*Summary, error) {
	r, err := Build(ctx, indexes, opts)
	if err != nil {
		return nil, err
	}
	return r.Summary, nil
}

// Graph returns the graph of the unit that the summary comes from.
func (r *Result) Graph() *projectgraph.Graph { return r.graph }

// Path returns the shortest path from a symbol of the summary to a target
// that it reaches, with one search inside the unit, or false when the
// symbol does not reach the target. The search has no depth cap, as the
// sets of the summary have none, so every target of Reaches has a path.
// A summary stores no path, so a caller asks for one when a finding
// needs it.
func (r *Result) Path(ctx context.Context, symbol string, target reach.Target) ([]reach.Hop, bool, error) {
	return r.explorer.Path(ctx, symbol, target, reach.NoDepthCap)
}

// Status answers whether a symbol of the summary reaches a target, as a
// reach query of the unit graph from the symbol answers it. The target is
// reached when Reaches holds a key that matches it. Otherwise an
// uncertainty of the symbol that concerns the target, as reach.Scoped
// decides it, makes it uncertain. A target that nothing concerns is not
// found, or no such target when the summary does not know it. A symbol
// that the summary does not hold is not found, unless Incomplete is not
// empty.
func (s *Summary) Status(symbol string, target reach.Target) reach.Status {
	i, ok := slices.BinarySearchFunc(s.Symbols, symbol, func(sym Symbol, key string) int { return strings.Compare(sym.Key, key) })
	if !ok {
		if len(s.Incomplete) > 0 {
			return reach.Uncertain
		}
		return reach.NotFound
	}
	sym := s.Symbols[i]
	if target.MatchesSorted(sym.Reaches) {
		return reach.Found
	}
	for _, idx := range sym.Uncertainties {
		if s.Uncertainties[idx].Concerns(target) {
			return reach.Uncertain
		}
	}
	if target.MatchesSorted(s.Targets) {
		return reach.NotFound
	}
	return reach.NoSuchTarget
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
