// Package projectgraph links the file indexes of a project into one call
// graph. The link step reads only fileindex records and the manifests of the
// project. It imports no tree-sitter package, so it builds with no CGO.
package projectgraph

import (
	"cmp"
	"fmt"
	"slices"
	"time"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/fileindex/roots"
	"github.com/safedep/code/projectgraph/modules"
)

// Version is the version of the link rules. Bump it when a rule that makes a
// node, an edge, a root or an unresolved site changes, so a diff of two
// reports shows the change.
const Version = "15"

type NodeKind string

const (
	NodeModule      NodeKind = "module"
	NodeFunction    NodeKind = "function"
	NodeMethod      NodeKind = "method"
	NodeConstructor NodeKind = "constructor"
	NodeClass       NodeKind = "class"
	// NodeInterface is a TypeScript interface. It has no code. A declared
	// type names it, and the classes that implement it hold the methods.
	NodeInterface NodeKind = "interface"
	// NodeVariable is a name that the top level of a module assigns. A walk
	// through it goes on to the value that it holds.
	NodeVariable NodeKind = "variable"
	// NodeExport is a name that a file imports and gives on to its own
	// importers, as a barrel file or an __init__.py does. The node keeps the
	// barrel on a path.
	NodeExport   NodeKind = "export"
	NodeExternal NodeKind = "external"
)

type EdgeKind string

const (
	EdgeDirect     EdgeKind = "direct"
	EdgeImport     EdgeKind = "import"
	EdgeExport     EdgeKind = "export"
	EdgeMember     EdgeKind = "member"
	EdgeAssignment EdgeKind = "assignment"
	EdgeCHA        EdgeKind = "cha"
	EdgeField      EdgeKind = "field"
	EdgeWildcard   EdgeKind = "wildcard"
	EdgeLLM        EdgeKind = "llm"
	// EdgeName joins the caller of a call with an unknown receiver to each
	// first-party definition with the name of the called method.
	EdgeName EdgeKind = "name"
	// EdgeReference joins a definition to a function, a method, a class or
	// an import that it uses as a value. The code that gets the value can
	// call it.
	EdgeReference EdgeKind = "reference"
	// EdgePackage joins an external key, as lodash//template, to the
	// definition that a unit of dependency code gives for it. It is no
	// call. It marks where a path leaves the code of its caller and goes
	// on into a package.
	EdgePackage EdgeKind = "package"
)

// outranks reports whether an edge of the kind replaces an edge of the old
// kind at one slot: the caller, the callee and the site. The higher
// confidence wins, and a tie keeps the old edge.
func (k EdgeKind) outranks(old EdgeKind) bool {
	return k.Confidence().Rank() > old.Confidence().Rank()
}

// confidences holds the confidence of each edge kind. A kind with no row
// is high, as a decoded graph of a newer version can carry one.
var confidences = map[EdgeKind]Confidence{
	EdgeDirect:     ConfidenceHigh,
	EdgeImport:     ConfidenceHigh,
	EdgeExport:     ConfidenceHigh,
	EdgeMember:     ConfidenceHigh,
	EdgeAssignment: ConfidenceHigh,
	EdgePackage:    ConfidenceHigh,
	EdgeCHA:        ConfidenceMedium,
	EdgeField:      ConfidenceLow,
	EdgeWildcard:   ConfidenceLow,
	EdgeLLM:        ConfidenceLow,
	EdgeName:       ConfidenceLow,
	EdgeReference:  ConfidenceLow,
}

// Confidence returns the confidence that an edge of the kind carries.
func (k EdgeKind) Confidence() Confidence {
	return cmp.Or(confidences[k], ConfidenceHigh)
}

type Confidence string

const (
	ConfidenceHigh   Confidence = "high"
	ConfidenceMedium Confidence = "medium"
	ConfidenceLow    Confidence = "low"
)

// Rank orders the confidences, low first. An unknown confidence ranks 0.
func (c Confidence) Rank() int {
	switch c {
	case ConfidenceLow:
		return 1
	case ConfidenceMedium:
		return 2
	case ConfidenceHigh:
		return 3
	default:
		return 0
	}
}

// RootKind is the kind of a root, as the root rules name it.
type RootKind = roots.Kind

const (
	RootMain           = roots.Main
	RootManifest       = roots.Manifest
	RootFramework      = roots.Framework
	RootTest           = roots.Test
	RootLibrarySurface = roots.LibrarySurface
)

// TruncationKind names the limit of the link step that a Truncation
// records.
type TruncationKind string

const (
	// TruncationEdges marks a graph that stopped adding edges at the edge
	// limit. Every part of the graph can miss an edge.
	TruncationEdges TruncationKind = "edges"
	// TruncationExportHops marks a name whose chain of re-exports is longer
	// than the hop limit.
	TruncationExportHops TruncationKind = "export-hops"
	// TruncationPackageClauses marks a Go file whose package clause is not
	// the clause of the package of its directory. The link step does not
	// read build tags, so an import of the directory does not reach it.
	TruncationPackageClauses TruncationKind = "package-clauses"
)

// Site is a place in a first-party file. Lines and columns are 1-based, and
// 0 means not known.
type Site struct {
	Path   string `json:"path"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
}

// String writes the site as path:line, or as the path alone for a site
// with no line.
func (s Site) String() string {
	if s.Line == 0 {
		return s.Path
	}
	return fmt.Sprintf("%s:%d", s.Path, s.Line)
}

// Node is a definition, a module top level, a re-exported name or an
// external target. An external node has only its key and kind. External
// is the key that an importer of the package writes for a function, a
// method or a constructor of a unit of dependency code, as
// golang.org//x//net//html//render1.
type Node struct {
	Key      string           `json:"key"`
	Kind     NodeKind         `json:"kind"`
	Path     string           `json:"path,omitempty"`
	Line     int              `json:"line,omitempty"`
	Exported bool             `json:"exported,omitempty"`
	Test     bool             `json:"test,omitempty"`
	Flags    []fileindex.Flag `json:"flags,omitempty"`
	External string           `json:"external,omitempty"`
}

// Edge is one resolved step from a caller to a callee. Resolver names the
// rule that made the edge, as python.import.
type Edge struct {
	From       string     `json:"from"`
	To         string     `json:"to"`
	Site       Site       `json:"site"`
	Kind       EdgeKind   `json:"kind"`
	Confidence Confidence `json:"confidence"`
	Resolver   string     `json:"resolver"`
}

// UnresolvedSite is a call site that neither the index nor the link step
// resolved. ImportPrefix is the key form of the static prefix of a computed
// import, so the reach query can set aside a site that cannot name a target.
// Unit names the package unit that a site enters at no known definition,
// as a name that the package makes at run time, or a module that the
// package does not have. The site of an external key has the key as its
// caller and the unit as its site. The code that runs there is code of the
// unit, so it can reach only what the code of the unit can reach. A site
// with no caller is a unit that no import names, and it holds for every
// search.
type UnresolvedSite struct {
	Caller       string                     `json:"caller"`
	Site         Site                       `json:"site"`
	Text         string                     `json:"text"`
	Receiver     string                     `json:"receiver,omitempty"`
	Method       string                     `json:"method,omitempty"`
	Reason       fileindex.UnresolvedReason `json:"reason"`
	ImportPrefix string                     `json:"import_prefix,omitempty"`
	Unit         string                     `json:"unit,omitempty"`
}

// Root is a node that runs with no caller in the graph. Rule is the id of the
// root rule that marked it.
type Root struct {
	Key  string   `json:"key"`
	Kind RootKind `json:"kind"`
	Rule string   `json:"rule"`
	Site Site     `json:"site"`
}

// Truncation records a limit of the link step. Key is the node where the
// limit hit, and is empty when the limit holds for the whole graph.
type Truncation struct {
	Kind TruncationKind `json:"kind"`
	Key  string         `json:"key,omitempty"`
	Site Site           `json:"site"`
}

// Stats holds the counts of a graph. Duration is not in the JSON form, so two
// builds of one input give one JSON form.
type Stats struct {
	Version      string        `json:"version"`
	Files        int           `json:"files"`
	Nodes        int           `json:"nodes"`
	Edges        int           `json:"edges"`
	Unresolved   int           `json:"unresolved"`
	Roots        int           `json:"roots"`
	RootFallback bool          `json:"root_fallback,omitempty"`
	Duration     time.Duration `json:"-"`
}

// Graph is the call graph of one project. Edges, Unresolved, Roots and
// Truncated are in a fixed order. Problems lists the manifests that the link
// step could not read.
type Graph struct {
	Nodes      map[string]*Node                `json:"nodes"`
	Edges      []Edge                          `json:"edges"`
	Unresolved []UnresolvedSite                `json:"unresolved"`
	Roots      []Root                          `json:"roots"`
	Files      map[string]fileindex.FileStatus `json:"files"`
	Truncated  []Truncation                    `json:"truncated,omitempty"`
	Problems   []modules.Problem               `json:"problems,omitempty"`
	Stats      Stats                           `json:"stats"`

	// edges is the set of Edges, which AddEdge makes on its first call and
	// keeps for the next ones.
	edges *edgeSet
}

// IsExternal reports whether the key names a node outside the project.
func (g *Graph) IsExternal(key string) bool {
	if n, ok := g.Nodes[key]; ok {
		return n.Kind == NodeExternal
	}
	_, ok := g.Files[fileindex.Unit(key)]
	return !ok
}

// AddEdge adds an edge to a built graph with the rule of the link step: the
// graph keeps one edge per caller, callee and site, and the edge of the
// higher confidence. It sets the confidence from the kind. It reports
// whether the graph changed. It never removes an unresolved site, so an
// edge from another source, as an LLM overlay, adds a path and leaves the
// uncertainty of the site. The caller must be a node of the graph, and a
// callee outside the project gets an external node.
func (g *Graph) AddEdge(e Edge) (bool, error) {
	if g.Nodes[e.From] == nil {
		return false, fmt.Errorf("edge from %s: no such node", e.From)
	}
	if g.Nodes[e.To] == nil {
		if !g.IsExternal(e.To) {
			return false, fmt.Errorf("edge to %s: no such node", e.To)
		}
		g.Nodes[e.To] = &Node{Key: e.To, Kind: NodeExternal}
		g.Stats.Nodes = len(g.Nodes)
	}
	if g.edges == nil {
		g.edges = newEdgeSet(len(g.Edges))
		for _, old := range g.Edges {
			g.edges.add(old)
		}
	}
	e = newEdge(e.From, e.To, e.Site, e.Kind, e.Resolver)
	if !g.edges.add(e) {
		return false, nil
	}
	i, found := slices.BinarySearchFunc(g.Edges, e, compareSlots)
	if found {
		g.Edges[i] = e
		return true, nil
	}
	g.Edges = slices.Insert(g.Edges, i, e)
	g.Stats.Edges = len(g.Edges)
	return true, nil
}

// compareSlots orders edges by caller, site and callee, as Edges are.
func compareSlots(a, b Edge) int {
	return cmp.Or(
		cmp.Compare(a.From, b.From),
		cmp.Compare(a.Site.Path, b.Site.Path),
		cmp.Compare(a.Site.Line, b.Site.Line),
		cmp.Compare(a.Site.Column, b.Site.Column),
		cmp.Compare(a.To, b.To),
	)
}
