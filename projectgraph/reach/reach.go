// Package reach answers which targets a project graph reaches from its
// roots. One breadth-first search serves every target of a query.
package reach

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/projectgraph"
)

const (
	defaultMaxDepth = 64
	// maxPerKind caps the uncertainties of one kind on a result. The first
	// ones are the nearest to a root, and they are enough to explain the
	// answer.
	maxPerKind = 3
	// wildcard ends a target that matches every external key under it.
	wildcard = fileindex.Separator + "*"
)

type Status string

const (
	Found    Status = "found"
	NotFound Status = "not-found"
	// Uncertain is a target with no path and an uncertainty that the root
	// fallback alone does not explain.
	Uncertain Status = "uncertain"
	// NoSuchTarget is a target that names no external node of the graph,
	// with no uncertainty that could name it. A key in another form, as
	// Thread//sleep for java//lang//Thread//sleep, gets it.
	NoSuchTarget Status = "no-such-target"
)

type UncertaintyKind string

const (
	UnresolvedCall UncertaintyKind = "unresolved-call"
	DynamicFlag    UncertaintyKind = "dynamic-flag"
	// Budget is a truncated or missing file, a limit of the link step, or a
	// search that the depth cap or the deadline stopped.
	Budget       UncertaintyKind = "budget"
	RootFallback UncertaintyKind = "root-fallback"
)

// Query asks for the targets, as external keys like yaml//load. A target
// is the key form of the import and the names under it: a Go import path
// with // for /, as os//exec//Command, and a Java class under its package,
// as java//lang//Thread//sleep. A target that ends in //* matches every
// external key under it. MaxDepth 0 selects 64 hops.
type Query struct {
	Targets      []string
	IncludeTests bool
	MaxDepth     int
}

// Hop is one step of a path. The first hop is the root and has no site.
// Resolver names the rule of the edge into the hop, or the model and the
// prompt of an llm edge.
type Hop struct {
	Key        string                  `json:"key"`
	Site       projectgraph.Site       `json:"site,omitzero"`
	EdgeKind   projectgraph.EdgeKind   `json:"edge_kind,omitempty"`
	Confidence projectgraph.Confidence `json:"confidence,omitempty"`
	Resolver   string                  `json:"resolver,omitempty"`
}

// CallSite is a call of the target. Reached tells whether the search reached
// the caller.
type CallSite struct {
	Caller  string            `json:"caller"`
	Site    projectgraph.Site `json:"site"`
	Reached bool              `json:"reached"`
}

// Uncertainty is a reason why no path is not a proof. Key is the node that
// the search reached and that holds the reason. Method scopes an unresolved
// call to the targets with that last segment, and to the targets that any
// node with that last segment reaches, in the project or in any unit of
// dependency code. ImportPrefix scopes a computed import to the targets
// under the prefix, and Unit scopes a call into a package at no known
// definition to the targets that the code of the package, and of each
// package that it calls, reaches. With none of them, the uncertainty
// concerns every target.
type Uncertainty struct {
	Kind         UncertaintyKind   `json:"kind"`
	Key          string            `json:"key,omitempty"`
	Site         projectgraph.Site `json:"site"`
	Text         string            `json:"text"`
	Method       string            `json:"method,omitempty"`
	ImportPrefix string            `json:"import_prefix,omitempty"`
	Unit         string            `json:"unit,omitempty"`
}

// Searched is what the search did. Uncertainties counts every uncertainty
// of the result before the cap per kind.
type Searched struct {
	Roots         []projectgraph.Root `json:"roots"`
	Nodes         int                 `json:"nodes"`
	Edges         int                 `json:"edges"`
	Uncertainties int                 `json:"uncertainties"`
	DepthCut      bool                `json:"depth_cut,omitempty"`
	Stopped       bool                `json:"stopped,omitempty"`
}

// Result is the answer for one target. Path is the shortest path from a
// root, with the highest lowest confidence among the paths of that length.
// TestPath is a path from a test root, for a target that only a test
// reaches. SetAside lists the sites that a rule proved cannot reach the
// target, with the rule.
type Result struct {
	Target        string        `json:"target"`
	Status        Status        `json:"status"`
	Path          []Hop         `json:"path,omitempty"`
	TestPath      []Hop         `json:"test_path,omitempty"`
	CallSites     []CallSite    `json:"call_sites"`
	Uncertainties []Uncertainty `json:"uncertainties,omitempty"`
	SetAside      []Uncertainty `json:"set_aside,omitempty"`
	Searched      Searched      `json:"searched"`
}

// String writes the result as lines: the status, the path, the test path,
// the uncertainties and, for a target with no path, the call sites that no
// root reaches and the size of the search.
func (r Result) String() string {
	lines := []string{fmt.Sprintf("%s: %s", r.Target, r.Status)}
	if len(r.Path) > 0 {
		lines = append(lines, "  "+PathString(r.Path))
	}
	if len(r.TestPath) > 0 {
		lines = append(lines, "  test only: "+PathString(r.TestPath))
	}
	for _, u := range r.Uncertainties {
		lines = append(lines, fmt.Sprintf("  %s: %s %s", u.Kind, u.Site, u.Text))
	}
	if r.Status == NoSuchTarget {
		lines = append(lines, "  no node has the key: a target is the key form of the import, as os//exec//Command or java//lang//Thread//sleep")
	}
	if r.Status != Found {
		for _, c := range r.CallSites {
			if !c.Reached {
				lines = append(lines, fmt.Sprintf("  call at %s in %s, which no root reaches", c.Site, c.Caller))
			}
		}
		lines = append(lines, fmt.Sprintf("  searched %d roots, %d nodes, %d edges", len(r.Searched.Roots), r.Searched.Nodes, r.Searched.Edges))
	}
	return strings.Join(lines, "\n")
}

// PathString writes a path as the call site of each hop and then the
// target, as main.py:3 -> handlers.py:12 -> yaml//load. A hop with low
// confidence names its edge kind, as main.py:3 (name), so a reader sees the
// weak hop. An llm hop also names its model and its prompt. A package hop
// names its kind at the definition in the package, so a reader sees where
// the path enters dependency code.
func PathString(hops []Hop) string {
	parts := make([]string, 0, len(hops))
	for _, h := range hops[1:] {
		part := h.Site.String()
		switch {
		case h.EdgeKind == projectgraph.EdgeLLM:
			part += " (" + string(h.EdgeKind) + ", " + h.Resolver + ")"
		case h.Confidence == projectgraph.ConfidenceLow || h.EdgeKind == projectgraph.EdgePackage:
			part += " (" + string(h.EdgeKind) + ")"
		}
		parts = append(parts, part)
	}
	return strings.Join(append(parts, hops[len(hops)-1].Key), " -> ")
}

// Reach returns one result per target, in the order of the query.
func Reach(ctx context.Context, g *projectgraph.Graph, q Query) ([]Result, error) {
	if q.MaxDepth < 0 {
		return nil, fmt.Errorf("max depth %d is negative", q.MaxDepth)
	}
	if slices.Contains(q.Targets, "") {
		return nil, errors.New("an empty target")
	}
	a := newAdjacency(g)
	a.includeTests = q.IncludeTests
	depth := cmp.Or(q.MaxDepth, defaultMaxDepth)

	targets := make([][]int, len(q.Targets))
	for i, t := range q.Targets {
		targets[i] = a.match(t)
	}

	var rootSet, testRoots []projectgraph.Root
	for _, r := range g.Roots {
		if r.Kind == projectgraph.RootTest && !q.IncludeTests {
			testRoots = append(testRoots, r)
		} else {
			rootSet = append(rootSet, r)
		}
	}
	s := a.search(ctx, a.ids(rootSet), depth, targets)
	unc := a.uncertainties(s, depth)

	var tests *search
	results := make([]Result, len(q.Targets))
	for i, t := range q.Targets {
		r := Result{
			Target:    t,
			CallSites: a.callSites(targets[i], s),
			Searched:  Searched{Roots: rootSet, Nodes: len(s.order), Edges: s.edges, DepthCut: s.depthCut, Stopped: s.stopped},
		}
		if node, ok := s.nearest(targets[i]); ok {
			r.Status, r.Path = Found, a.path(s, node)
		} else {
			r.Status = NotFound
			r.Uncertainties, r.SetAside, r.Searched.Uncertainties = unc.forTarget(t)
			switch {
			case len(r.Uncertainties) > 0:
				r.Status = Uncertain
			case len(targets[i]) == 0:
				r.Status = NoSuchTarget
			}
			if len(testRoots) > 0 {
				if tests == nil {
					tests = a.search(ctx, a.ids(g.Roots), depth, targets)
				}
				if node, ok := tests.nearest(targets[i]); ok {
					r.TestPath = a.path(tests, node)
				}
			}
		}
		if g.Stats.RootFallback {
			r.Uncertainties = append(r.Uncertainties, Uncertainty{Kind: RootFallback, Text: "roots: every exported symbol"})
			r.Searched.Uncertainties++
		}
		results[i] = r
	}
	return results, nil
}

// Exploration is what a search from some nodes reaches. Reached holds each
// external node that the search reaches, in key order, with its shortest
// path. Uncertainties holds each uncertainty of the search with no cap, in
// the order of the search: first the ones that concern every target, then
// the ones that a Method, an ImportPrefix or a Unit scopes.
type Exploration struct {
	Reached       []Reached     `json:"reached"`
	Uncertainties []Uncertainty `json:"uncertainties,omitempty"`
}

// Reached is an external node that a search reaches, with the path to it.
type Reached struct {
	Target string `json:"target"`
	Path   []Hop  `json:"path"`
}

// Explore searches the graph from the nodes of the keys in place of its
// roots, as from the exported symbols of a package. MaxDepth 0 selects 64
// hops. A caller with many searches of one graph makes an Explorer once.
func Explore(ctx context.Context, g *projectgraph.Graph, from []string, maxDepth int) (*Exploration, error) {
	return NewExplorer(g).Explore(ctx, from, maxDepth)
}

// Explorer searches one graph from any nodes. It builds the adjacency of
// the graph once, so a summary of a package with many symbols pays it once
// and not once per symbol.
type Explorer struct {
	a *adjacency
}

// NewExplorer builds the adjacency of the graph for the searches of an Explorer.
func NewExplorer(g *projectgraph.Graph) *Explorer {
	return &Explorer{a: newAdjacency(g)}
}

// Explore searches from the nodes of the keys, as Explore does.
func (x *Explorer) Explore(ctx context.Context, from []string, maxDepth int) (*Exploration, error) {
	if maxDepth < 0 {
		return nil, fmt.Errorf("max depth %d is negative", maxDepth)
	}
	a := x.a
	sources := make([]int, 0, len(from))
	for _, key := range from {
		id, ok := a.id[key]
		if !ok {
			return nil, fmt.Errorf("no node %s", key)
		}
		sources = append(sources, id)
	}
	slices.Sort(sources)
	depth := cmp.Or(maxDepth, defaultMaxDepth)
	s := a.search(ctx, slices.Compact(sources), depth, nil)

	e := &Exploration{Reached: []Reached{}}
	for _, id := range a.externals {
		if s.dist[id] >= 0 && !slices.Contains(sources, id) {
			e.Reached = append(e.Reached, Reached{Target: a.keys[id], Path: a.path(s, id)})
		}
	}
	f := a.uncertainties(s, depth)
	e.Uncertainties = slices.Clone(f.global)
	for _, m := range f.methods {
		e.Uncertainties = append(e.Uncertainties, f.byMethod[m]...)
	}
	for _, p := range f.prefixed {
		e.Uncertainties = append(e.Uncertainties, p.u)
	}
	e.Uncertainties = append(e.Uncertainties, f.byUnit...)
	return e, nil
}

// adjacency is the graph with integer node ids, in key order, and the edges out
// of each node. unitNodes holds the nodes of each unit of dependency code,
// and closures what the code of each unit reaches. byLast holds the nodes
// by the last segment of their key, and methodClosures what the nodes of
// each last segment reach. Each table fills on first use. includeTests
// lets a node of a test file be a source of a method closure.
type adjacency struct {
	g         *projectgraph.Graph
	keys      []string
	id        map[string]int
	out       [][]int32
	in        [][]int32
	externals []int

	includeTests bool

	unitNodes      map[string][]int
	closures       map[string][]bool
	byLast         map[string][]int
	methodClosures map[string][]bool
}

func newAdjacency(g *projectgraph.Graph) *adjacency {
	keys := make([]string, 0, len(g.Nodes))
	for k := range g.Nodes {
		keys = append(keys, k)
	}
	for _, e := range g.Edges {
		for _, k := range []string{e.From, e.To} {
			if _, ok := g.Nodes[k]; !ok {
				keys = append(keys, k)
			}
		}
	}
	slices.Sort(keys)
	keys = slices.Compact(keys)

	a := &adjacency{g: g, keys: keys, id: make(map[string]int, len(keys)), out: make([][]int32, len(keys)), in: make([][]int32, len(keys))}
	for i, k := range keys {
		a.id[k] = i
		if n := g.Nodes[k]; n != nil && n.Kind == projectgraph.NodeExternal {
			a.externals = append(a.externals, i)
		}
	}
	for i, e := range g.Edges {
		from, to := a.id[e.From], a.id[e.To]
		a.out[from] = append(a.out[from], int32(i))
		a.in[to] = append(a.in[to], int32(i))
	}
	return a
}

// unitOf returns the unit of dependency code of a node, or "".
func (a *adjacency) unitOf(id int) string {
	if n := a.g.Nodes[a.keys[id]]; n != nil {
		return fileindex.PackageOf(n.Path)
	}
	return ""
}

// closure returns the nodes that the code of a unit reaches, and the code
// of each unit that a node of it reaches. A value of the project that the
// code of a unit calls comes to it on an edge, as a reference, so the
// search from the project reaches that value already.
func (a *adjacency) closure(unit string) []bool {
	if c, ok := a.closures[unit]; ok {
		return c
	}
	if a.unitNodes == nil {
		a.unitNodes, a.closures = map[string][]int{}, map[string][]bool{}
		for id := range a.keys {
			if u := a.unitOf(id); u != "" {
				a.unitNodes[u] = append(a.unitNodes[u], id)
			}
		}
	}
	entered := map[string]bool{}
	seen := a.reachable(a.unitNodes[unit], func(id int) []int {
		u := a.unitOf(id)
		if u == "" || entered[u] {
			return nil
		}
		entered[u] = true
		return a.unitNodes[u]
	})
	a.closures[unit] = seen
	return seen
}

// reachable returns the nodes that a search from the sources reaches. The
// search also takes the nodes that more gives for each node that it
// reaches, as the other nodes of the unit of the node.
func (a *adjacency) reachable(sources []int, more func(id int) []int) []bool {
	seen := make([]bool, len(a.keys))
	var queue []int
	add := func(id int) {
		if !seen[id] {
			seen[id] = true
			queue = append(queue, id)
		}
	}
	for _, id := range sources {
		add(id)
	}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, v := range more(id) {
			add(v)
		}
		for _, e := range a.out[id] {
			add(a.id[a.g.Edges[e].To])
		}
	}
	return seen
}

// unitReaches reports whether the code of a unit reaches a node of the
// target.
func (a *adjacency) unitReaches(unit, target string) bool {
	c := a.closure(unit)
	return slices.ContainsFunc(a.match(target), func(id int) bool { return c[id] })
}

// methodClosure returns the nodes that the nodes with the last segment
// reach: the definitions of the name in the project and in each unit of
// dependency code, and the external keys of the name, which can lead into
// a unit. An unresolved call of the name can run any of them. A node of a
// test file is a source only when the query includes the tests.
func (a *adjacency) methodClosure(name string) []bool {
	if c, ok := a.methodClosures[name]; ok {
		return c
	}
	if a.byLast == nil {
		a.byLast, a.methodClosures = map[string][]int{}, map[string][]bool{}
		for id, key := range a.keys {
			if n := a.g.Nodes[key]; n != nil && (a.includeTests || !n.Test) {
				a.byLast[fileindex.Last(key)] = append(a.byLast[fileindex.Last(key)], id)
			}
		}
	}
	seen := a.reachable(a.byLast[name], func(int) []int { return nil })
	a.methodClosures[name] = seen
	return seen
}

// methodReaches reports whether a node with the last segment reaches a
// node of the target.
func (a *adjacency) methodReaches(name, target string) bool {
	c := a.methodClosure(name)
	return slices.ContainsFunc(a.match(target), func(id int) bool { return c[id] })
}

// match returns the external nodes of a target, by id.
func (a *adjacency) match(target string) []int {
	prefix, ok := strings.CutSuffix(target, "*")
	if ok && strings.HasSuffix(prefix, fileindex.Separator) {
		var ids []int
		for _, id := range a.externals {
			if strings.HasPrefix(a.keys[id], prefix) {
				ids = append(ids, id)
			}
		}
		return ids
	}
	if n := a.g.Nodes[target]; n != nil && n.Kind == projectgraph.NodeExternal {
		return []int{a.id[target]}
	}
	return nil
}

func (a *adjacency) ids(roots []projectgraph.Root) []int {
	ids := make([]int, 0, len(roots))
	for _, r := range roots {
		if id, ok := a.id[r.Key]; ok {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return slices.Compact(ids)
}

// search is the state of one breadth-first search. best is the highest
// lowest confidence rank of a shortest path to each node, and parent is the
// edge into the node on that path, or -1 at a root.
type search struct {
	dist     []int32
	best     []int8
	parent   []int32
	order    []int
	edges    int
	depthCut bool
	stopped  bool
}

func (a *adjacency) search(ctx context.Context, sources []int, maxDepth int, targets [][]int) *search {
	n := len(a.keys)
	s := &search{dist: make([]int32, n), best: make([]int8, n), parent: make([]int32, n)}
	for i := range s.dist {
		s.dist[i], s.parent[i] = -1, -1
	}
	frontier := make([]int, 0, len(sources))
	for _, id := range sources {
		s.dist[id], s.best[id] = 0, int8(projectgraph.ConfidenceHigh.Rank())
		frontier = append(frontier, id)
	}
	s.order = append(s.order, frontier...)

	for depth := 0; len(frontier) > 0; depth++ {
		if ctx.Err() != nil {
			s.stopped = true
			return s
		}
		if len(targets) > 0 && s.allFound(targets) {
			return s
		}
		if depth == maxDepth {
			s.depthCut = slices.ContainsFunc(frontier, func(u int) bool {
				return slices.ContainsFunc(a.out[u], func(e int32) bool { return s.dist[a.id[a.g.Edges[e].To]] < 0 })
			})
			return s
		}
		var next []int
		for _, u := range frontier {
			for _, e := range a.out[u] {
				s.edges++
				edge := &a.g.Edges[e]
				v := a.id[edge.To]
				rank := min(s.best[u], int8(edge.Confidence.Rank()))
				switch {
				case s.dist[v] < 0:
					s.dist[v], s.best[v], s.parent[v] = int32(depth+1), rank, e
					next = append(next, v)
				case s.dist[v] == int32(depth+1) && rank > s.best[v]:
					s.best[v], s.parent[v] = rank, e
				}
			}
		}
		s.order = append(s.order, next...)
		frontier = next
	}
	return s
}

func (s *search) allFound(targets [][]int) bool {
	for _, ids := range targets {
		if !slices.ContainsFunc(ids, func(id int) bool { return s.dist[id] >= 0 }) {
			return false
		}
	}
	return true
}

// nearest returns the reached node of a target with the shortest path, then
// the highest confidence, then the first key.
func (s *search) nearest(ids []int) (int, bool) {
	best, found := 0, false
	for _, id := range ids {
		if s.dist[id] < 0 {
			continue
		}
		if !found || s.dist[id] < s.dist[best] || s.dist[id] == s.dist[best] && s.best[id] > s.best[best] {
			best, found = id, true
		}
	}
	return best, found
}

func (a *adjacency) path(s *search, node int) []Hop {
	var hops []Hop
	for s.parent[node] >= 0 {
		e := a.g.Edges[s.parent[node]]
		hops = append(hops, Hop{Key: e.To, Site: e.Site, EdgeKind: e.Kind, Confidence: e.Confidence, Resolver: e.Resolver})
		node = a.id[e.From]
	}
	hops = append(hops, Hop{Key: a.keys[node]})
	slices.Reverse(hops)
	return hops
}

func (a *adjacency) callSites(ids []int, s *search) []CallSite {
	sites := []CallSite{}
	for _, id := range ids {
		for _, i := range a.in[id] {
			e := &a.g.Edges[i]
			sites = append(sites, CallSite{Caller: e.From, Site: e.Site, Reached: s.dist[a.id[e.From]] >= 0})
		}
	}
	slices.SortFunc(sites, func(a, b CallSite) int {
		return cmp.Or(
			cmp.Compare(a.Site.Path, b.Site.Path), cmp.Compare(a.Site.Line, b.Site.Line),
			cmp.Compare(a.Site.Column, b.Site.Column), cmp.Compare(a.Caller, b.Caller),
		)
	})
	return slices.CompactFunc(sites, func(a, b CallSite) bool { return a == b })
}
