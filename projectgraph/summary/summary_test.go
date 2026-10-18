package summary_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/projectgraph"
	"github.com/safedep/code/projectgraph/reach"
	"github.com/safedep/code/projectgraph/summary"
	"github.com/safedep/code/projectgraph/summary/summarytest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const unit = "npm:kit@1.0.0"

// kit is a package whose run calls child_process.exec, console.log and its
// own helper, and whose stop calls a name that the package does not give.
func kit() []*fileindex.Index {
	p := unit + "//index.js"
	fn := func(name string, line int) fileindex.Definition {
		return fileindex.Definition{Key: fileindex.Join(p, name), Kind: fileindex.DefFunction, Name: name, StartLine: line, EndLine: line + 2, Exported: true}
	}
	call := func(caller string, line int, callee, target string) fileindex.Call {
		return fileindex.Call{Caller: fileindex.Join(p, caller), Line: line, Column: 3, Callee: callee, Targets: []string{target}, Kind: fileindex.CallCall}
	}
	return []*fileindex.Index{{
		Version: fileindex.Version, Path: p, Language: langcode.Javascript, Unit: unit,
		Imports: []fileindex.Import{{Line: 1, Module: "child_process", Item: "exec", Alias: "exec", Kind: fileindex.ImportStatic, Namespace: "child_process//exec"}},
		Exports: []fileindex.Export{{Name: "run", Target: p + "//run", Line: 3}, {Name: "stop", Target: p + "//stop", Line: 7}},
		Definitions: []fileindex.Definition{
			{Key: p, Kind: fileindex.DefModule, Name: "index", StartLine: 1, EndLine: 12},
			fn("run", 3), fn("stop", 7), {Key: p + "//helper", Kind: fileindex.DefFunction, Name: "helper", StartLine: 11, EndLine: 12},
		},
		Calls: []fileindex.Call{
			call("run", 4, "exec", "child_process//exec"),
			call("run", 5, "console.log", "console//log"),
			call("stop", 8, "helper", p+"//helper"),
		},
	}}
}

func TestSummarize(t *testing.T) {
	r, err := summary.Build(context.Background(), kit(), projectgraph.Options{})
	require.NoError(t, err)
	s := r.Summary
	assert.Equal(t, summary.Version, s.Version)
	assert.Equal(t, fileindex.Version, s.IndexVersion)
	assert.Equal(t, projectgraph.Version, s.LinkVersion)
	assert.Equal(t, unit, s.Unit)
	assert.Empty(t, s.Incomplete)
	assert.Equal(t, []string{"child_process//exec"}, s.Targets, "a builtin and a symbol of the package are no targets")

	var keys []string
	for _, sym := range s.Symbols {
		keys = append(keys, sym.Key)
	}
	assert.Equal(t, []string{"kit//index//run", "kit//index//stop", "kit//run", "kit//stop"}, keys)
	run := s.Symbols[2]
	assert.Equal(t, []string{unit + "//index.js//run"}, run.Definitions)
	assert.Equal(t, []string{"child_process//exec"}, run.Reaches, "a builtin is no symbol of a dependency")
	assert.Empty(t, run.Uncertainties)
	assert.Empty(t, s.Symbols[3].Reaches)
	assert.Equal(t, reach.Found, s.Status("kit//run", "child_process//exec"))
	assert.Equal(t, reach.NotFound, s.Status("kit//stop", "child_process//exec"))
	assert.Equal(t, reach.NoSuchTarget, s.Status("kit//stop", "fs//rm"))
	assert.Equal(t, reach.NotFound, s.Status("kit//nope", "child_process//exec"))

	hops, found, err := r.Path(context.Background(), "kit//run", "child_process//exec")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "kit//run", hops[0].Key)
	assert.Equal(t, projectgraph.EdgePackage, hops[1].EdgeKind)
	assert.Equal(t, "child_process//exec", hops[len(hops)-1].Key)
	_, found, err = r.Path(context.Background(), "kit//stop", "child_process//exec")
	require.NoError(t, err)
	assert.False(t, found)

	first, err := json.Marshal(s)
	require.NoError(t, err)
	again, err := summary.Summarize(context.Background(), kit(), projectgraph.Options{})
	require.NoError(t, err)
	second, err := json.Marshal(again)
	require.NoError(t, err)
	assert.Equal(t, string(first), string(second), "one input gives one JSON form")
}

func TestSummarizeKeepsTheUncertaintiesOfTheUnit(t *testing.T) {
	s, err := summary.Summarize(context.Background(), kit(), projectgraph.Options{
		Files: map[string]fileindex.FileStatus{unit + "//lib/big.js": fileindex.FileFailed},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{unit + "//lib/big.js"}, s.Incomplete)
	require.Len(t, s.Uncertainties, 1)
	assert.Equal(t, unit+"//lib/big.js", s.Uncertainties[0].Site.Path)
	for _, sym := range s.Symbols {
		assert.Equal(t, []int{0}, sym.Uncertainties, sym.Key)
	}
	assert.Equal(t, reach.Uncertain, s.Status("kit//stop", "child_process//exec"))
	assert.Equal(t, reach.Uncertain, s.Status("kit//nope", "child_process//exec"), "a file with no index can hold the symbol")
}

func TestSummarizeTakesOneUnit(t *testing.T) {
	project := &fileindex.Index{Version: fileindex.Version, Path: "main.js", Language: langcode.Javascript}
	other := &fileindex.Index{Version: fileindex.Version, Path: "npm:other@1.0.0//index.js", Language: langcode.Javascript, Unit: "npm:other@1.0.0"}
	tests := []struct {
		name    string
		indexes []*fileindex.Index
		files   map[string]fileindex.FileStatus
		err     string
	}{
		{name: "a file of the project", indexes: append(kit(), project), err: "only files of dependency code"},
		{name: "two units", indexes: append(kit(), other), err: "one unit, not 2"},
		{name: "a file of another unit", indexes: kit(), files: map[string]fileindex.FileStatus{"npm:other@1.0.0//a.js": fileindex.FileMissing}, err: "one unit, not 2"},
		{name: "no file", err: "one unit, not 0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := summary.Summarize(context.Background(), tt.indexes, projectgraph.Options{Files: tt.files})
			assert.ErrorContains(t, err, tt.err)
		})
	}
}

func TestSummarizeManySymbolsIsFast(t *testing.T) {
	const n = 3000
	p := unit + "//index.js"
	ix := &fileindex.Index{
		Version: fileindex.Version, Path: p, Language: langcode.Javascript, Unit: unit,
		Imports:     []fileindex.Import{{Line: 1, Module: "fs", Alias: "fs", Kind: fileindex.ImportStatic, Namespace: "fs"}},
		Definitions: []fileindex.Definition{{Key: p, Kind: fileindex.DefModule, Name: "index", StartLine: 1, EndLine: n + 2}},
	}
	for i := range n {
		name := fmt.Sprintf("f%d", i)
		ix.Exports = append(ix.Exports, fileindex.Export{Name: name, Target: fileindex.Join(p, name), Line: i + 2})
		ix.Definitions = append(ix.Definitions, fileindex.Definition{Key: fileindex.Join(p, name), Kind: fileindex.DefFunction, Name: name, StartLine: i + 2, EndLine: i + 2, Exported: true})
		ix.Calls = append(ix.Calls, fileindex.Call{Caller: fileindex.Join(p, name), Line: i + 2, Column: 3, Callee: "fs.rmSync", Targets: []string{"fs//rmSync"}, Kind: fileindex.CallCall})
	}
	start := time.Now()
	s, err := summary.Summarize(context.Background(), []*fileindex.Index{ix}, projectgraph.Options{})
	require.NoError(t, err)
	assert.Less(t, time.Since(start), 20*time.Second)
	assert.GreaterOrEqual(t, len(s.Symbols), n)
}

func TestStatusMatchesReach(t *testing.T) {
	indexes := kit()
	p := unit + "//index.js"
	ix := indexes[0]
	ix.Definitions = append(ix.Definitions,
		fileindex.Definition{Key: p + "//loop", Kind: fileindex.DefFunction, Name: "loop", StartLine: 13, EndLine: 16, Exported: true},
		fileindex.Definition{Key: p + "//dyn", Kind: fileindex.DefFunction, Name: "dyn", StartLine: 17, EndLine: 19, Exported: true, Flags: []fileindex.Flag{{Kind: fileindex.FlagEval, Line: 18}}},
	)
	ix.Exports = append(ix.Exports, fileindex.Export{Name: "loop", Target: p + "//loop", Line: 13}, fileindex.Export{Name: "dyn", Target: p + "//dyn", Line: 17})
	ix.Calls = append(ix.Calls,
		fileindex.Call{Caller: p + "//loop", Line: 14, Column: 3, Callee: "helper", Targets: []string{p + "//helper"}, Kind: fileindex.CallCall},
		fileindex.Call{Caller: p + "//helper", Line: 12, Column: 3, Callee: "loop", Targets: []string{p + "//loop"}, Kind: fileindex.CallCall},
		fileindex.Call{Caller: p + "//helper", Line: 12, Column: 10, Callee: "fs.rmSync", Targets: []string{"fs//rmSync"}, Kind: fileindex.CallCall},
		fileindex.Call{Caller: p + "//dyn", Line: 19, Column: 3, Callee: "os.exit", Targets: []string{"os//exit"}, Kind: fileindex.CallCall},
	)
	ix.Imports = append(ix.Imports, fileindex.Import{Line: 2, Module: "fs", Alias: "fs", Kind: fileindex.ImportStatic, Namespace: "fs"}, fileindex.Import{Line: 2, Module: "os", Alias: "os", Kind: fileindex.ImportStatic, Namespace: "os"})
	ix.Unresolved = append(ix.Unresolved,
		fileindex.Unresolved{Caller: p + "//loop", Line: 15, Column: 3, Text: "x.exit", Receiver: "x", Method: "exit", Reason: fileindex.UnresolvedUnknownReceiver},
		fileindex.Unresolved{Caller: p + "//stop", Line: 9, Column: 3, Text: "y[k]()", Reason: fileindex.UnresolvedComputedName},
	)
	r, err := summary.Build(context.Background(), indexes, projectgraph.Options{Files: map[string]fileindex.FileStatus{unit + "//lib/big.js": fileindex.FileFailed}})
	require.NoError(t, err)
	summarytest.AssertStatusMatchesReach(t, r)
	assert.Equal(t, reach.Uncertain, r.Summary.Status("kit//loop", "os//exit"), "a record of the method exit concerns os//exit")
	assert.Equal(t, reach.Uncertain, r.Summary.Status("kit//stop", "os//exit"), "a computed name concerns every target")
}

// chain returns a package whose one symbol reaches fs//rmSync through
// hops definitions.
func chain(hops int) []*fileindex.Index {
	p := unit + "//index.js"
	ix := &fileindex.Index{
		Version: fileindex.Version, Path: p, Language: langcode.Javascript, Unit: unit,
		Imports:     []fileindex.Import{{Line: 1, Module: "fs", Alias: "fs", Kind: fileindex.ImportStatic, Namespace: "fs"}},
		Exports:     []fileindex.Export{{Name: "f0", Target: p + "//f0", Line: 2}},
		Definitions: []fileindex.Definition{{Key: p, Kind: fileindex.DefModule, Name: "index", StartLine: 1, EndLine: hops + 3}},
	}
	for i := range hops + 1 {
		name := fmt.Sprintf("f%d", i)
		ix.Definitions = append(ix.Definitions, fileindex.Definition{Key: fileindex.Join(p, name), Kind: fileindex.DefFunction, Name: name, StartLine: i + 2, EndLine: i + 2, Exported: i == 0})
		callee := fmt.Sprintf("f%d", i+1)
		targets := []string{fileindex.Join(p, callee)}
		if i == hops {
			callee, targets = "fs.rmSync", []string{"fs//rmSync"}
		}
		ix.Calls = append(ix.Calls, fileindex.Call{Caller: fileindex.Join(p, name), Line: i + 2, Column: 3, Callee: callee, Targets: targets, Kind: fileindex.CallCall})
	}
	return []*fileindex.Index{ix}
}

func TestPathHasNoDepthCap(t *testing.T) {
	r, err := summary.Build(context.Background(), chain(70), projectgraph.Options{})
	require.NoError(t, err)
	assert.Equal(t, []string{"fs//rmSync"}, r.Summary.Symbols[0].Reaches)
	assert.Equal(t, reach.Found, r.Summary.Status("kit//f0", "fs//rmSync"))
	hops, found, err := r.Path(context.Background(), "kit//f0", "fs//rmSync")
	require.NoError(t, err)
	require.True(t, found, "every target of Reaches has a path")
	assert.Equal(t, "fs//rmSync", hops[len(hops)-1].Key)
	assert.Greater(t, len(hops), 64)
}

func TestBuildReturnsTheErrorOfTheContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := summary.Build(ctx, chain(3), projectgraph.Options{})
	assert.ErrorIs(t, err, context.Canceled)
}
