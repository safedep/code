package reach

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

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
