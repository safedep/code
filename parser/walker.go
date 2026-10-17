package parser

import (
	"context"
	"fmt"

	"github.com/safedep/code/core"
)

type walkingParser struct {
	parser core.Parser
	walker core.SourceWalker
}

var _ core.TreeWalker = (*walkingParser)(nil)

// NewWalkingParser creates a tree walker that parses each file of the source
// walker. The options apply to the parser, as in NewParser.
func NewWalkingParser(walker core.SourceWalker, languages []core.Language, opts ...Option) (*walkingParser, error) {
	parser, err := NewParser(languages, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create parser: %w", err)
	}

	return &walkingParser{
		parser: parser,
		walker: walker,
	}, nil
}

func (p *walkingParser) Walk(ctx context.Context, fs core.ImportAwareFileSystem, visitor core.TreeVisitor) error {
	return p.walker.Walk(ctx, fs, &sourceVisitor{parser: p.parser, visitor: visitor})
}

type sourceVisitor struct {
	parser  core.Parser
	visitor core.TreeVisitor
}

func (v *sourceVisitor) VisitFile(f core.File) error {
	parseTree, err := v.parser.Parse(context.Background(), f)
	if err != nil {
		return fmt.Errorf("failed to parse file: %w", err)
	}

	return v.visitor.VisitTree(parseTree)
}
