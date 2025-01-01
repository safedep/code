package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fs"
	"github.com/safedep/code/lang"
	"github.com/safedep/code/parser"
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

type treeVisitor struct{}

func (v *treeVisitor) VisitTree(language core.Language, tree core.ParseTree) error {
	file, err := tree.File()
	if err != nil {
		return fmt.Errorf("failed to get file: %w", err)
	}

	log.Infof("Visiting tree for language: %s file: %s",
		language.Meta().Name, file.Name())

	// Example of how consumers of ISP can check if a language resolver supports
	// a specific interface.
	if or, ok := language.Resolvers().(core.ObjectOrientedLanguageResolvers); ok {
		fmt.Printf("Language resolver supports OO: %v\n", or)
	}

	imports, err := language.Resolvers().ResolveImports(tree)
	if err != nil {
		return fmt.Errorf("failed to resolve imports: %w", err)
	}

	for _, imp := range imports {
		log.Infof("Import: %s", imp.String())
	}

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

	err = treeWalker.Walk(context.Background(), fileSystem, &treeVisitor{})
	if err != nil {
		return fmt.Errorf("failed to walk parse trees: %w", err)
	}

	return nil
}
