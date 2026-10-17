// Package overlay reads a file of call edges that a model proposed offline
// and merges them into a project graph. No code in this module calls a
// model. A user or a job writes the file, and a reviewer removes an edge by
// editing it.
//
// The file is JSON. Each edge resolves one unresolved site of the graph, as
// Graph.Unresolved holds it, to a callee that the graph already holds:
//
//	{
//	  "version": "1",
//	  "edges": [
//	    {
//	      "caller": "app/plugins.py//load",
//	      "site": {"path": "app/plugins.py", "line": 12, "column": 12},
//	      "callee": "app/parsers.py//parse",
//	      "kind": "llm",
//	      "confidence": "low",
//	      "model": "claude-opus-5-5",
//	      "prompt_version": "v2",
//	      "input_hash": "sha256:4f1c...",
//	      "output": "parse"
//	    }
//	  ]
//	}
//
// The caller and the site name the unresolved site. The callee is a node key
// of the graph, first party or external. The kind is always llm and the
// confidence is always low. The model, the prompt version, the hash of the
// prompt input and the raw output of the model make the edge auditable.
package overlay

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/safedep/code/projectgraph"
)

// Version is the version of the file format. Bump it when a field changes.
const Version = "1"

// File is one overlay file.
type File struct {
	Version string `json:"version"`
	Edges   []Edge `json:"edges"`
}

// Edge is one edge that a model proposed for an unresolved site.
type Edge struct {
	Caller        string                  `json:"caller"`
	Site          projectgraph.Site       `json:"site"`
	Callee        string                  `json:"callee"`
	Kind          projectgraph.EdgeKind   `json:"kind"`
	Confidence    projectgraph.Confidence `json:"confidence"`
	Model         string                  `json:"model"`
	PromptVersion string                  `json:"prompt_version"`
	InputHash     string                  `json:"input_hash"`
	Output        string                  `json:"output"`
}

// Resolver is the resolver name that the merged graph edge carries. A path
// through the edge shows it, so a reader sees which model and which prompt
// made the hop.
func (e Edge) Resolver() string {
	return fmt.Sprintf("model %s, prompt %s", e.Model, e.PromptVersion)
}

func (e Edge) validate() error {
	switch {
	case e.Kind != projectgraph.EdgeLLM:
		return fmt.Errorf("kind %q is not %s", e.Kind, projectgraph.EdgeLLM)
	case e.Confidence != projectgraph.EdgeLLM.Confidence():
		return fmt.Errorf("confidence %q is not %s", e.Confidence, projectgraph.EdgeLLM.Confidence())
	case e.Caller == "" || e.Callee == "":
		return errors.New("no caller or no callee")
	case e.Site.Path == "" || e.Site.Line < 1:
		return errors.New("no site path or no site line")
	case e.Model == "" || e.PromptVersion == "" || e.InputHash == "" || e.Output == "":
		return errors.New("no model, prompt version, input hash or output")
	}
	return nil
}

// Validate reports the first field of the file that breaks the format.
func (f *File) Validate() error {
	if f.Version != Version {
		return fmt.Errorf("overlay version %q is not %q", f.Version, Version)
	}
	for i, e := range f.Edges {
		if err := e.validate(); err != nil {
			return fmt.Errorf("overlay edge %d: %w", i, err)
		}
	}
	return nil
}

// Read decodes and validates an overlay. An unknown field is an error, so a
// file of another format version does not merge in part.
func Read(r io.Reader) (*File, error) {
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	var f File
	if err := decoder.Decode(&f); err != nil {
		return nil, fmt.Errorf("failed to decode the overlay: %w", err)
	}
	if err := f.Validate(); err != nil {
		return nil, err
	}
	return &f, nil
}

// ReadFile reads the overlay at a path.
func ReadFile(path string) (f *File, err error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	return Read(file)
}

// Reason says why the merge dropped an edge.
type Reason string

const (
	// UndefinedCallee is a callee that is not a node of the graph. The
	// model must pick from the definitions and the external targets that
	// the graph holds.
	UndefinedCallee Reason = "undefined-callee"
	// NoUnresolvedSite is a caller and a site that match no unresolved site
	// of the graph. An overlay resolves only what the link step did not.
	NoUnresolvedSite Reason = "no-unresolved-site"
)

// Diagnostic is an edge that the merge dropped. Index is its place in the
// file.
type Diagnostic struct {
	Index  int    `json:"index"`
	Edge   Edge   `json:"edge"`
	Reason Reason `json:"reason"`
}

func (d Diagnostic) String() string {
	return fmt.Sprintf("overlay edge %d at %s:%d from %s to %s: %s", d.Index, d.Edge.Site.Path, d.Edge.Site.Line, d.Edge.Caller, d.Edge.Callee, d.Reason)
}

// Report is the outcome of a merge. Added counts the edges that changed the
// graph. An edge that a better edge of its slot already holds does not
// count and is not a diagnostic.
type Report struct {
	Added       int          `json:"added"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
}

type siteKey struct {
	caller string
	site   projectgraph.Site
}

// Merge adds the edges of a valid overlay to a built graph with
// Graph.AddEdge. The unresolved sites stay in the graph. So an edge can turn
// a target that no path reaches into a found target, but it never removes
// an uncertainty.
func Merge(g *projectgraph.Graph, f *File) (Report, error) {
	if err := f.Validate(); err != nil {
		return Report{}, err
	}
	unresolved := make(map[siteKey]bool, len(g.Unresolved))
	for _, u := range g.Unresolved {
		unresolved[siteKey{caller: u.Caller, site: u.Site}] = true
	}

	var report Report
	for i, e := range f.Edges {
		switch {
		case !unresolved[siteKey{caller: e.Caller, site: e.Site}]:
			report.Diagnostics = append(report.Diagnostics, Diagnostic{Index: i, Edge: e, Reason: NoUnresolvedSite})
			continue
		case g.Nodes[e.Callee] == nil:
			report.Diagnostics = append(report.Diagnostics, Diagnostic{Index: i, Edge: e, Reason: UndefinedCallee})
			continue
		}
		changed, err := g.AddEdge(projectgraph.Edge{From: e.Caller, To: e.Callee, Site: e.Site, Kind: e.Kind, Resolver: e.Resolver()})
		if err != nil {
			return report, fmt.Errorf("overlay edge %d: %w", i, err)
		}
		if changed {
			report.Added++
		}
	}
	return report, nil
}
