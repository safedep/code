package summary

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/projectgraph"
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
	s, err := Summarize(context.Background(), kit(), projectgraph.Options{})
	require.NoError(t, err)
	assert.Equal(t, Version, s.Version)
	assert.Equal(t, fileindex.Version, s.IndexVersion)
	assert.Equal(t, projectgraph.Version, s.LinkVersion)
	assert.Equal(t, unit, s.Unit)
	assert.Empty(t, s.Incomplete)

	var keys []string
	for _, sym := range s.Symbols {
		keys = append(keys, sym.Key)
	}
	assert.Equal(t, []string{"kit//index//run", "kit//index//stop", "kit//run", "kit//stop"}, keys)
	run := s.Symbols[2]
	assert.Equal(t, []string{unit + "//index.js//run"}, run.Definitions)
	require.Len(t, run.Reaches, 1, "a builtin is no symbol of a dependency")
	assert.Equal(t, "child_process//exec", run.Reaches[0].Target)
	assert.Equal(t, "kit//run", run.Reaches[0].Path[0].Key)
	assert.Equal(t, projectgraph.EdgePackage, run.Reaches[0].Path[1].EdgeKind)
	assert.Empty(t, s.Symbols[3].Reaches)

	first, err := json.Marshal(s)
	require.NoError(t, err)
	again, err := Summarize(context.Background(), kit(), projectgraph.Options{})
	require.NoError(t, err)
	second, err := json.Marshal(again)
	require.NoError(t, err)
	assert.Equal(t, string(first), string(second), "one input gives one JSON form")
}

func TestSummarizeKeepsTheUncertaintiesOfTheUnit(t *testing.T) {
	s, err := Summarize(context.Background(), kit(), projectgraph.Options{
		Files: map[string]fileindex.FileStatus{unit + "//lib/big.js": fileindex.FileFailed},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{unit + "//lib/big.js"}, s.Incomplete)
	for _, sym := range s.Symbols {
		require.Len(t, sym.Uncertainties, 1, sym.Key)
		assert.Equal(t, unit+"//lib/big.js", sym.Uncertainties[0].Site.Path)
	}
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
			_, err := Summarize(context.Background(), tt.indexes, projectgraph.Options{Files: tt.files})
			assert.ErrorContains(t, err, tt.err)
		})
	}
}
