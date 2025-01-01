package plugin

import (
	"context"
	"fmt"

	"github.com/safedep/code/core"
)

type PluginExecutor interface {
	Execute(context.Context, core.ImportAwareFileSystem) error
}

type treeWalkPluginExecutor struct {
	walker  core.TreeWalker
	plugins []core.Plugin
}

var _ PluginExecutor = &treeWalkPluginExecutor{}

type treeVisitor struct {
	plugins []core.Plugin
	ctx     context.Context
}

func (v *treeVisitor) VisitTree(language core.Language, tree core.ParseTree) error {
	for _, plugin := range v.plugins {
		file, err := tree.File()
		if err != nil {
			return fmt.Errorf("failed to get file from tree: %w", err)
		}

		if filePlugin, ok := plugin.(core.FilePlugin); ok {
			if err := filePlugin.AnalyzeSource(v.ctx, file); err != nil {
				return fmt.Errorf("failed to analyze source: %w", err)
			}
		}

		if treePlugin, ok := plugin.(core.TreePlugin); ok {
			if err := treePlugin.AnalyzeTree(v.ctx, tree); err != nil {
				return fmt.Errorf("failed to analyze tree: %w", err)
			}
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

func (e *treeWalkPluginExecutor) Execute(ctx context.Context, fs core.ImportAwareFileSystem) error {
	return e.walker.Walk(ctx, fs, &treeVisitor{plugins: e.plugins, ctx: ctx})
}
