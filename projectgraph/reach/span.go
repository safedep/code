package reach

import (
	"slices"

	"github.com/safedep/code/fileindex"
)

// scopeKey names the code that a scoped uncertainty can run: the nodes
// with a last segment, for an unresolved call of a method, the code of a
// unit, for a call into a unit at no known definition, or the methods that
// code outside the graph can call back.
type scopeKey struct {
	method   string
	unit     string
	callback bool
}

// scopeOf returns the scope of an uncertainty, or false for an uncertainty
// that an import prefix scopes or that concerns every target.
func scopeOf(u Uncertainty) (scopeKey, bool) {
	switch {
	case u.Callback:
		return scopeKey{callback: true}, true
	case u.Unit != "":
		return scopeKey{unit: u.Unit}, true
	case u.ImportPrefix != "":
		return scopeKey{}, false
	case u.Method != "":
		return scopeKey{method: u.Method}, true
	}
	return scopeKey{}, false
}

// span is what the code of a scope can run: the nodes that it reaches, and
// what each uncertainty of those nodes concerns. An unresolved call in that
// code can run code that no edge leads to, so the span goes on into the
// scope of the call. methods holds each method name that the span enters,
// whose targets a call of the name concerns by their last segment, and
// prefixes the import prefix of each computed import. every marks an
// uncertainty that concerns every target. Without it, a target that only a
// candidate of an ambiguous call reaches, through an unresolved call of
// its own, reads as not-found.
type span struct {
	nodes    []bool
	methods  []string
	prefixes []string
	every    bool
}

// concerns reports whether the span concerns a target, by the nodes of the
// target.
func (s *span) concerns(ids []int, t Target) bool {
	return s.every ||
		slices.ContainsFunc(s.methods, t.hasMethod) ||
		slices.ContainsFunc(s.prefixes, t.mayImport) ||
		slices.ContainsFunc(ids, func(id int) bool { return s.nodes[id] })
}

// concerns reports whether the code of a scope can run a target, or meets
// an uncertainty that concerns it.
func (a *adjacency) concerns(k scopeKey, t Target) bool {
	return a.span(k).concerns(a.match(t), t)
}

// span returns the span of a scope, computed once per adjacency.
func (a *adjacency) span(k scopeKey) *span {
	if s, ok := a.spans[k]; ok {
		return s
	}
	if a.spans == nil {
		a.spans = map[scopeKey]*span{}
	}
	w := &spanWalk{a: a, src: a.sources(), s: &span{nodes: make([]bool, len(a.keys))}, inUnit: make([]bool, len(a.keys)), entered: map[scopeKey]bool{}, units: map[string]bool{}}
	w.enter(k)
	w.run()
	a.spans[k] = w.s
	return w.s
}

// spanWalk is the state of the search of one span. The code of a unit is
// any code of the unit, so a node that the code of a unit reaches enters
// every node of its own unit, as inUnit marks. entered holds the scopes
// that the walk entered, and units the units whose code it entered.
type spanWalk struct {
	a       *adjacency
	src     *sources
	s       *span
	inUnit  []bool
	queue   []spanNode
	entered map[scopeKey]bool
	units   map[string]bool
}

type spanNode struct {
	id   int
	unit bool
}

func (w *spanWalk) add(id int, unit bool) {
	if w.inUnit[id] || !unit && w.s.nodes[id] {
		return
	}
	w.s.nodes[id] = true
	w.inUnit[id] = unit
	w.queue = append(w.queue, spanNode{id: id, unit: unit})
}

// enter adds the nodes where the code of a scope starts.
func (w *spanWalk) enter(k scopeKey) {
	if w.entered[k] {
		return
	}
	w.entered[k] = true
	switch {
	case k.callback:
		for _, id := range w.a.callbackMethods() {
			w.add(id, false)
		}
	case k.unit != "":
		w.enterUnit(k.unit)
	default:
		w.s.methods = append(w.s.methods, k.method)
		for _, id := range w.a.definitionsOf(k.method) {
			w.add(id, false)
		}
	}
}

// enterUnit adds every node of a unit as code of the unit. A file of the
// unit with no complete index can hold any code.
func (w *spanWalk) enterUnit(unit string) {
	if w.units[unit] {
		return
	}
	w.units[unit] = true
	w.budget(unit)
	for _, id := range w.a.nodesOf(unit) {
		w.add(id, true)
	}
}

// budget marks every target when a unit that the span enters has a file
// with no complete index, as the search of a query counts it.
func (w *spanWalk) budget(unit string) {
	if len(w.src.budgets[unit]) > 0 || w.a.includeTests && len(w.src.testBudgets[unit]) > 0 {
		w.s.every = true
	}
}

func (w *spanWalk) run() {
	a := w.a
	for i := 0; i < len(w.queue) && !w.s.every; i++ {
		n := w.queue[i]
		if n.unit {
			if u := a.unitOf(n.id); u != "" {
				w.enterUnit(u)
			}
		}
		for _, e := range a.out[n.id] {
			w.add(a.id[a.g.Edges[e].To], n.unit)
		}
		w.meet(n.id)
	}
}

// meet enters the scope of each uncertainty of a node, as the search of a
// query adds it.
func (w *spanWalk) meet(id int) {
	key := w.a.keys[id]
	n := w.a.g.Nodes[key]
	if n == nil {
		return
	}
	for _, unit := range w.src.unitsOf(n, key) {
		w.budget(unit)
	}
	if w.src.callbacks[key] {
		w.enter(scopeKey{callback: true})
	}
	for _, u := range w.src.at(key, n) {
		k, ok := scopeOf(u)
		switch {
		case ok:
			w.enter(k)
		case u.ImportPrefix != "":
			if !slices.Contains(w.s.prefixes, u.ImportPrefix) {
				w.s.prefixes = append(w.s.prefixes, u.ImportPrefix)
			}
		default:
			w.s.every = true
		}
	}
}

// unitOf returns the unit of dependency code of a node, or "".
func (a *adjacency) unitOf(id int) string {
	if n := a.g.Nodes[a.keys[id]]; n != nil {
		return fileindex.PackageOf(n.Path)
	}
	return ""
}

// nodesOf returns the nodes of a unit of dependency code.
func (a *adjacency) nodesOf(unit string) []int {
	if a.unitNodes == nil {
		a.unitNodes = map[string][]int{}
		for id := range a.keys {
			if u := a.unitOf(id); u != "" {
				a.unitNodes[u] = append(a.unitNodes[u], id)
			}
		}
	}
	return a.unitNodes[unit]
}

// definitionsOf returns the nodes with the last segment: the definitions of
// the name in the project and in each unit of dependency code, and the
// external keys of the name, which can lead into a unit. An unresolved
// call of the name can run any of them.
func (a *adjacency) definitionsOf(name string) []int {
	if a.byLast == nil {
		a.byLast = map[string][]int{}
		for id, key := range a.keys {
			if n := a.g.Nodes[key]; n != nil && (a.includeTests || !n.Test) {
				a.byLast[fileindex.Last(key)] = append(a.byLast[fileindex.Last(key)], id)
			}
		}
	}
	return a.byLast[name]
}

// callbackMethods returns the methods that code outside the graph can call
// on a value that it holds, as the graph names them.
func (a *adjacency) callbackMethods() []int {
	if a.callbacks == nil {
		a.callbacks = []int{}
		for id, key := range a.keys {
			if n := a.g.Nodes[key]; n != nil && (a.includeTests || !n.Test) && a.g.CallbackMethod(key) {
				a.callbacks = append(a.callbacks, id)
			}
		}
	}
	return a.callbacks
}
