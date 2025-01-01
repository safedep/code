package parser

import (
	"context"
	"fmt"
	"io"

	"github.com/safedep/code/core"
	sitter "github.com/smacker/go-tree-sitter"
)

// Parser wraps TreeSitter parser for a language
// to provide common concerns
type parserWrapper struct {
	lang   core.Language
	parser *sitter.Parser
}

type parseTree struct {
	tree *sitter.Tree
	data []byte
	file core.File
	lang core.Language
}

var _ core.Parser = (*parserWrapper)(nil)
var _ core.ParseTree = (*parseTree)(nil)

func NewParser(lang core.Language) (*parserWrapper, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(lang.Language())

	return &parserWrapper{
		lang:   lang,
		parser: parser,
	}, nil
}

func (p *parserWrapper) Parse(ctx context.Context, file core.File) (core.ParseTree, error) {
	r, err := file.Reader()
	if err != nil {
		return nil, fmt.Errorf("failed to get reader for file: %w", err)
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	tree, err := p.parser.ParseCtx(ctx, nil, data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

	return &parseTree{
		tree: tree,
		data: data,
		file: file,
		lang: p.lang,
	}, nil
}

func (t *parseTree) Tree() *sitter.Tree {
	return t.tree
}

func (t *parseTree) Data() ([]byte, error) {
	return t.data, nil
}

func (t *parseTree) File() (core.File, error) {
	return t.file, nil
}

func (t *parseTree) Language() (core.Language, error) {
	return t.lang, nil
}
