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
	byExtension map[string]core.Language
	// parsers holds the sitter parser of each language that parsed a file.
	parsers  map[core.Language]*sitter.Parser
	maxBytes int
}

type parseTree struct {
	tree   *sitter.Tree
	data   *[]byte
	file   core.File
	lang   core.Language
	markup *core.Markup
}

var _ core.Parser = (*parserWrapper)(nil)
var _ core.ParseTree = (*parseTree)(nil)
var _ core.MarkupTree = (*parseTree)(nil)

// NewParser creates a parser for the given languages. It picks the language
// of a file by the file extension. A language that implements
// core.GrammarSelector picks the language of each of its extensions, and a
// language that implements core.SourceSelector picks the language of the
// content. The caller closes the parser.
func NewParser(languages []core.Language, opts ...Option) (*parserWrapper, error) {
	cfg := newConfig(opts)
	byExtension, err := LanguagesByExtension(languages)
	if err != nil {
		return nil, err
	}

	return &parserWrapper{
		byExtension: byExtension,
		parsers:     map[core.Language]*sitter.Parser{},
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
	for _, sp := range p.parsers {
		sp.Close()
	}
}

// Source returns the source of data as the content of the file of the
// name. language is the language of the extension of the name. A
// core.SourceSelector picks the source.
func Source(language core.Language, name string, data []byte) core.Source {
	if selector, ok := language.(core.SourceSelector); ok {
		return selector.Source(filepath.Ext(name), data)
	}
	return core.Source{Language: language, Code: data}
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

	language, err := p.languageOf(file)
	if err != nil {
		return nil, core.StageParse, err
	}

	data, err := readAll(file, p.maxBytes)
	if err != nil {
		return nil, core.StageRead, err
	}

	tree, err := p.ParseSource(ctx, file, data, Source(language, file.Name(), data))
	return tree, core.StageParse, err
}

// ParseContent parses data as the content of file, with the language of
// the file extension. A caller that already read the file uses it, so the
// file is not read again. The tree keeps data, so the caller must not
// change data after the call.
func (p *parserWrapper) ParseContent(ctx context.Context, file core.File, data []byte) (core.ParseTree, error) {
	language, err := p.languageOf(file)
	if err != nil {
		return nil, err
	}
	return p.ParseSource(ctx, file, data, Source(language, file.Name(), data))
}

func (p *parserWrapper) languageOf(file core.File) (core.Language, error) {
	language, ok := p.byExtension[filepath.Ext(file.Name())]
	if !ok {
		return nil, fmt.Errorf("%w: %s", core.ErrNoLanguage, file.Name())
	}
	return language, nil
}

// sitterParser returns the sitter parser of the language, and makes it at
// the first file of the language.
func (p *parserWrapper) sitterParser(language core.Language) *sitter.Parser {
	sp, ok := p.parsers[language]
	if !ok {
		sp = sitter.NewParser()
		sp.SetLanguage(language.Language())
		p.parsers[language] = sp
	}
	return sp
}

// ParseSource parses the source of data that Source gave for file. A
// caller that already picked the source uses it, so the parse does not pick
// it again. The tree keeps data, not the code of the source, so the tree
// data is the bytes of the file. The code keeps the offsets of data, so a
// node has the same content in both. The caller must not change data after
// the call.
func (p *parserWrapper) ParseSource(ctx context.Context, file core.File, data []byte, source core.Source) (core.ParseTree, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p.maxBytes > 0 && len(data) > p.maxBytes {
		return nil, fmt.Errorf("%w: %s is over %d bytes", ErrInputTooLarge, file.Name(), p.maxBytes)
	}

	sp := p.sitterParser(source.Language)
	tree, err := sp.ParseCtx(ctx, nil, source.Code)
	if err != nil {
		// A sitter parser resumes a cancelled parse on its next call.
		// The reset makes the next file parse from its start.
		sp.Reset()
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

	return &parseTree{
		tree:   tree,
		data:   &data,
		file:   file,
		lang:   source.Language,
		markup: source.Markup,
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

func (t *parseTree) Markup() *core.Markup {
	return t.markup
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
