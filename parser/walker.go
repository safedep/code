package parser

import (
	"context"
	"fmt"
	"slices"

	"github.com/safedep/code/core"
)

type walkingParser struct {
	languages []core.Language
	opts      []Option
	walker    core.SourceWalker
}

var _ core.TreeWalker = (*walkingParser)(nil)

// NewWalkingParser creates a tree walker that parses each file of the source
// walker. The options apply to the parser, as in NewParser.
func NewWalkingParser(walker core.SourceWalker, languages []core.Language, opts ...Option) (*walkingParser, error) {
	if _, err := LanguagesByExtension(languages); err != nil {
		return nil, fmt.Errorf("failed to create parser: %w", err)
	}

	return &walkingParser{
		languages: slices.Clone(languages),
		opts:      slices.Clone(opts),
		walker:    walker,
	}, nil
}

// Walk parses with a parser of its own and closes the parser when the walk
// ends. A sitter parser is not safe for concurrent use, so two walks do not
// share one.
func (p *walkingParser) Walk(ctx context.Context, fs core.ImportAwareFileSystem, visitor core.TreeVisitor) error {
	parser, err := NewParser(p.languages, p.opts...)
	if err != nil {
		return fmt.Errorf("failed to create parser: %w", err)
	}
	defer parser.Close()

	return p.walker.Walk(ctx, fs, &sourceVisitor{parser: parser, visitor: visitor})
}

type sourceVisitor struct {
	parser  *parserWrapper
	visitor core.TreeVisitor
}

func (v *sourceVisitor) VisitFile(ctx context.Context, f core.File) error {
	parseTree, stage, err := v.parser.parse(ctx, f)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if errorVisitor, ok := v.visitor.(core.FileErrorVisitor); ok {
			return errorVisitor.VisitFileError(ctx, f, stage, err)
		}
		return fmt.Errorf("failed to %s file %s: %w", stage, f.Name(), err)
	}

	return v.visitor.VisitTree(ctx, parseTree)
}
