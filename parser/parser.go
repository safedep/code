package parser

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/safedep/code/core"
	sitter "github.com/smacker/go-tree-sitter"
)

// parserWrapper parses the files of the languages that it was built with.
// It is not safe for concurrent use, because a sitter.Parser is not.
type parserWrapper struct {
	byExtension map[string]*grammar
	maxBytes    int
}

// grammar is a language with the sitter parser that parses its files.
type grammar struct {
	language core.Language
	parser   *sitter.Parser
}

type parseTree struct {
	tree *sitter.Tree
	data *[]byte
	file core.File
	lang core.Language
}

var _ core.Parser = (*parserWrapper)(nil)
var _ core.ParseTree = (*parseTree)(nil)

// NewParser creates a parser for the given languages. It picks the language
// of a file by the file extension. A language that implements
// core.GrammarSelector picks the language of each of its extensions. The
// caller closes the parser.
func NewParser(languages []core.Language, opts ...Option) (*parserWrapper, error) {
	cfg := newConfig(opts)
	fileLanguages, err := LanguagesByExtension(languages)
	if err != nil {
		return nil, err
	}

	byExtension := make(map[string]*grammar, len(fileLanguages))
	byLanguage := make(map[core.Language]*grammar)
	for ext, language := range fileLanguages {
		g, ok := byLanguage[language]
		if !ok {
			parser := sitter.NewParser()
			parser.SetLanguage(language.Language())
			g = &grammar{language: language, parser: parser}
			byLanguage[language] = g
		}
		byExtension[ext] = g
	}

	return &parserWrapper{
		byExtension: byExtension,
		maxBytes:    cfg.maxBytes,
	}, nil
}

// LanguagesByExtension returns the language that parses each extension. It
// fails when two languages claim one extension.
func LanguagesByExtension(languages []core.Language) (map[string]core.Language, error) {
	byExtension := make(map[string]core.Language)
	for _, language := range languages {
		for _, ext := range language.Meta().SourceFileExtensions {
			if _, taken := byExtension[ext]; taken {
				return nil, fmt.Errorf("two languages parse the extension %s", ext)
			}

			fileLanguage := language
			if selector, ok := language.(core.GrammarSelector); ok {
				fileLanguage = selector.LanguageForExtension(ext)
			}
			byExtension[ext] = fileLanguage
		}
	}
	return byExtension, nil
}

// Close frees the sitter parsers. The trees that the parser made stay
// valid. The parser must not parse after Close.
func (p *parserWrapper) Close() {
	for _, g := range p.byExtension {
		g.parser.Close()
	}
}

// Parse parses the file with the language of its extension. It returns an
// error that wraps core.ErrNoLanguage when no language parses the file, and
// ErrInputTooLarge for a file over the size limit.
func (p *parserWrapper) Parse(ctx context.Context, file core.File) (core.ParseTree, error) {
	tree, _, err := p.parse(ctx, file)
	return tree, err
}

// parse is Parse that also names the stage of a failure.
func (p *parserWrapper) parse(ctx context.Context, file core.File) (core.ParseTree, core.Stage, error) {
	if err := ctx.Err(); err != nil {
		return nil, core.StageRead, err
	}

	g, ok := p.byExtension[filepath.Ext(file.Name())]
	if !ok {
		return nil, core.StageParse, fmt.Errorf("%w: %s", core.ErrNoLanguage, file.Name())
	}

	data, err := readAll(file, p.maxBytes)
	if err != nil {
		return nil, core.StageRead, err
	}
	if p.maxBytes > 0 && len(data) > p.maxBytes {
		return nil, core.StageParse, fmt.Errorf("%w: %s is over %d bytes", ErrInputTooLarge, file.Name(), p.maxBytes)
	}

	tree, err := g.parser.ParseCtx(ctx, nil, data)
	if err != nil {
		return nil, core.StageParse, fmt.Errorf("failed to parse file: %w", err)
	}

	// We must guarantee that none of the pointers are nil
	return &parseTree{
		tree: tree,
		data: &data,
		file: file,
		lang: g.language,
	}, "", nil
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

func (t *parseTree) Close() {
	t.tree.Close()
}

// readAll reads the file and closes its reader. An open reader keeps the
// file in use, and on Windows a file in use cannot be deleted. It reads one
// byte over maxBytes at most, so a large file does not fill the memory, and
// the caller can see that the file is over the limit.
func readAll(file core.File, maxBytes int) (data []byte, err error) {
	r, err := file.Reader()
	if err != nil {
		return nil, fmt.Errorf("failed to get reader for file: %w", err)
	}
	defer func() {
		if closeErr := r.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close file: %w", closeErr)
		}
	}()

	var src io.Reader = r
	if maxBytes > 0 {
		src = io.LimitReader(r, int64(maxBytes)+1)
	}
	data, err = io.ReadAll(src)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	return data, nil
}
