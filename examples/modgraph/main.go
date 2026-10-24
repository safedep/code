// Command modgraph prints the module graph of a Go module: each module of
// the build list, what Build knows of its imports, and the edges between
// them. It finds the source of each module with modgraph.Discover in the
// vendor directory and in the module cache, which -modcache names, or
// $GOMODCACHE, or $GOPATH/pkg/mod. It runs no go command and downloads
// nothing. Run go mod download first for a graph with no open node.
//
//	go run ./examples/modgraph -dir ../vet
//	go run ./examples/modgraph -dir ../vet -json
package main

import (
	"cmp"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/safedep/code/modgraph"
)

func main() {
	dir := flag.String("dir", ".", "the directory of the main go.mod")
	modcache := flag.String("modcache", "", "the module cache, else $GOMODCACHE, else $GOPATH/pkg/mod, else $HOME/go/pkg/mod")
	asJSON := flag.Bool("json", false, "print the graph as JSON")
	flag.Parse()
	if err := run(os.Stdout, *dir, *modcache, *asJSON); err != nil {
		fmt.Fprintln(os.Stderr, "modgraph:", err)
		os.Exit(1)
	}
}

func run(w io.Writer, dir, modcache string, asJSON bool) error {
	sources, err := modgraph.Discover(modgraph.DiscoverOptions{Dir: dir, ModuleCache: moduleCache(modcache), LocalRoot: dir})
	if err != nil {
		return err
	}
	g, err := modgraph.Build(context.Background(), modgraph.Input{MainFS: os.DirFS(dir), Modules: sources})
	if err != nil {
		return err
	}
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(g)
	}
	for _, n := range g.Nodes {
		reached := "unreached"
		if n.Reached {
			reached = "reached"
		}
		if _, err := fmt.Fprintf(w, "%-9s %-9s %s %s\n", reached, n.State, n.Module, n.Reason); err != nil {
			return err
		}
	}
	for _, e := range g.Edges {
		if _, err := fmt.Fprintf(w, "%s -> %s (%s, %s:%d)\n", e.From.Path, e.To.Path, e.Kind, e.Evidence.File, e.Evidence.Line); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(w, "%d files, %d packages\n", g.Stats.Files, g.Stats.Packages)
	return err
}

// moduleCache finds the module cache as the go command does, with no go
// command.
func moduleCache(flagValue string) string {
	if c := cmp.Or(flagValue, os.Getenv("GOMODCACHE")); c != "" {
		return c
	}
	if p := filepath.SplitList(os.Getenv("GOPATH")); len(p) > 0 && p[0] != "" {
		return filepath.Join(p[0], "pkg", "mod")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, "go", "pkg", "mod")
	}
	return ""
}
