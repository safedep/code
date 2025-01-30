package test

import (
	"fmt"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fs"
	"github.com/safedep/code/lang"
	"github.com/safedep/code/parser"
)

// SetupBasicPluginContext sets up a basic plugin context for testing plugins.
func SetupBasicPluginContext(filePaths []string, languageCodes []core.LanguageCode) (core.TreeWalker, core.ImportAwareFileSystem, error) {
	fileSystem, err := fs.NewLocalFileSystem(fs.LocalFileSystemConfig{
		AppDirectories: filePaths,
	})

	if err != nil {
		return nil, nil, fmt.Errorf("failed to create file system: %w", err)
	}

	var languages []core.Language
	for _, code := range languageCodes {
		language, err := lang.GetLanguage(string(code))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get language: %w", err)
		}
		languages = append(languages, language)
	}

	walker, err := fs.NewSourceWalker(fs.SourceWalkerConfig{}, languages)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create source walker: %w", err)
	}

	treeWalker, err := parser.NewWalkingParser(walker, languages)
	if err != nil {
		return treeWalker, nil, fmt.Errorf("failed to create tree walker: %w", err)
	}

	return treeWalker, fileSystem, nil
}
