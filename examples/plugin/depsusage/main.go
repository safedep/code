package main

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fs"
	"github.com/safedep/code/lang"
	"github.com/safedep/code/parser"
	"github.com/safedep/code/plugin"
	"github.com/safedep/code/plugin/depsusage"
	"github.com/safedep/dry/log"
)

var (
	dirToWalk string
	languages arrayFlags
)

type arrayFlags []string

func (a *arrayFlags) String() string {
	return strings.Join(*a, ", ")
}
func (a *arrayFlags) Set(value string) error {
	*a = append(*a, value)
	return nil
}

func init() {
	log.InitZapLogger("walker", "dev")

	flag.StringVar(&dirToWalk, "dir", "", "Directory to walk")
	flag.Var(&languages, "lang", "Languages to use for parsing files")

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

	var filteredLanguages []core.Language
	if len(languages) == 0 {
		filteredLanguages, err = lang.AllLanguages()
		if err != nil {
			return fmt.Errorf("failed to get all languages: %w", err)
		}
	} else {
		for _, language := range languages {
			lang, err := lang.GetLanguage(language)
			if err != nil {
				return fmt.Errorf("failed to get language: %w", err)
			}
			filteredLanguages = append(filteredLanguages, lang)
		}
	}

	walker, err := fs.NewSourceWalker(fs.SourceWalkerConfig{}, filteredLanguages)
	if err != nil {
		return fmt.Errorf("failed to create source walker: %w", err)
	}

	treeWalker, err := parser.NewWalkingParser(walker, filteredLanguages)
	if err != nil {
		return fmt.Errorf("failed to create tree walker: %w", err)
	}

	// consume usage evidences
	var usageCallback depsusage.DependencyUsageCallback = func(ctx context.Context, evidence *depsusage.UsageEvidence) error {
		fmt.Println(evidence)
		return nil
	}

	pluginExecutor, err := plugin.NewTreeWalkPluginExecutor(treeWalker, []core.Plugin{
		depsusage.NewDependencyUsagePlugin(usageCallback),
	})

	if err != nil {
		return fmt.Errorf("failed to create plugin executor: %w", err)
	}

	return pluginExecutor.Execute(context.Background(), fileSystem)
}
