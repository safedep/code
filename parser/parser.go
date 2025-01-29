package parser

import (
	"context"
	"fmt"
	"io"

	"github.com/safedep/code/core"
	"github.com/safedep/code/lang"
	sitter "github.com/smacker/go-tree-sitter"
)

// Parser wraps TreeSitter parser for a language
// to provide common concerns
type parserWrapper struct {
}

type parseTree struct {
	tree *sitter.Tree
	data *[]byte
	file core.File
	lang core.Language
}

var _ core.Parser = (*parserWrapper)(nil)
var _ core.ParseTree = (*parseTree)(nil)

func NewParser() (*parserWrapper, error) {
	return &parserWrapper{}, nil
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

	language, exists := lang.ResolveLanguageFromPath(file.Name())
	if !exists {
		return nil, fmt.Errorf("failed to resolve language from file path")
	}

	parser := sitter.NewParser()
	parser.SetLanguage(language.Language())

	tree, err := parser.ParseCtx(ctx, nil, data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

	// We must guarantee that none of the pointers are nil
	return &parseTree{
		tree: tree,
		data: &data,
		file: file,
		lang: language,
	}, nil
}

func (t *parseTree) Tree() *sitter.Tree {
	return t.tree
}

func (t *parseTree) Data() (*[]byte, error) {
	return t.data, nil
}

func (t *parseTree) File() (core.File, error) {
	return t.file, nil
}

func (t *parseTree) Language() (core.Language, error) {
	return t.lang, nil
}
