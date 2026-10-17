package plugin

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/safedep/code/core"
)

type PluginExecutor interface {
	// Execute runs the plugins on each file of the file system. A file that
	// fails at a stage is in the report and the walk goes on. Execute returns
	// an error only when ctx is done or the walk itself fails, and then the
	// report holds the files before the stop. A plugin that fails because
	// ctx is done puts its file in Failed. To stop the walk, a plugin
	// callback cancels ctx.
	Execute(context.Context, core.ImportAwareFileSystem) (*Report, error)
}

// Report is what one Execute did. It lists files by name, in walk order.
type Report struct {
	// Visited are the files whose tree reached the plugins.
	Visited []string

	// Skipped are the files that no language of the walker parses.
	Skipped []string

	// Failed are the failures of files at a stage. A file that fails in a
	// plugin is also in Visited.
	Failed []FileError
}

// FileError is the failure of one file at one stage.
type FileError struct {
	File string

	// Stage is core.StageRead, core.StageParse or the name of the plugin
	// that failed.
	Stage core.Stage

	Err error
}

func (e FileError) Error() string {
	return fmt.Sprintf("%s: %s: %v", e.File, e.Stage, e.Err)
}

func (e FileError) Unwrap() error {
	return e.Err
}

type treeWalkPluginExecutor struct {
	walker  core.TreeWalker
	plugins []core.Plugin
}

var _ PluginExecutor = &treeWalkPluginExecutor{}

type treeVisitor struct {
	plugins []core.Plugin
	report  *Report
}

var _ core.FileErrorVisitor = (*treeVisitor)(nil)

// VisitTree runs each plugin that supports the language of the tree, and
// closes the tree after the last plugin. A plugin must not keep a node of
// the tree after it returns.
func (v *treeVisitor) VisitTree(ctx context.Context, tree core.ParseTree) error {
	defer tree.Close()

	file, err := tree.File()
	if err != nil {
		return fmt.Errorf("failed to get file from tree: %w", err)
	}

	language, err := tree.Language()
	if err != nil {
		return fmt.Errorf("failed to get language from tree: %w", err)
	}

	v.report.Visited = append(v.report.Visited, file.Name())
	code := language.Meta().Code
	for _, plugin := range v.plugins {
		if !slices.Contains(plugin.SupportedLanguages(), code) {
			continue
		}

		if err := analyze(ctx, plugin, file, tree); err != nil {
			v.report.Failed = append(v.report.Failed, FileError{File: file.Name(), Stage: core.Stage(plugin.Name()), Err: err})
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
		}
	}

	return nil
}

func (v *treeVisitor) VisitFileError(_ context.Context, file core.File, stage core.Stage, err error) error {
	if errors.Is(err, core.ErrNoLanguage) {
		v.report.Skipped = append(v.report.Skipped, file.Name())
		return nil
	}

	v.report.Failed = append(v.report.Failed, FileError{File: file.Name(), Stage: stage, Err: err})
	return nil
}

func analyze(ctx context.Context, plugin core.Plugin, file core.File, tree core.ParseTree) error {
	if filePlugin, ok := plugin.(core.FilePlugin); ok {
		if err := filePlugin.AnalyzeSource(ctx, file); err != nil {
			return fmt.Errorf("failed to analyze source: %w", err)
		}
	}

	if treePlugin, ok := plugin.(core.TreePlugin); ok {
		if err := treePlugin.AnalyzeTree(ctx, tree); err != nil {
			return fmt.Errorf("failed to analyze tree: %w", err)
		}
	}

	return nil
}

// NewTreeWalkPluginExecutor creates a simple plugin executor using a tree walker.
// It just makes it easy to execute plugins suitable for ParseTree and File
func NewTreeWalkPluginExecutor(walker core.TreeWalker, plugins []core.Plugin) (*treeWalkPluginExecutor, error) {
	return &treeWalkPluginExecutor{
		walker:  walker,
		plugins: plugins,
	}, nil
}

func (e *treeWalkPluginExecutor) Execute(ctx context.Context, fs core.ImportAwareFileSystem) (*Report, error) {
	visitor := &treeVisitor{plugins: e.plugins, report: &Report{}}
	err := e.walker.Walk(ctx, fs, visitor)
	return visitor.report, err
}
