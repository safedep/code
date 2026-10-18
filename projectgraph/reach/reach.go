// Package reach answers which targets a project graph reaches from its
// roots. One breadth-first search serves every target of a query.
package reach

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
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

// Reach returns one result per target, in the order of the query.
func Reach(ctx context.Context, g *projectgraph.Graph, q Query) ([]Result, error) {
	if q.MaxDepth < 0 {
		return nil, fmt.Errorf("max depth %d is negative", q.MaxDepth)
	}
	if slices.Contains(q.Targets, "") {
		return nil, errors.New("an empty target")
	}
	ix := newIndex(g)
	ix.includeTests = q.IncludeTests
	depth := cmp.Or(q.MaxDepth, defaultMaxDepth)

	targets := make([][]int, len(q.Targets))
	for i, t := range q.Targets {
		targets[i] = ix.match(t)
	}

	var rootSet, testRoots []projectgraph.Root
	for _, r := range g.Roots {
		if r.Kind == projectgraph.RootTest && !q.IncludeTests {
			testRoots = append(testRoots, r)
		} else {
			rootSet = append(rootSet, r)
		}
	}
	s := ix.search(ctx, ix.ids(rootSet), depth, targets)
	unc := ix.uncertainties(s, depth)

	var tests *search
	results := make([]Result, len(q.Targets))
	for i, t := range q.Targets {
		r := Result{
			Target:    t,
			CallSites: ix.callSites(targets[i], s),
			Searched:  Searched{Roots: rootSet, Nodes: len(s.order), Edges: s.edges, DepthCut: s.depthCut, Stopped: s.stopped},
		}
		if node, ok := s.nearest(targets[i]); ok {
			r.Status, r.Path = Found, ix.path(s, node)
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
					tests = ix.search(ctx, ix.ids(g.Roots), depth, targets)
				}
				if node, ok := tests.nearest(targets[i]); ok {
					r.TestPath = ix.path(tests, node)
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

// Explorer searches one graph from any nodes. It builds the index of the
// graph once, so a summary of a package with many symbols pays the index
// once and not once per symbol.
type Explorer struct {
	ix *index
}

// NewExplorer indexes the graph for the searches of an Explorer.
func NewExplorer(g *projectgraph.Graph) *Explorer {
	return &Explorer{ix: newIndex(g)}
}

// Explore searches from the nodes of the keys, as Explore does.
func (x *Explorer) Explore(ctx context.Context, from []string, maxDepth int) (*Exploration, error) {
	if maxDepth < 0 {
		return nil, fmt.Errorf("max depth %d is negative", maxDepth)
	}
	ix := x.ix
	sources := make([]int, 0, len(from))
	for _, key := range from {
		id, ok := ix.id[key]
		if !ok {
			return nil, fmt.Errorf("no node %s", key)
		}
		sources = append(sources, id)
	}
	slices.Sort(sources)
	depth := cmp.Or(maxDepth, defaultMaxDepth)
	s := ix.search(ctx, slices.Compact(sources), depth, nil)

	e := &Exploration{Reached: []Reached{}}
	for _, id := range ix.externals {
		if s.dist[id] >= 0 && !slices.Contains(sources, id) {
			e.Reached = append(e.Reached, Reached{Target: ix.keys[id], Path: ix.path(s, id)})
		}
	}
	f := ix.uncertainties(s, depth)
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

// index is the graph with integer node ids, in key order, and the edges out
// of each node. unitNodes holds the nodes of each unit of dependency code,
// and closures what the code of each unit reaches. byLast holds the nodes
// by the last segment of their key, and methodClosures what the nodes of
// each last segment reach. Each table fills on first use. includeTests
// lets a node of a test file be a source of a method closure.
type index struct {
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

func newIndex(g *projectgraph.Graph) *index {
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

	ix := &index{g: g, keys: keys, id: make(map[string]int, len(keys)), out: make([][]int32, len(keys)), in: make([][]int32, len(keys))}
	for i, k := range keys {
		ix.id[k] = i
		if n := g.Nodes[k]; n != nil && n.Kind == projectgraph.NodeExternal {
			ix.externals = append(ix.externals, i)
		}
	}
	for i, e := range g.Edges {
		from, to := ix.id[e.From], ix.id[e.To]
		ix.out[from] = append(ix.out[from], int32(i))
		ix.in[to] = append(ix.in[to], int32(i))
	}
	return ix
}

// unitOf returns the unit of dependency code of a node, or "".
func (ix *index) unitOf(id int) string {
	if n := ix.g.Nodes[ix.keys[id]]; n != nil {
		return fileindex.PackageOf(n.Path)
	}
	return ""
}

// closure returns the nodes that the code of a unit reaches, and the code
// of each unit that a node of it reaches. A value of the project that the
// code of a unit calls comes to it on an edge, as a reference, so the
// search from the project reaches that value already.
func (ix *index) closure(unit string) []bool {
	if c, ok := ix.closures[unit]; ok {
		return c
	}
	if ix.unitNodes == nil {
		ix.unitNodes, ix.closures = map[string][]int{}, map[string][]bool{}
		for id := range ix.keys {
			if u := ix.unitOf(id); u != "" {
				ix.unitNodes[u] = append(ix.unitNodes[u], id)
			}
		}
	}
	entered := map[string]bool{}
	seen := ix.reachable(ix.unitNodes[unit], func(id int) []int {
		u := ix.unitOf(id)
		if u == "" || entered[u] {
			return nil
		}
		entered[u] = true
		return ix.unitNodes[u]
	})
	ix.closures[unit] = seen
	return seen
}

// reachable returns the nodes that a search from the sources reaches. The
// search also takes the nodes that more gives for each node that it
// reaches, as the other nodes of the unit of the node.
func (ix *index) reachable(sources []int, more func(id int) []int) []bool {
	seen := make([]bool, len(ix.keys))
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
		for _, e := range ix.out[id] {
			add(ix.id[ix.g.Edges[e].To])
		}
	}
	return seen
}

// unitReaches reports whether the code of a unit reaches a node of the
// target.
func (ix *index) unitReaches(unit, target string) bool {
	c := ix.closure(unit)
	return slices.ContainsFunc(ix.match(target), func(id int) bool { return c[id] })
}

// methodClosure returns the nodes that the nodes with the last segment
// reach: the definitions of the name in the project and in each unit of
// dependency code, and the external keys of the name, which can lead into
// a unit. An unresolved call of the name can run any of them. A node of a
// test file is a source only when the query includes the tests.
func (ix *index) methodClosure(name string) []bool {
	if c, ok := ix.methodClosures[name]; ok {
		return c
	}
	if ix.byLast == nil {
		ix.byLast, ix.methodClosures = map[string][]int{}, map[string][]bool{}
		for id, key := range ix.keys {
			if n := ix.g.Nodes[key]; n != nil && (ix.includeTests || !n.Test) {
				ix.byLast[fileindex.Last(key)] = append(ix.byLast[fileindex.Last(key)], id)
			}
		}
	}
	seen := ix.reachable(ix.byLast[name], func(int) []int { return nil })
	ix.methodClosures[name] = seen
	return seen
}

// methodReaches reports whether a node with the last segment reaches a
// node of the target.
func (ix *index) methodReaches(name, target string) bool {
	c := ix.methodClosure(name)
	return slices.ContainsFunc(ix.match(target), func(id int) bool { return c[id] })
}

// match returns the external nodes of a target, by id.
func (ix *index) match(target string) []int {
	prefix, ok := strings.CutSuffix(target, "*")
	if ok && strings.HasSuffix(prefix, fileindex.Separator) {
		var ids []int
		for _, id := range ix.externals {
			if strings.HasPrefix(ix.keys[id], prefix) {
				ids = append(ids, id)
			}
		}
		return ids
	}
	if n := ix.g.Nodes[target]; n != nil && n.Kind == projectgraph.NodeExternal {
		return []int{ix.id[target]}
	}
	return nil
}

func (ix *index) ids(roots []projectgraph.Root) []int {
	ids := make([]int, 0, len(roots))
	for _, r := range roots {
		if id, ok := ix.id[r.Key]; ok {
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

func (ix *index) search(ctx context.Context, sources []int, maxDepth int, targets [][]int) *search {
	n := len(ix.keys)
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
				return slices.ContainsFunc(ix.out[u], func(e int32) bool { return s.dist[ix.id[ix.g.Edges[e].To]] < 0 })
			})
			return s
		}
		var next []int
		for _, u := range frontier {
			for _, e := range ix.out[u] {
				s.edges++
				edge := &ix.g.Edges[e]
				v := ix.id[edge.To]
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

func (ix *index) path(s *search, node int) []Hop {
	var hops []Hop
	for s.parent[node] >= 0 {
		e := ix.g.Edges[s.parent[node]]
		hops = append(hops, Hop{Key: e.To, Site: e.Site, EdgeKind: e.Kind, Confidence: e.Confidence, Resolver: e.Resolver})
		node = ix.id[e.From]
	}
	hops = append(hops, Hop{Key: ix.keys[node]})
	slices.Reverse(hops)
	return hops
}

func (ix *index) callSites(ids []int, s *search) []CallSite {
	sites := []CallSite{}
	for _, id := range ids {
		for _, i := range ix.in[id] {
			e := &ix.g.Edges[i]
			sites = append(sites, CallSite{Caller: e.From, Site: e.Site, Reached: s.dist[ix.id[e.From]] >= 0})
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

// prefixed is an uncertainty that a computed import makes, and the key
// prefix that its static string gives.
type prefixed struct {
	u      Uncertainty
	prefix string
}

// found collects the uncertainties of the nodes that a search reached, in
// the order of the search. A record with a method name concerns only the
// targets with that last segment and the targets that a node with that
// last segment reaches, unless its reason can name any target.
type found struct {
	global   []Uncertainty
	byMethod map[string][]Uncertainty
	methods  []string
	prefixed []prefixed
	byUnit   []Uncertainty
	// reaches reports whether the code of a unit reaches a target, and
	// methodReaches whether a node with the last segment reaches one.
	reaches       func(unit, target string) bool
	methodReaches func(name, target string) bool
	// units holds the file budgets of each unit of dependency code, and
	// entered the units that the search entered.
	units   map[string][]Uncertainty
	entered map[string]bool
}

func (ix *index) uncertainties(s *search, maxDepth int) *found {
	g := ix.g
	f := &found{byMethod: map[string][]Uncertainty{}, reaches: ix.unitReaches, methodReaches: ix.methodReaches, units: map[string][]Uncertainty{}, entered: map[string]bool{}}

	unresolved := map[string][]projectgraph.UnresolvedSite{}
	prefixes := map[projectgraph.Site]string{}
	for _, u := range g.Unresolved {
		unresolved[u.Caller] = append(unresolved[u.Caller], u)
		if u.ImportPrefix != "" {
			prefixes[projectgraph.Site{Path: u.Site.Path, Line: u.Site.Line}] = u.ImportPrefix
		}
	}
	truncated := map[string][]projectgraph.Truncation{}
	for _, t := range g.Truncated {
		if t.Key == "" {
			f.global = append(f.global, Uncertainty{Kind: Budget, Site: t.Site, Text: "the link step hit its " + string(t.Kind) + " limit"})
		} else {
			truncated[t.Key] = append(truncated[t.Key], t)
		}
	}
	f.addFileBudgets(g.Files)
	// A record with no caller is a unit that no import names. Any search
	// can enter it, so it counts for every search.
	for _, r := range unresolved[""] {
		f.enter(r.Unit)
		f.byUnit = append(f.byUnit, Uncertainty{Kind: UnresolvedCall, Site: r.Site, Text: fmt.Sprintf("%s (%s)", r.Text, r.Reason), Unit: r.Unit})
	}
	for _, id := range s.order {
		key := ix.keys[id]
		n := g.Nodes[key]
		if n == nil {
			continue
		}
		f.enter(fileindex.PackageOf(n.Path))
		for _, t := range truncated[key] {
			site := t.Site
			site.Path = cmp.Or(site.Path, n.Path)
			f.global = append(f.global, Uncertainty{Kind: Budget, Key: key, Site: site, Text: "the link step hit its " + string(t.Kind) + " limit"})
		}
		f.addFlags(key, n, prefixes)
		f.addRecords(key, unresolved[key])
	}
	if s.depthCut {
		f.global = append(f.global, Uncertainty{Kind: Budget, Text: fmt.Sprintf("the search stopped at depth %d", maxDepth)})
	}
	if s.stopped {
		f.global = append(f.global, Uncertainty{Kind: Budget, Text: "the search stopped at the deadline"})
	}
	return f
}

// addFileBudgets adds a budget uncertainty for each file with no complete
// index. A file of the project can hold a root or a caller that the graph
// does not know, so it counts where the search does not reach it. A file
// of dependency code holds no root, and its code runs only when a call
// enters its package. So it counts only for a search that reaches a node
// of its unit.
func (f *found) addFileBudgets(files map[string]fileindex.FileStatus) {
	for _, p := range slices.Sorted(maps.Keys(files)) {
		status := files[p]
		if status.Complete() {
			continue
		}
		u := Uncertainty{Kind: Budget, Key: p, Site: projectgraph.Site{Path: p}, Text: "the file is " + string(status)}
		if unit := fileindex.PackageOf(p); unit != "" {
			f.units[unit] = append(f.units[unit], u)
		} else {
			f.global = append(f.global, u)
		}
	}
}

// enter adds the file budgets of a unit the first time the search enters
// it.
func (f *found) enter(unit string) {
	if unit != "" && !f.entered[unit] {
		f.entered[unit] = true
		f.global = append(f.global, f.units[unit]...)
	}
}

// addFlags adds the dynamic flags of a node. A parse error is a budget. A
// computed import with a static prefix concerns only the targets that the
// prefix can name.
func (f *found) addFlags(key string, n *projectgraph.Node, prefixes map[projectgraph.Site]string) {
	for _, fl := range n.Flags {
		site := projectgraph.Site{Path: n.Path, Line: fl.Line}
		u := Uncertainty{Kind: DynamicFlag, Key: key, Site: site, Text: string(fl.Kind)}
		switch {
		case fl.Kind == fileindex.FlagInjection, fl.Kind == fileindex.FlagModuleGetattr:
			// The declared type of an injected value narrows the calls
			// on it, and an external type of it leaves the call
			// unresolved. The link step resolves a name that a module
			// with __getattr__ does not define to __getattr__ with a
			// computed-name record. So neither flag adds an uncertainty
			// of its own.
			continue
		case fl.Kind == fileindex.FlagParseError:
			u.Kind = Budget
			f.global = append(f.global, u)
		case fl.Kind == fileindex.FlagComputedImport && prefixes[site] != "":
			u.ImportPrefix = prefixes[site]
			f.prefixed = append(f.prefixed, prefixed{u: u, prefix: prefixes[site]})
		default:
			f.global = append(f.global, u)
		}
	}
}

// addRecords adds the unresolved sites of a node: a site of a unit counts
// for the targets that the unit reaches, a computed import for the targets
// that its prefix can name, and a site with a method name for the targets
// of that name.
func (f *found) addRecords(key string, records []projectgraph.UnresolvedSite) {
	for _, r := range records {
		f.enter(fileindex.PackageOf(r.Site.Path))
		f.enter(r.Unit)
		u := Uncertainty{Kind: UnresolvedCall, Key: key, Site: r.Site, Text: fmt.Sprintf("%s (%s)", r.Text, r.Reason)}
		switch {
		case r.Unit != "":
			u.Unit = r.Unit
			f.byUnit = append(f.byUnit, u)
		case r.ImportPrefix != "":
			u.ImportPrefix = r.ImportPrefix
			f.prefixed = append(f.prefixed, prefixed{u: u, prefix: r.ImportPrefix})
		case r.Method != "" && !projectgraph.ReachesAny(r.Reason):
			u.Method = r.Method
			if _, ok := f.byMethod[r.Method]; !ok {
				f.methods = append(f.methods, r.Method)
			}
			f.byMethod[r.Method] = append(f.byMethod[r.Method], u)
		default:
			f.global = append(f.global, u)
		}
	}
}

// forTarget returns the uncertainties of a target, at most maxPerKind of
// each kind, the sites that a computed import prefix sets aside, and the
// count before the cap.
func (f *found) forTarget(target string) ([]Uncertainty, []Uncertainty, int) {
	all := slices.Clone(f.global)
	prefix, isWildcard := strings.CutSuffix(target, wildcard)
	for _, m := range f.methods {
		if isWildcard || m == fileindex.Last(target) || f.methodReaches(m, target) {
			all = append(all, f.byMethod[m]...)
		}
	}

	for _, u := range f.byUnit {
		if f.reaches(u.Unit, target) {
			all = append(all, u)
		}
	}

	var setAside []Uncertainty
	for _, p := range f.prefixed {
		if mayName(p.prefix, target, prefix, isWildcard) {
			all = append(all, p.u)
			continue
		}
		u := p.u
		u.Text = fmt.Sprintf("%s: the import prefix %q names another module", u.Text, p.prefix)
		setAside = append(setAside, u)
	}

	count := map[UncertaintyKind]int{}
	var capped []Uncertainty
	for _, u := range all {
		if count[u.Kind] < maxPerKind {
			capped = append(capped, u)
		}
		count[u.Kind]++
	}
	return capped, setAside, len(all)
}

// mayName reports whether a computed import with a static prefix can load
// the module of a target.
func mayName(importPrefix, target, targetPrefix string, isWildcard bool) bool {
	if strings.HasPrefix(target, importPrefix) {
		return true
	}
	return isWildcard && strings.HasPrefix(importPrefix, targetPrefix)
}
