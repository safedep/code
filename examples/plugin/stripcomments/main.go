package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fs"
	"github.com/safedep/code/lang"
	"github.com/safedep/code/parser"
	"github.com/safedep/code/plugin"
	"github.com/safedep/code/plugin/stripcomments"
	"github.com/safedep/dry/log"
)

var (
	dirToWalk string
	language  string
)

func init() {
	log.InitZapLogger("walker", "dev")

	flag.StringVar(&dirToWalk, "dir", "", "Directory to walk")
	flag.StringVar(&language, "lang", "python", "Language to use for parsing files")

	flag.Parse()
}

func main() {
	if dirToWalk == "" {
		flag.Usage()
		return
	}

	err := run()
	if err != nil {
		panic(err)
	}
}

func run() error {
	fileSystem, err := fs.NewLocalFileSystem(fs.LocalFileSystemConfig{
		AppDirectories: []string{dirToWalk},
	})

	if err != nil {
		return fmt.Errorf("failed to create local filesystem: %w", err)
	}

	language, err := lang.GetLanguage(language)
	if err != nil {
		return fmt.Errorf("failed to get language: %w", err)
	}

	walker, err := fs.NewSourceWalker(fs.SourceWalkerConfig{}, language)
	if err != nil {
		return fmt.Errorf("failed to create source walker: %w", err)
	}

	treeWalker, err := parser.NewWalkingParser(walker, language)
	if err != nil {
		return fmt.Errorf("failed to create tree walker: %w", err)
	}

	// consume stripped file contents
	var stripCommentsCallback stripcomments.StripCommentsCallback = func(ctx context.Context, strippedData *stripcomments.StripCommentsPluginData) error {
		// Print File contents
		fmt.Println(strippedData.File.Name(), "Stripped --------------------------")
		_, err := io.Copy(os.Stdout, strippedData.Reader)
		fmt.Println("\n-------------------------------------------------------------------------------")
		if err != nil {
			return fmt.Errorf("failed to copy file contents: %w", err)
		}
		return nil
	}

	pluginExecutor, err := plugin.NewTreeWalkPluginExecutor(treeWalker, []core.Plugin{
		stripcomments.NewStripCommentsPlugin(stripCommentsCallback),
	})

	if err != nil {
		return fmt.Errorf("failed to create plugin executor: %w", err)
	}

	return pluginExecutor.Execute(context.Background(), fileSystem)
}
