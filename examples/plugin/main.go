package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fs"
	"github.com/safedep/code/lang"
	"github.com/safedep/code/parser"
	"github.com/safedep/code/plugin"
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

// Example tree plugin
type exampleTreePlugin struct{}

// Verify contract
var _ core.TreePlugin = (*exampleTreePlugin)(nil)

func (p *exampleTreePlugin) Name() string {
	return "exampleTreePlugin"
}

var supportedLanguages = []core.LanguageCode{core.LanguageCodePython, core.LanguageCodeJavascript}

func (p *exampleTreePlugin) SupportedLanguages() []core.LanguageCode {
	return supportedLanguages
}

// Example plugin handler that actually performs the analysis
// on a parse tree
func (p *exampleTreePlugin) AnalyzeTree(ctx context.Context, tree core.ParseTree) error {
	lang, err := tree.Language()
	if err != nil {
		return fmt.Errorf("failed to get language: %w", err)
	}

	file, err := tree.File()
	if err != nil {
		return fmt.Errorf("failed to get file: %w", err)
	}

	log.Debugf("examplePlugin - Analyzing tree for language: %s, file: %s\n",
		lang.Meta().Code, file.Name())

	return nil
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

	pluginExecutor, err := plugin.NewTreeWalkPluginExecutor(treeWalker, []core.Plugin{
		&exampleTreePlugin{},
	})

	if err != nil {
		return fmt.Errorf("failed to create plugin executor: %w", err)
	}

	return pluginExecutor.Execute(context.Background(), fileSystem)
}
