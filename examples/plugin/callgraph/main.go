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
	"github.com/safedep/code/plugin/callgraph"
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

	walker, err := fs.NewSourceWalker(fs.SourceWalkerConfig{}, []core.Language{language})
	if err != nil {
		return fmt.Errorf("failed to create source walker: %w", err)
	}

	treeWalker, err := parser.NewWalkingParser(walker, []core.Language{language})
	if err != nil {
		return fmt.Errorf("failed to create tree walker: %w", err)
	}

	// consume callgraph
	var callgraphCallback callgraph.CallgraphCallback = func(cg *callgraph.CallGraph) error {
		cg.PrintAssignmentGraph()
		cg.PrintCallGraph()

		fmt.Println("DFS Traversal results:")
		for _, resultItem := range cg.DFS() {
			terminalMessage := ""
			if resultItem.Terminal {
				terminalMessage = " (terminal)"
			}
			fmt.Printf("%s %s%s\n", strings.Repeat(">", resultItem.Depth), resultItem.Namespace, terminalMessage)
		}

		signatureMatches, err := cg.MatchSignatures(callgraph.ParsedSignatures.Signatures)
		if err != nil {
			return fmt.Errorf("failed to match signatures: %w", err)
		}

		fmt.Println("\nSignature matches:")
		for _, match := range signatureMatches {
			fmt.Printf("Match found: %s (%s)\n", match.MatchedSignature.ID, match.MatchedLanguageCode)
		}
		return nil
	}

	pluginExecutor, err := plugin.NewTreeWalkPluginExecutor(treeWalker, []core.Plugin{
		callgraph.NewCallGraphPlugin(callgraphCallback),
	})

	if err != nil {
		return fmt.Errorf("failed to create plugin executor: %w", err)
	}

	return pluginExecutor.Execute(context.Background(), fileSystem)
}
