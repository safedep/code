package reach

import (
	"cmp"
	"context"
	"fmt"
	"math/bits"
	"slices"
)

// Scoped is an uncertainty with the targets of the graph that its scope
// concerns. Targets is nil for an uncertainty that concerns every target.
// A Method scope also concerns every target whose last segment is the
// method, and an ImportPrefix scope every target under the prefix, whether
// the graph holds the target or not.
type Scoped struct {
	Uncertainty
	Targets []string `json:"targets,omitempty"`
}

// Concerns reports whether the uncertainty concerns a target, as the
// result of a reach query lists it: a Unit scope concerns its Targets, an
// ImportPrefix scope every target under the prefix, a Method scope its
// Targets and every target whose last segment is the method, and an
// uncertainty with no scope every target.
func (u Scoped) Concerns(t Target) bool {
	switch {
	case u.Unit != "":
		return t.MatchesSorted(u.Targets)
	case u.ImportPrefix != "":
		return t.mayImport(u.ImportPrefix)
	case u.Method != "":
		return t.hasMethod(u.Method) || t.MatchesSorted(u.Targets)
	}
	return true
}

// Reachable is what one source reaches: the external nodes, in key order,
// and the indexes into the table of uncertainties of the Sets, ascending.
type Reachable struct {
	Reached       []string `json:"reached"`
	Uncertainties []int    `json:"uncertainties,omitempty"`
}

// Sets is what each of several sources reaches, with no path and no depth
// cap. Sources holds one Reachable per source, in the order of the
// sources. Uncertainties is the table that the sources index: first the
// ones that count for every search, then the file budgets of each unit,
// then the ones of each node in key order.
type Sets struct {
	Sources       []Reachable `json:"sources"`
	Uncertainties []Scoped    `json:"uncertainties,omitempty"`
}

// ExploreAll searches from each of the keys, as Explore does, with one
// walk over the strongly connected components of the graph in place of
// one search per key. The components come in reverse topological order,
// so each one unions the sets of its successors once. Two sources that
// reach the same set share its storage. When ctx ends during the walk,
// the sets of the components that the walk did not reach are empty, and
// every source gets a budget uncertainty that says the search stopped,
// as Explore reports it.
func (x *Explorer) ExploreAll(ctx context.Context, from []string) (*Sets, error) {
	a := x.a
	ids := make([]int, len(from))
	for i, key := range from {
		id, ok := a.id[key]
		if !ok {
			return nil, fmt.Errorf("no node %s", key)
		}
		ids[i] = id
	}
	c := a.condense()
	t := a.table()
	sets, stopped := c.sets(ctx, a, t)
	if stopped {
		t.global = append(t.global, len(t.entries))
		t.entries = append(t.entries, Scoped{Uncertainty: Uncertainty{Kind: Budget, Text: "the search stopped at the deadline"}})
	}

	out := &Sets{Sources: make([]Reachable, len(from)), Uncertainties: t.entries}
	shared := map[string]Reachable{}
	for i, id := range ids {
		set := sets[c.comp[id]]
		// A source in a cycle is in its own set, and materialize drops
		// it, so the sources of one cycle do not share one slice.
		if item, ok := t.target[id]; ok && set.has(item) {
			out.Sources[i] = t.materialize(a, set, id)
			continue
		}
		key := string(set.bytes())
		r, ok := shared[key]
		if !ok {
			r = t.materialize(a, set, id)
			shared[key] = r
		}
		out.Sources[i] = r
	}
	return out, nil
}

// table is the item space of the sets: the external nodes first, then the
// units a search can enter, then the nodes with an uncertainty. target
// holds the item of each external node, and unc the item of each node
// with an uncertainty. entries is the table of uncertainties, and at holds
// the index range of each item of a unit or a node in it.
type table struct {
	items   int
	targets []int
	target  map[int]int
	unc     map[int]int
	units   map[string]int
	entries []Scoped
	at      map[int][2]int
	// global holds the indexes of the entries that count for every
	// search.
	global []int
}

func (a *adjacency) table() *table {
	src := a.sources()
	t := &table{target: map[int]int{}, unc: map[int]int{}, units: map[string]int{}, at: map[int][2]int{}}
	for _, id := range a.externals {
		t.target[id] = t.items
		t.targets = append(t.targets, id)
		t.items++
	}
	for _, u := range src.global {
		t.global = append(t.global, len(t.entries))
		t.entries = append(t.entries, a.scoped(u))
	}
	for _, unit := range src.units {
		t.units[unit] = t.items
		start := len(t.entries)
		for _, u := range src.budgets[unit] {
			t.entries = append(t.entries, a.scoped(u))
		}
		t.at[t.items] = [2]int{start, len(t.entries)}
		t.items++
	}
	for id, key := range a.keys {
		n := a.g.Nodes[key]
		if n == nil {
			continue
		}
		us := src.at(key, n)
		if len(us) == 0 {
			continue
		}
		t.unc[id] = t.items
		start := len(t.entries)
		for _, u := range us {
			t.entries = append(t.entries, a.scoped(u))
		}
		t.at[t.items] = [2]int{start, len(t.entries)}
		t.items++
	}
	return t
}

// scoped adds the targets that the scope of an uncertainty concerns.
func (a *adjacency) scoped(u Uncertainty) Scoped {
	s := Scoped{Uncertainty: u}
	var closure []bool
	switch {
	case u.Unit != "":
		closure = a.closure(u.Unit)
	case u.Method != "":
		closure = a.methodClosure(u.Method)
	default:
		return s
	}
	for _, id := range a.externals {
		if closure[id] {
			s.Targets = append(s.Targets, a.keys[id])
		}
	}
	return s
}

// own returns the items of one node: its external node, the units it
// enters and its uncertainties.
func (t *table) own(a *adjacency, id int) []int {
	var items []int
	if item, ok := t.target[id]; ok {
		items = append(items, item)
	}
	if item, ok := t.unc[id]; ok {
		items = append(items, item)
	}
	n := a.g.Nodes[a.keys[id]]
	if n == nil {
		return items
	}
	for _, unit := range a.sources().unitsOf(n, a.keys[id]) {
		items = append(items, t.units[unit])
	}
	return items
}

// materialize returns the reached targets and the uncertainty indexes of
// a set. The source itself is no target of its own search.
func (t *table) materialize(a *adjacency, set bitset, source int) Reachable {
	r := Reachable{Reached: []string{}, Uncertainties: slices.Clone(t.global)}
	for _, id := range t.targets {
		if id != source && set.has(t.target[id]) {
			r.Reached = append(r.Reached, a.keys[id])
		}
	}
	for item := len(t.targets); item < t.items; item++ {
		if !set.has(item) {
			continue
		}
		for i := t.at[item][0]; i < t.at[item][1]; i++ {
			r.Uncertainties = append(r.Uncertainties, i)
		}
	}
	return r
}

// bitset is a set of items.
type bitset []uint64

func newBitset(items int) bitset {
	return make(bitset, (items+63)/64)
}

func (b bitset) set(i int)      { b[i/64] |= 1 << (i % 64) }
func (b bitset) has(i int) bool { return b[i/64]&(1<<(i%64)) != 0 }

func (b bitset) or(o bitset) {
	for i := range o {
		b[i] |= o[i]
	}
}

// subset reports whether every item of b is in o.
func (b bitset) subset(o bitset) bool {
	for i := range b {
		if b[i]&^o[i] != 0 {
			return false
		}
	}
	return true
}

func (b bitset) count() int {
	n := 0
	for _, w := range b {
		n += bits.OnesCount64(w)
	}
	return n
}

func (b bitset) bytes() []byte {
	out := make([]byte, 0, 8*len(b))
	for _, w := range b {
		for i := range 8 {
			out = append(out, byte(w>>(8*i)))
		}
	}
	return out
}

// condensation is the strongly connected components of the graph. comp
// holds the component of each node, and order the components in reverse
// topological order, so each one comes after every component that it
// reaches.
type condensation struct {
	comp    []int32
	order   []int32
	members [][]int32
}

// condense finds the components with the algorithm of Tarjan, on an
// explicit stack, so a deep graph does not exhaust the Go stack.
func (a *adjacency) condense() *condensation {
	n := len(a.keys)
	c := &condensation{comp: make([]int32, n)}
	index := make([]int32, n)
	low := make([]int32, n)
	onStack := make([]bool, n)
	for i := range index {
		index[i], c.comp[i] = -1, -1
	}
	var stack []int32
	type frame struct {
		node int32
		next int
	}
	var frames []frame
	next := int32(0)
	for root := range n {
		if index[root] >= 0 {
			continue
		}
		frames = append(frames, frame{node: int32(root)})
		index[root], low[root] = next, next
		next++
		stack = append(stack, int32(root))
		onStack[root] = true
		for len(frames) > 0 {
			f := &frames[len(frames)-1]
			v := f.node
			if f.next < len(a.out[v]) {
				w := int32(a.id[a.g.Edges[a.out[v][f.next]].To])
				f.next++
				switch {
				case index[w] < 0:
					index[w], low[w] = next, next
					next++
					stack = append(stack, w)
					onStack[w] = true
					frames = append(frames, frame{node: w})
				case onStack[w]:
					low[v] = min(low[v], index[w])
				}
				continue
			}
			if low[v] == index[v] {
				id := int32(len(c.members))
				var members []int32
				for {
					w := stack[len(stack)-1]
					stack = stack[:len(stack)-1]
					onStack[w] = false
					c.comp[w] = id
					members = append(members, w)
					if w == v {
						break
					}
				}
				c.members = append(c.members, members)
				c.order = append(c.order, id)
			}
			frames = frames[:len(frames)-1]
			if len(frames) > 0 {
				u := frames[len(frames)-1].node
				low[u] = min(low[u], low[v])
			}
		}
	}
	return c
}

// sets returns the items that each component reaches. A component whose
// set is the set of one successor shares its storage. It polls ctx per
// component and reports whether ctx ended before the walk did. A
// component that the walk did not reach has an empty set.
func (c *condensation) sets(ctx context.Context, a *adjacency, t *table) ([]bitset, bool) {
	sets := make([]bitset, len(c.members))
	for _, id := range c.order {
		if ctx.Err() != nil {
			empty := newBitset(t.items)
			for i := range sets {
				if sets[i] == nil {
					sets[i] = empty
				}
			}
			return sets, true
		}
		var own []int
		var succ []int32
		for _, v := range c.members[id] {
			own = append(own, t.own(a, int(v))...)
			for _, e := range a.out[v] {
				if w := c.comp[a.id[a.g.Edges[e].To]]; w != id {
					succ = append(succ, w)
				}
			}
		}
		slices.Sort(succ)
		succ = slices.Compact(succ)
		sets[id] = c.union(t, sets, own, succ)
	}
	return sets, false
}

// union returns the union of the own items and the sets of the
// successors. When one successor already holds everything, the result is
// that set.
func (c *condensation) union(t *table, sets []bitset, own []int, succ []int32) bitset {
	var largest bitset
	for _, s := range succ {
		if largest == nil || sets[s].count() > largest.count() {
			largest = sets[s]
		}
	}
	if largest != nil {
		holds := true
		for _, item := range own {
			holds = holds && largest.has(item)
		}
		for _, s := range succ {
			holds = holds && sets[s].subset(largest)
		}
		if holds {
			return largest
		}
	}
	set := newBitset(t.items)
	for _, item := range own {
		set.set(item)
	}
	for _, s := range succ {
		set.or(sets[s])
	}
	return set
}

// NoDepthCap is the maxDepth of a Path with no depth cap, as the sets of
// ExploreAll have none.
const NoDepthCap = -1

// Path returns the shortest path from a node to an external target, as
// Explore finds it, or false when the search from the node does not reach
// the target. The node itself is no target of its own search, as in
// Explore. MaxDepth 0 selects 64 hops, and NoDepthCap no cap.
func (x *Explorer) Path(ctx context.Context, from string, target Target, maxDepth int) ([]Hop, bool, error) {
	if maxDepth < 0 && maxDepth != NoDepthCap {
		return nil, false, fmt.Errorf("max depth %d is negative", maxDepth)
	}
	a := x.a
	id, ok := a.id[from]
	if !ok {
		return nil, false, fmt.Errorf("no node %s", from)
	}
	targets := [][]int{slices.DeleteFunc(a.match(target), func(t int) bool { return t == id })}
	if len(targets[0]) == 0 {
		return nil, false, nil
	}
	depth := cmp.Or(maxDepth, defaultMaxDepth)
	if maxDepth == NoDepthCap {
		depth = len(a.keys)
	}
	s := a.search(ctx, []int{id}, depth, targets)
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	node, found := s.nearest(targets[0])
	if !found {
		return nil, false, nil
	}
	return a.path(s, node), true, nil
}
