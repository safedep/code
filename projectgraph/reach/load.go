package reach

import "github.com/safedep/code/projectgraph"

// LoadResolver is the Resolver of a hop into a module top level that the
// program runs before the code of the hop before it, as the init code of
// a Go package before a function of the package.
const LoadResolver = "load"

// loadGroup holds the module top levels of one load group: all of them,
// and the ones of the files that are not tests.
type loadGroup struct {
	all, run []int
}

// loadsOf returns the module top levels that the program runs before the
// code of a node can run: the files of the load group of its file. Code
// at a module top level holds the package-level values, as a table of
// function values, and the reference edges of a value start there. A
// search from a source or a root that the rules do not tie to the load
// code would not reach them. A test file loads only with the code of a
// test.
func (a *adjacency) loadsOf(id int) []int {
	n := a.g.Nodes[a.keys[id]]
	if n == nil || n.Path == "" {
		return nil
	}
	if a.loadGroups == nil {
		a.loadGroups = map[string]*loadGroup{}
		for fid, key := range a.keys {
			f := a.g.Nodes[key]
			if f == nil || f.Kind != projectgraph.NodeModule || f.Path != key {
				continue
			}
			k := a.g.LoadGroup(key)
			grp := a.loadGroups[k]
			if grp == nil {
				grp = &loadGroup{}
				a.loadGroups[k] = grp
			}
			grp.all = append(grp.all, fid)
			if !f.Test {
				grp.run = append(grp.run, fid)
			}
		}
	}
	grp := a.loadGroups[a.g.LoadGroup(n.Path)]
	switch {
	case grp == nil:
		return nil
	case n.Test:
		return grp.all
	}
	return grp.run
}

// loadParent is the parent of a module top level that a search enters
// from the node u by its load group. It is below -1, the parent of a root.
func loadParent(u int) int32 {
	return int32(-2 - u)
}

// edgeInto returns the edge into a node on its shortest path: an edge of
// the graph, or the load step from the node that entered its load group.
func (a *adjacency) edgeInto(s *search, node int) projectgraph.Edge {
	p := s.parent[node]
	if p >= 0 {
		return a.g.Edges[p]
	}
	key := a.keys[node]
	return projectgraph.Edge{
		From:       a.keys[-2-p],
		To:         key,
		Site:       projectgraph.Site{Path: key},
		Kind:       projectgraph.EdgeImport,
		Confidence: projectgraph.ConfidenceHigh,
		Resolver:   LoadResolver,
	}
}
