// Command distgraph prints the distribution graph of a Python project:
// each distribution of its site-packages directories, what Build knows of
// its imports, and the edges between them. It runs no Python and reads no
// byte code. Install the project first, as pip install -e . or uv sync, so
// each distribution that the code imports is in a site-packages directory:
// an import that no distribution holds opens its importer, and one open
// node that the project reaches makes each distribution reached.
//
//	go run ./examples/distgraph -dir ../app -site-packages ../app/.venv/lib/python3.12/site-packages
//	go run ./examples/distgraph -dir ../app -site-packages ../app/.venv/lib/python3.12/site-packages -json
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/safedep/code/distgraph"
)

type dirList []string

func (d *dirList) String() string     { return fmt.Sprint(*d) }
func (d *dirList) Set(v string) error { *d = append(*d, v); return nil }

func main() {
	dir := flag.String("dir", ".", "the root directory of the project")
	name := flag.String("name", "", "the name of the project node, else the name of its directory")
	var sitePackages dirList
	flag.Var(&sitePackages, "site-packages", "a site-packages directory of the environment of the project (repeatable)")
	trustCompiled := flag.Bool("trust-compiled", false, "keep a distribution with an extension module closed")
	asJSON := flag.Bool("json", false, "print the graph as JSON")
	flag.Parse()
	if err := run(os.Stdout, *dir, *name, sitePackages, *trustCompiled, *asJSON); err != nil {
		fmt.Fprintln(os.Stderr, "distgraph:", err)
		os.Exit(1)
	}
}

func run(w io.Writer, dir, name string, sitePackages []string, trustCompiled, asJSON bool) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if name == "" {
		name = filepath.Base(abs)
	}
	in := distgraph.Input{ProjectFS: os.DirFS(abs), ProjectName: name, TrustCompiled: trustCompiled}
	for _, sp := range sitePackages {
		in.SitePackages = append(in.SitePackages, os.DirFS(sp))
	}
	g, err := distgraph.Build(context.Background(), in)
	if err != nil {
		return err
	}
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(g)
	}
	return print(w, g)
}

func print(w io.Writer, g *distgraph.Graph) error {
	for _, p := range g.Problems {
		if _, err := fmt.Fprintf(w, "problem %s: %s\n", p.Path, p.Text); err != nil {
			return err
		}
	}
	for _, n := range g.Nodes {
		reached := "unreached"
		if n.Reached {
			reached = "reached"
		}
		if _, err := fmt.Fprintf(w, "%-9s %-9s %s %s\n", reached, n.State, n.Distribution, n.Reason); err != nil {
			return err
		}
	}
	for _, e := range g.Edges {
		if _, err := fmt.Fprintf(w, "%s -> %s (%s, %s:%d)\n", e.From.Name, e.To.Name, e.Kind, e.Evidence.File, e.Evidence.Line); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "%d files, %d distributions\n", g.Stats.Files, g.Stats.Distributions)
	return err
}
