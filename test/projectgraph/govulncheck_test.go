//go:build projectgraph

package projectgraph

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/safedep/code/core"
	fidx "github.com/safedep/code/fileindex"
	codefs "github.com/safedep/code/fs"
	"github.com/safedep/code/lang"
	"github.com/safedep/code/plugin/fileindex"
	"github.com/safedep/code/projectgraph"
	"github.com/safedep/code/projectgraph/reach"
	"github.com/stretchr/testify/require"
	"golang.org/x/mod/modfile"
)

// govulncheckDir holds the source modules and the JSON test scripts of
// govulncheck. NOTICE names the commit.
var govulncheckDir = filepath.Join("public", "govulncheck")

// vulnFrame is one frame of a govulncheck trace. The trace starts at the
// vulnerable symbol and ends at the entry of the program. The position of a
// frame is the call site of the frame before it.
type vulnFrame struct {
	Module   string `json:"module"`
	Package  string `json:"package"`
	Function string `json:"function"`
	Receiver string `json:"receiver"`
	Position *struct {
		Filename string `json:"filename"`
		Line     int    `json:"line"`
	} `json:"position"`
}

type vulnMessage struct {
	OSV *struct {
		ID       string `json:"id"`
		Affected []struct {
			EcosystemSpecific struct {
				Imports []struct {
					Path    string   `json:"path"`
					Symbols []string `json:"symbols"`
				} `json:"imports"`
			} `json:"ecosystem_specific"`
		} `json:"affected"`
	} `json:"osv"`
	Finding *struct {
		OSV   string      `json:"osv"`
		Trace []vulnFrame `json:"trace"`
	} `json:"finding"`
}

// vulnRun is the output of one govulncheck command of a test script.
type vulnRun struct {
	dir      string
	findings [][]vulnFrame
	// symbols holds the symbols of each OSV entry, as pkg//Type//Method.
	symbols map[string][]string
}

var (
	moddirArg = regexp.MustCompile(`-C \$\{moddir\}/(\S+)`)
	// placeholder matches a position that a script leaves out, as the line
	// of a frame in the standard library.
	placeholder = regexp.MustCompile(`<[a-z]>`)
)

// readVulnRuns reads the commands of a test script and the JSON that each
// one prints.
func readVulnRuns(t *testing.T, file string) []vulnRun {
	data, err := os.ReadFile(file)
	require.NoError(t, err)
	var runs []vulnRun
	var body bytes.Buffer
	flush := func() {
		if len(runs) == 0 {
			return
		}
		run := &runs[len(runs)-1]
		dec := json.NewDecoder(&body)
		for {
			var m vulnMessage
			err := dec.Decode(&m)
			if errors.Is(err, io.EOF) {
				break
			}
			require.NoError(t, err, file)
			switch {
			case m.OSV != nil:
				for _, a := range m.OSV.Affected {
					for _, imp := range a.EcosystemSpecific.Imports {
						for _, sym := range imp.Symbols {
							run.symbols[m.OSV.ID] = append(run.symbols[m.OSV.ID], fidx.Join(fidx.Namespace("go", imp.Path), strings.Split(sym, ".")...))
						}
					}
				}
			case m.Finding != nil && len(m.Finding.Trace) > 0 && m.Finding.Trace[0].Function != "":
				run.findings = append(run.findings, m.Finding.Trace)
			}
		}
		body.Reset()
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "$ govulncheck"):
			flush()
			m := moddirArg.FindStringSubmatch(line)
			require.NotNil(t, m, line)
			runs = append(runs, vulnRun{dir: m[1], symbols: map[string][]string{}})
		case strings.HasPrefix(line, "#"):
			flush()
		default:
			body.WriteString(placeholder.ReplaceAllString(line, "0") + "\n")
		}
	}
	require.NoError(t, scanner.Err())
	flush()
	return runs
}

// vulnScore counts the traces of govulncheck that the graph finds. A trace
// gives the first function of a dependency that the module calls, and the
// call site. found counts the traces whose function the reach query finds,
// site those whose path ends at the call site of the trace, of the sited
// traces that give a line, and stack those
// whose path has the functions of the module in the order of the trace.
// called counts the symbols that govulncheck reports as not called and that
// the reach query finds.
type vulnScore struct {
	traces, found, sited, site, stack int
	quiet, called                     int
	modules                           map[string]bool
	files                             int
}

func TestGovulncheckTraces(t *testing.T) {
	ctx := context.Background()
	language, err := lang.GetLanguage(string(core.LanguageCodeGo))
	require.NoError(t, err)

	files, err := filepath.Glob(filepath.Join(govulncheckDir, "testfiles", "*.ct"))
	require.NoError(t, err)
	require.NotEmpty(t, files)

	s := vulnScore{modules: map[string]bool{}}
	graphs := map[string]*projectgraph.Graph{}
	for _, file := range files {
		for _, run := range readVulnRuns(t, file) {
			root, module := vulnModule(t, run.dir)
			g, ok := graphs[root]
			if !ok {
				g = vulnGraph(ctx, t, root, language)
				graphs[root] = g
				s.files += len(g.Files)
			}
			s.modules[root] = true
			called := map[string]bool{}
			for _, trace := range run.findings {
				called[trace[0].Package] = true
				s.scoreTrace(ctx, t, g, module, trace)
			}
			for _, symbols := range run.symbols {
				for _, sym := range symbols {
					if slices.ContainsFunc(run.findings, func(trace []vulnFrame) bool { return frameKey(trace[0]) == sym }) {
						continue
					}
					s.quiet++
					if reachStatus(ctx, t, g, sym) == reach.Found {
						s.called++
						t.Logf("%s: govulncheck reports no call of %s, and the graph finds one", run.dir, sym)
					}
				}
			}
		}
	}
	rows := []string{
		fmt.Sprintf("set=govulncheck language=go modules=%d files=%d", len(s.modules), s.files),
		fmt.Sprintf("  traces=%d", s.traces),
		fmt.Sprintf("  found=%s", ratio{hit: s.found, all: s.traces}),
		fmt.Sprintf("  call-site=%s", ratio{hit: s.site, all: s.sited}),
		fmt.Sprintf("  stack=%s", ratio{hit: s.stack, all: s.traces}),
		fmt.Sprintf("  false-found=%s", ratio{hit: s.called, all: s.quiet}),
	}
	t.Log("\n" + strings.Join(rows, "\n"))
	if out := os.Getenv("PROJECTGRAPH_GOVULNCHECK_REPORT"); out != "" {
		require.NoError(t, os.WriteFile(out, []byte(strings.Join(rows, "\n")+"\n"), 0o644))
	}
}

// vulnModule returns the directory of the go.mod that holds the directory of
// a command, and the module path.
func vulnModule(t *testing.T, dir string) (string, string) {
	for d := filepath.Join(govulncheckDir, "modules", filepath.FromSlash(dir)); ; d = filepath.Dir(d) {
		data, err := os.ReadFile(filepath.Join(d, "go.mod"))
		if err == nil {
			return d, modfile.ModulePath(data)
		}
		require.ErrorIs(t, err, os.ErrNotExist)
		require.NotEqual(t, govulncheckDir, d, "no go.mod for %s", dir)
	}
}

func vulnGraph(ctx context.Context, t *testing.T, dir string, language core.Language) *projectgraph.Graph {
	fsys, err := codefs.NewLocalFileSystem(codefs.LocalFileSystemConfig{AppDirectories: []string{dir}})
	require.NoError(t, err)
	indexed, err := (&fileindex.Indexer{}).Run(ctx, fsys, fileindex.Options{Languages: []core.Language{language}})
	require.NoError(t, err)
	g, err := projectgraph.Build(ctx, indexed.Indexes, projectgraph.Options{Root: os.DirFS(dir), Files: indexed.Files})
	require.NoError(t, err)
	return g
}

func (s *vulnScore) scoreTrace(ctx context.Context, t *testing.T, g *projectgraph.Graph, module string, trace []vulnFrame) {
	first := slices.IndexFunc(trace, func(f vulnFrame) bool { return f.Module == module })
	if first < 1 || trace[first].Position == nil {
		return
	}
	s.traces++
	target := frameKey(trace[first-1])
	results, err := reach.Reach(ctx, g, reach.Query{Targets: []string{target}})
	require.NoError(t, err)
	r := results[0]
	if r.Status != reach.Found {
		t.Logf("%s: %s, want found", target, r.Status)
		return
	}
	s.found++
	last := r.Path[len(r.Path)-1].Site
	if pos := trace[first].Position; pos.Line > 0 {
		s.sited++
		if path.Base(last.Path) == path.Base(pos.Filename) && last.Line == pos.Line {
			s.site++
		}
	}
	var want, got []string
	for _, f := range trace[first:] {
		want = append(want, frameName(f))
	}
	slices.Reverse(want)
	for _, h := range r.Path[:len(r.Path)-1] {
		if segs := fidx.Segments(h.Key); len(segs) > 0 {
			got = append(got, strings.Join(segs, "."))
		}
	}
	if slices.Equal(want, got) {
		s.stack++
	} else {
		t.Logf("%s: path %v, want %v", target, got, want)
	}
}

func reachStatus(ctx context.Context, t *testing.T, g *projectgraph.Graph, target string) reach.Status {
	results, err := reach.Reach(ctx, g, reach.Query{Targets: []string{target}})
	require.NoError(t, err)
	return results[0].Status
}

// frameKey returns the external key of the function of a frame, as
// github.com//tidwall//gjson//Result//Get.
func frameKey(f vulnFrame) string {
	segs := []string{frameName(f)}
	if f.Receiver != "" {
		segs = strings.Split(frameName(f), ".")
	}
	return fidx.Join(fidx.Namespace("go", f.Package), segs...)
}

// frameName returns the name of the function of a frame with its receiver
// type and with no type arguments, as Result.Get or work.
func frameName(f vulnFrame) string {
	name, _, _ := strings.Cut(f.Function, "[")
	if f.Receiver == "" {
		return name
	}
	return strings.TrimPrefix(f.Receiver, "*") + "." + name
}
