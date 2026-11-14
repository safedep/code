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
	tree *sitter.Tree
	data *[]byte
	file core.File
	lang core.Language
}

var _ core.Parser = (*parserWrapper)(nil)
var _ core.ParseTree = (*parseTree)(nil)

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

// Source returns the language that parses data as the content of the file
// of the name, and the bytes that its grammar parses. language is the
// language of the extension of the name. A core.SourceSelector picks both.
func Source(language core.Language, name string, data []byte) (core.Language, []byte) {
	if selector, ok := language.(core.SourceSelector); ok {
		return selector.Source(filepath.Ext(name), data)
	}
	return language, data
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

	tree, err := p.parseContent(ctx, language, file, data)
	return tree, core.StageParse, err
}

// ParseContent parses data as the content of file, with the language of
// the file extension. A caller that already read the file uses it, so the
// file is not read again. The tree keeps data, so the caller must not
// change data after the call.
func (p *parserWrapper) ParseContent(ctx context.Context, file core.File, data []byte) (core.ParseTree, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	language, err := p.languageOf(file)
	if err != nil {
		return nil, err
	}
	return p.parseContent(ctx, language, file, data)
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

// parseContent parses the source of data. The tree keeps data, not the
// source, so the tree data is the bytes of the file. The source keeps the
// offsets of data, so a node has the same content in both.
func (p *parserWrapper) parseContent(ctx context.Context, language core.Language, file core.File, data []byte) (core.ParseTree, error) {
	if p.maxBytes > 0 && len(data) > p.maxBytes {
		return nil, fmt.Errorf("%w: %s is over %d bytes", ErrInputTooLarge, file.Name(), p.maxBytes)
	}

	language, source := Source(language, file.Name(), data)
	sp := p.sitterParser(language)
	tree, err := sp.ParseCtx(ctx, nil, source)
	if err != nil {
		// A sitter parser resumes a cancelled parse on its next call.
		// The reset makes the next file parse from its start.
		sp.Reset()
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

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
