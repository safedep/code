package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fs"
	"github.com/safedep/code/lang"
	"github.com/safedep/code/parser"
	"github.com/safedep/code/plugin"
	"github.com/safedep/code/plugin/callgraph"
	"github.com/safedep/dry/log"
	"github.com/safedep/dry/utils"
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
	var callgraphCallback callgraph.CallgraphCallback = func(_ context.Context, cg *callgraph.CallGraph) error {
		if err := cg.Dump(os.Stdout); err != nil {
			return fmt.Errorf("failed to dump call graph: %w", err)
		}

		fmt.Printf("DFS Traversal results for %s:\n", cg.FileName)
		for _, resultItem := range cg.DFS() {
			terminalMessage := ""
			if resultItem.Terminal {
				terminalMessage = " (terminal)"
			}

			callerIdentifierStr := "(callerIdentifier not avl)"
			if resultItem.Text != "" {
				callerIdentifierStr = fmt.Sprintf(
					"(L%d:%d - %s)",
					resultItem.Site.StartLine,
					resultItem.Site.StartColumn,
					utils.TrimWithEllipsis(resultItem.Text, 100, true, 3),
				)
			}

			fmt.Printf(
				"%s %s %s %s\n",
				strings.Repeat(">", resultItem.Depth),
				resultItem.Namespace,
				callerIdentifierStr,
				terminalMessage,
			)
		}

		signatureMatcher, err := callgraph.NewSignatureMatcher(ParsedSignatures)
		if err != nil {
			return fmt.Errorf("failed to create signature matcher: %w", err)
		}

		signatureMatches, err := signatureMatcher.MatchSignatures(cg)
		if err != nil {
			return fmt.Errorf("failed to match signatures: %w", err)
		}

		fmt.Printf("\nSignature matches for %s:\n", cg.FileName)
		for _, match := range signatureMatches {
			fmt.Printf("Match found: %s (%s)\n", match.MatchedSignature.Id, match.MatchedLanguageCode)
			for _, condition := range match.MatchedConditions {
				fmt.Printf("\tCondition: %s - %s\n", condition.Condition.Type, condition.Condition.Value)
				for _, evidence := range condition.Evidences {
					calledByStr := "called by " + evidence.Caller.Namespace
					if evidence.Caller.Kind != "" {
						calledByStr += fmt.Sprintf(" (L%d - L%d)", evidence.Caller.Position.StartLine, evidence.Caller.Position.EndLine)
					}

					calledAtStr := "exact location not available"
					if evidence.Text != "" {
						calledAtStr = fmt.Sprintf("at L%d:%d (%s)", evidence.Site.StartLine, evidence.Site.StartColumn, utils.TrimWithEllipsis(evidence.Text, 100, true, 3))
					}

					fmt.Printf("\t\tEvidence: %s %s %s \n", evidence.Callee.Namespace, calledByStr, calledAtStr)

					argString := ""
					for _, arg := range evidence.Arguments {
						argNamespaces := make([]string, len(arg.Values))
						for i, value := range arg.Values {
							argNamespaces[i] = value.Namespace
						}
						argString += fmt.Sprintf("(%s), ", strings.Join(argNamespaces, ", "))
					}

					fmt.Printf("\t\tArgs (%d): [%s]\n", len(evidence.Arguments), argString)
				}
			}
		}
		fmt.Println()

		return nil
	}

	pluginExecutor := plugin.NewTreeWalkPluginExecutor(treeWalker, []core.Plugin{
		callgraph.NewCallGraphPlugin(callgraphCallback),
	})

	report, err := pluginExecutor.Execute(context.Background(), fileSystem)
	if err != nil {
		return fmt.Errorf("failed to execute plugins: %w", err)
	}

	for _, failure := range report.Failed {
		log.Warnf("%v", failure)
	}
	return nil
}
