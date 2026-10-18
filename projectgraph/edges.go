package projectgraph

import (
	"cmp"
	"slices"
)

// symbol is the id of a string that the link step interned. The edge set
// keys an edge by the ids of its strings, so a project of 400,000 edges
// keeps one copy of each key, path and resolver name, and its map keys are
// 24 bytes and not 64.
type symbol uint32

// symbols interns the strings of the edges.
type symbols struct {
	ids     map[string]symbol
	strings []string
}

func (s *symbols) intern(v string) symbol {
	if id, ok := s.ids[v]; ok {
		return id
	}
	id := symbol(len(s.strings))
	s.ids[v] = id
	s.strings = append(s.strings, v)
	return id
}

// ranks returns the place of each symbol in the sorted order of the
// strings, so a sort of the edges compares integers and not strings.
func (s *symbols) ranks() []uint32 {
	order := make([]symbol, len(s.strings))
	for i := range order {
		order[i] = symbol(i)
	}
	slices.SortFunc(order, func(a, b symbol) int { return cmp.Compare(s.strings[a], s.strings[b]) })
	ranks := make([]uint32, len(order))
	for rank, id := range order {
		ranks[id] = uint32(rank)
	}
	return ranks
}

// edgeSlot is the caller, the callee and the site of an edge. The graph
// keeps one edge per slot.
type edgeSlot struct {
	from, to, path symbol
	line, column   int32
}

// edgeValue is the kind and the resolver of the edge of a slot.
type edgeValue struct {
	kind, resolver symbol
}

// edgeSet holds the edges of the graph and the pairs of caller and callee
// that an edge joins.
type edgeSet struct {
	symbols symbols
	slots   map[edgeSlot]edgeValue
	linked  map[[2]symbol]struct{}
}

// newEdgeSet returns a set with room for about sites edges. The growth of
// a map to 400,000 entries costs about 7% of the link step.
func newEdgeSet(sites int) *edgeSet {
	return &edgeSet{
		symbols: symbols{ids: make(map[string]symbol, sites)},
		slots:   make(map[edgeSlot]edgeValue, sites),
		linked:  make(map[[2]symbol]struct{}, sites),
	}
}

func (s *edgeSet) len() int {
	return len(s.slots)
}

func (s *edgeSet) slot(e Edge) edgeSlot {
	return edgeSlot{
		from:   s.symbols.intern(e.From),
		to:     s.symbols.intern(e.To),
		path:   s.symbols.intern(e.Site.Path),
		line:   int32(e.Site.Line),
		column: int32(e.Site.Column),
	}
}

// kind returns the kind of the edge of the slot, if the slot has one.
func (s *edgeSet) kind(slot edgeSlot) (EdgeKind, bool) {
	v, ok := s.slots[slot]
	if !ok {
		return "", false
	}
	return EdgeKind(s.symbols.strings[v.kind]), true
}

func (s *edgeSet) put(slot edgeSlot, e Edge) {
	s.slots[slot] = edgeValue{kind: s.symbols.intern(string(e.Kind)), resolver: s.symbols.intern(e.Resolver)}
	s.linked[[2]symbol{slot.from, slot.to}] = struct{}{}
}

// holds reports whether the set has an edge at the slot of e.
func (s *edgeSet) holds(e Edge) bool {
	_, ok := s.slots[s.slot(e)]
	return ok
}

// add puts the edge in the set unless the caller already has an edge to
// the callee at the site with a confidence at least as high. A higher one
// replaces it, so a path through the site shows the best kind. It reports
// whether the set changed.
func (s *edgeSet) add(e Edge) bool {
	slot := s.slot(e)
	if old, ok := s.kind(slot); ok && !e.Kind.outranks(old) {
		return false
	}
	s.put(slot, e)
	return true
}

// newEdge returns an edge of the kind with the confidence of the kind.
func newEdge(from, to string, site Site, kind EdgeKind, resolver string) Edge {
	return Edge{From: from, To: to, Site: site, Kind: kind, Confidence: kind.Confidence(), Resolver: resolver}
}

// joins reports whether an edge goes from the caller to the callee.
func (s *edgeSet) joins(from, to string) bool {
	f, ok := s.symbols.ids[from]
	if !ok {
		return false
	}
	t, ok := s.symbols.ids[to]
	if !ok {
		return false
	}
	_, ok = s.linked[[2]symbol{f, t}]
	return ok
}

// sorted returns the edges by caller, site, callee, kind and resolver.
func (s *edgeSet) sorted() []Edge {
	type entry struct {
		edgeSlot
		edgeValue
	}
	entries := make([]entry, 0, len(s.slots))
	for slot, v := range s.slots {
		entries = append(entries, entry{slot, v})
	}
	rank := s.symbols.ranks()
	slices.SortFunc(entries, func(a, b entry) int {
		return cmp.Or(
			cmp.Compare(rank[a.from], rank[b.from]),
			cmp.Compare(rank[a.path], rank[b.path]),
			cmp.Compare(a.line, b.line),
			cmp.Compare(a.column, b.column),
			cmp.Compare(rank[a.to], rank[b.to]),
			cmp.Compare(rank[a.kind], rank[b.kind]),
			cmp.Compare(rank[a.resolver], rank[b.resolver]),
		)
	})

	str := s.symbols.strings
	edges := make([]Edge, len(entries))
	for i, e := range entries {
		site := Site{Path: str[e.path], Line: int(e.line), Column: int(e.column)}
		edges[i] = newEdge(str[e.from], str[e.to], site, EdgeKind(str[e.kind]), str[e.resolver])
	}
	return edges
}
