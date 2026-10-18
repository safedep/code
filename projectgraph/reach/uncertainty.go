package reach

import (
	"cmp"
	"fmt"
	"maps"
	"slices"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/projectgraph"
)

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
	reaches       func(unit string, target Target) bool
	methodReaches func(name string, target Target) bool
	// units holds the file budgets of each unit of dependency code, and
	// entered the units that the search entered.
	units   map[string][]Uncertainty
	entered map[string]bool
}

// sources holds the records, the import prefixes and the truncations of a
// graph by node, so a search and a set computation read them once.
type sources struct {
	unresolved map[string][]projectgraph.UnresolvedSite
	prefixes   map[projectgraph.Site]string
	truncated  map[string][]projectgraph.Truncation
	// global holds the truncations with no node and the records of a unit
	// that no import names. They count for every search.
	global []Uncertainty
	// budgets holds the file budgets of each unit of dependency code. A
	// file of the project is a global budget.
	budgets map[string][]Uncertainty
	// units lists the units that a search can enter, in key order.
	units []string
}

func newSources(g *projectgraph.Graph) *sources {
	s := &sources{unresolved: map[string][]projectgraph.UnresolvedSite{}, prefixes: map[projectgraph.Site]string{}, truncated: map[string][]projectgraph.Truncation{}, budgets: map[string][]Uncertainty{}}
	for _, u := range g.Unresolved {
		s.unresolved[u.Caller] = append(s.unresolved[u.Caller], u)
		if u.ImportPrefix != "" {
			s.prefixes[projectgraph.Site{Path: u.Site.Path, Line: u.Site.Line}] = u.ImportPrefix
		}
	}
	for _, t := range g.Truncated {
		if t.Key == "" {
			s.global = append(s.global, Uncertainty{Kind: Budget, Site: t.Site, Text: "the link step hit its " + string(t.Kind) + " limit"})
		} else {
			s.truncated[t.Key] = append(s.truncated[t.Key], t)
		}
	}
	// A record with no caller is a unit that no import names. Any search
	// can enter it, so it counts for every search.
	for _, r := range s.unresolved[""] {
		s.global = append(s.global, Uncertainty{Kind: UnresolvedCall, Site: r.Site, Text: fmt.Sprintf("%s (%s)", r.Text, r.Reason), Unit: r.Unit})
	}
	// A file of the project can hold a root or a caller that the graph
	// does not know, so it counts where the search does not reach it. A
	// file of dependency code holds no root, and its code runs only when a
	// call enters its package. So it counts only for a search that
	// reaches a node of its unit.
	for _, p := range slices.Sorted(maps.Keys(g.Files)) {
		status := g.Files[p]
		if status.Complete() {
			continue
		}
		u := Uncertainty{Kind: Budget, Key: p, Site: projectgraph.Site{Path: p}, Text: "the file is " + string(status)}
		if unit := fileindex.PackageOf(p); unit != "" {
			s.budgets[unit] = append(s.budgets[unit], u)
		} else {
			s.global = append(s.global, u)
		}
	}
	units := map[string]bool{}
	for _, n := range g.Nodes {
		if unit := fileindex.PackageOf(n.Path); unit != "" {
			units[unit] = true
		}
	}
	for _, r := range g.Unresolved {
		if r.Unit != "" {
			units[r.Unit] = true
		}
	}
	s.units = slices.Sorted(maps.Keys(units))
	return s
}

// at returns the uncertainties of one reached node: its truncations, its
// dynamic flags and its unresolved sites. The fields of each one say what
// it concerns: a Unit, an ImportPrefix, a Method, or every target.
func (s *sources) at(key string, n *projectgraph.Node) []Uncertainty {
	var out []Uncertainty
	for _, t := range s.truncated[key] {
		site := t.Site
		site.Path = cmp.Or(site.Path, n.Path)
		out = append(out, Uncertainty{Kind: Budget, Key: key, Site: site, Text: "the link step hit its " + string(t.Kind) + " limit"})
	}
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
		case fl.Kind == fileindex.FlagComputedImport && s.prefixes[site] != "":
			u.ImportPrefix = s.prefixes[site]
		}
		out = append(out, u)
	}
	for _, r := range s.unresolved[key] {
		u := Uncertainty{Kind: UnresolvedCall, Key: key, Site: r.Site, Text: fmt.Sprintf("%s (%s)", r.Text, r.Reason)}
		switch {
		case r.Unit != "":
			u.Unit = r.Unit
		case r.ImportPrefix != "":
			u.ImportPrefix = r.ImportPrefix
		case r.Method != "" && !projectgraph.ReachesAny(r.Reason):
			u.Method = r.Method
		}
		out = append(out, u)
	}
	return out
}

// unitsOf returns the units that a node makes a search enter: the unit of
// its file and the units of its records.
func (s *sources) unitsOf(n *projectgraph.Node, key string) []string {
	var units []string
	if u := fileindex.PackageOf(n.Path); u != "" {
		units = append(units, u)
	}
	for _, r := range s.unresolved[key] {
		for _, u := range []string{fileindex.PackageOf(r.Site.Path), r.Unit} {
			if u != "" {
				units = append(units, u)
			}
		}
	}
	return units
}

func (a *adjacency) uncertainties(s *search, maxDepth int) *found {
	g := a.g
	src := a.sources()
	f := &found{byMethod: map[string][]Uncertainty{}, reaches: a.unitReaches, methodReaches: a.methodReaches, units: src.budgets, entered: map[string]bool{}}
	for _, u := range src.global {
		f.add(u)
	}
	for _, id := range s.order {
		key := a.keys[id]
		n := g.Nodes[key]
		if n == nil {
			continue
		}
		for _, unit := range src.unitsOf(n, key) {
			f.enter(unit)
		}
		for _, u := range src.at(key, n) {
			f.add(u)
		}
	}
	if s.depthCut {
		f.global = append(f.global, Uncertainty{Kind: Budget, Text: fmt.Sprintf("the search stopped at depth %d", maxDepth)})
	}
	if s.stopped {
		f.global = append(f.global, Uncertainty{Kind: Budget, Text: "the search stopped at the deadline"})
	}
	return f
}

// add routes an uncertainty by what it concerns: a unit, an import
// prefix, a method name, or every target.
func (f *found) add(u Uncertainty) {
	switch {
	case u.Unit != "":
		f.byUnit = append(f.byUnit, u)
	case u.ImportPrefix != "":
		f.prefixed = append(f.prefixed, prefixed{u: u, prefix: u.ImportPrefix})
	case u.Method != "":
		if _, ok := f.byMethod[u.Method]; !ok {
			f.methods = append(f.methods, u.Method)
		}
		f.byMethod[u.Method] = append(f.byMethod[u.Method], u)
	default:
		f.global = append(f.global, u)
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

// all returns every uncertainty of the search with no cap, in the order of
// the search: first the ones that concern every target, then the ones
// that a Method, an ImportPrefix or a Unit scopes.
func (f *found) all() []Uncertainty {
	out := slices.Clone(f.global)
	for _, m := range f.methods {
		out = append(out, f.byMethod[m]...)
	}
	for _, p := range f.prefixed {
		out = append(out, p.u)
	}
	return append(out, f.byUnit...)
}

// forTarget returns the uncertainties of a target, at most maxPerKind of
// each kind, the sites that a computed import prefix sets aside, and the
// count before the cap.
func (f *found) forTarget(target Target) ([]Uncertainty, []Uncertainty, int) {
	all := slices.Clone(f.global)
	for _, m := range f.methods {
		if target.hasMethod(m) || f.methodReaches(m, target) {
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
		if target.mayImport(p.prefix) {
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
