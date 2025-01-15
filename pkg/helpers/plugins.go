package helpers

import (
	"fmt"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fs"
	"github.com/safedep/code/lang"
	"github.com/safedep/code/parser"
)

func SetupBasicPluginContext(filePaths []string, languageCode core.LanguageCode) (core.TreeWalker, core.ImportAwareFileSystem, error) {
	fileSystem, err := fs.NewLocalFileSystem(fs.LocalFileSystemConfig{
		AppDirectories: filePaths,
	})

	if err != nil {
		return nil, nil, fmt.Errorf("failed to create file system: %w", err)
	}

	language, err := lang.GetLanguage(string(languageCode))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get language: %w", err)
	}

	walker, err := fs.NewSourceWalker(fs.SourceWalkerConfig{}, language)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create source walker: %w", err)
	}

	treeWalker, err := parser.NewWalkingParser(walker, language)
	if err != nil {
		return treeWalker, nil, fmt.Errorf("failed to create tree walker: %w", err)
	}

	return treeWalker, fileSystem, nil
}
