package test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fs"
	"github.com/safedep/code/lang"
	"github.com/safedep/code/parser"
)

// SetupBasicPluginContext sets up a basic plugin context for testing plugins.
// The file system enumerates root and keeps only files, given relative to
// root. No files keeps every file. The options apply to the parser of the
// tree walker.
func SetupBasicPluginContext(root string, files []string, languageCodes []core.LanguageCode, opts ...parser.Option) (core.TreeWalker, core.ImportAwareFileSystem, error) {
	excludes, err := excludeOthers(root, files)
	if err != nil {
		return nil, nil, err
	}

	fileSystem, err := fs.NewLocalFileSystem(fs.LocalFileSystemConfig{
		AppDirectories:  []string{root},
		ExcludePatterns: excludes,
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

	treeWalker, err := parser.NewWalkingParser(walker, languages, opts...)
	if err != nil {
		return treeWalker, nil, fmt.Errorf("failed to create tree walker: %w", err)
	}

	return treeWalker, fileSystem, nil
}

// excludeOthers returns a pattern for each file under root that is not one
// of files. A file that is not under root is an error.
func excludeOthers(root string, files []string) ([]*regexp.Regexp, error) {
	if len(files) == 0 {
		return nil, nil
	}

	var excludes []*regexp.Regexp
	found := 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if slices.Contains(files, filepath.ToSlash(rel)) {
			found++
			return nil
		}
		excludes = append(excludes, regexp.MustCompile("^"+regexp.QuoteMeta(path)+"$"))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list %s: %w", root, err)
	}
	if found != len(files) {
		return nil, fmt.Errorf("found %d of the files %v under %s", found, files, root)
	}
	return excludes, nil
}
