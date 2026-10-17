// Package projectgraph links the file indexes of a project into one call
// graph. The link step reads only fileindex records and the manifests of the
// project. It imports no tree-sitter package, so it builds with no CGO.
package projectgraph

import (
	"time"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/fileindex/roots"
	"github.com/safedep/code/projectgraph/modules"
)

// Version is the version of the link rules. Bump it when a rule that makes a
// node, an edge, a root or an unresolved site changes, so a diff of two
// reports shows the change.
const Version = "4"

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
)

// Confidence returns the confidence that an edge of the kind carries.
func (k EdgeKind) Confidence() Confidence {
	switch k {
	case EdgeCHA:
		return ConfidenceMedium
	case EdgeField, EdgeWildcard, EdgeLLM, EdgeName, EdgeReference:
		return ConfidenceLow
	default:
		return ConfidenceHigh
	}
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

type TruncationKind string

const (
	// TruncationEdges marks a graph that stopped adding edges at the edge
	// limit. Every part of the graph can miss an edge.
	TruncationEdges TruncationKind = "edges"
	// TruncationExportHops marks a name whose chain of re-exports is longer
	// than the hop limit.
	TruncationExportHops TruncationKind = "export-hops"
)

// Site is a place in a first-party file. Lines and columns are 1-based, and
// 0 means not known.
type Site struct {
	Path   string `json:"path"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
}

// Node is a definition, a module top level, a re-exported name or an
// external target. An external node has only its key and kind.
type Node struct {
	Key      string           `json:"key"`
	Kind     NodeKind         `json:"kind"`
	Path     string           `json:"path,omitempty"`
	Line     int              `json:"line,omitempty"`
	Exported bool             `json:"exported,omitempty"`
	Test     bool             `json:"test,omitempty"`
	Flags    []fileindex.Flag `json:"flags,omitempty"`
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
type UnresolvedSite struct {
	Caller       string                     `json:"caller"`
	Site         Site                       `json:"site"`
	Text         string                     `json:"text"`
	Receiver     string                     `json:"receiver,omitempty"`
	Method       string                     `json:"method,omitempty"`
	Reason       fileindex.UnresolvedReason `json:"reason"`
	ImportPrefix string                     `json:"import_prefix,omitempty"`
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
}

// IsExternal reports whether the key names a node outside the project.
func (g *Graph) IsExternal(key string) bool {
	if n, ok := g.Nodes[key]; ok {
		return n.Kind == NodeExternal
	}
	_, ok := g.Files[fileindex.Unit(key)]
	return !ok
}
