// Package fileindex builds the file index of each source file: the
// projection of the call graph of the file into the records of
// github.com/safedep/code/fileindex.
package fileindex

import (
	"context"
	"fmt"

	"github.com/safedep/code/core"
	fidx "github.com/safedep/code/fileindex"
	"github.com/safedep/code/plugin/callgraph"
)

type fileIndexPlugin struct {
	callback core.PluginCallback[*fidx.Index]
}

var _ core.TreePlugin = (*fileIndexPlugin)(nil)

// NewFileIndexPlugin returns a plugin that calls cb with the index of each
// tree. The index of a file that the deadline cut is partial. The plugin
// gives it to cb with a deadline truncation, and then fails the file.
func NewFileIndexPlugin(cb func(context.Context, *fidx.Index) error) core.TreePlugin {
	return &fileIndexPlugin{callback: cb}
}

func (p *fileIndexPlugin) Name() string {
	return "FileIndexPlugin"
}

func (p *fileIndexPlugin) SupportedLanguages() []core.LanguageCode {
	return callgraph.SupportedLanguages()
}

func (p *fileIndexPlugin) AnalyzeTree(ctx context.Context, tree core.ParseTree) error {
	ix, err := project(ctx, tree)
	if err != nil {
		return fmt.Errorf("failed to index file: %w", err)
	}

	if err := p.callback(ctx, ix); err != nil {
		return err
	}

	if partial(ix) {
		return fmt.Errorf("index of %s is partial: %w", ix.Path, ctx.Err())
	}
	return nil
}

// partial reports that the deadline cut the build of the index.
func partial(ix *fidx.Index) bool {
	for _, t := range ix.Truncated {
		if t.Kind == fidx.TruncationDeadline {
			return true
		}
	}
	return false
}
