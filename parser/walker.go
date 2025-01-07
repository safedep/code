package parser

import (
	"context"
	"fmt"

	"github.com/safedep/code/core"
)

type WalkingParser struct {
	parser   core.Parser
	language core.Language
	walker   core.SourceWalker
}

var _ core.TreeWalker = (*WalkingParser)(nil)

func NewWalkingParser(walker core.SourceWalker, language core.Language) (*WalkingParser, error) {
	parser, err := NewParser(language)
	if err != nil {
		return nil, fmt.Errorf("failed to create parser: %w", err)
	}

	return &WalkingParser{
		parser:   parser,
		language: language,
		walker:   walker,
	}, nil
}

type sourceVisitor struct {
	parser  core.Parser
	visitor core.TreeVisitor
}

func (v *sourceVisitor) VisitFile(lang core.Language, f core.File) error {
	parseTree, err := v.parser.Parse(context.Background(), f)
	if err != nil {
		return fmt.Errorf("failed to parse file: %w", err)
	}

	return v.visitor.VisitTree(lang, parseTree)
}

func (p *WalkingParser) Walk(ctx context.Context, fs core.ImportAwareFileSystem, visitor core.TreeVisitor) error {
	return p.walker.Walk(ctx, fs, &sourceVisitor{parser: p.parser, visitor: visitor})
}
