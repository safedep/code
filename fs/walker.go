package fs

import (
	"context"
	"fmt"
	"strings"

	"github.com/safedep/code/core"
)

type SourceWalkerConfig struct {
	IncludeImports bool
}

type sourceWalker struct {
	langs  []core.Language
	config SourceWalkerConfig
}

var _ core.SourceWalker = (*sourceWalker)(nil)

// NewSourceWalker creates a new source walker that
// can walk the source files in a file system based on
// language specific rules.
func NewSourceWalker(config SourceWalkerConfig, langs []core.Language) (*sourceWalker, error) {
	return &sourceWalker{
		langs:  langs,
		config: config,
	}, nil
}

func (s *sourceWalker) Walk(ctx context.Context, fs core.ImportAwareFileSystem, visitor core.SourceVisitor) error {
	enumFunc := func(f core.File) error {
		if !s.validSourceFile(f) {
			return nil
		}

		return visitor.VisitFile(f)
	}

	err := fs.EnumerateApp(ctx, enumFunc)
	if err != nil {
		return fmt.Errorf("failed to walk app files: %w", err)
	}

	if s.config.IncludeImports {
		err := fs.EnumerateImports(ctx, enumFunc)
		if err != nil {
			return fmt.Errorf("failed to walk import files: %w", err)
		}
	}

	return nil
}

func (s *sourceWalker) validSourceFile(f core.File) bool {
	for _, lang := range s.langs {
		for _, ext := range lang.Meta().SourceFileExtensions {
			if strings.HasSuffix(f.Name(), ext) {
				return true
			}
		}
	}
	return false
}
