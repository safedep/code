package parser

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/safedep/code/core"
	sitter "github.com/smacker/go-tree-sitter"
)

// ParseBytes parses data as the given language. It does not look at a
// file name, so a caller can parse a file of any name as any language.
// Node runs a required file of any name as JavaScript, for example.
//
// The parse stops when ctx is done and returns the error of ctx. Pass a
// deadline for hostile input, because some inputs make the parse slow, as
// a long chain of TypeScript assertions read as JavaScript. The tree keeps
// a copy of data, so the caller can use data again.
func ParseBytes(ctx context.Context, language core.Language, data []byte, opts ...Option) (core.ParseTree, error) {
	if language == nil {
		return nil, errors.New("no language to parse with")
	}
	cfg := newConfig(opts)
	if cfg.maxBytes > 0 && len(data) > cfg.maxBytes {
		return nil, fmt.Errorf("%w: %d bytes, limit %d", ErrInputTooLarge, len(data), cfg.maxBytes)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	data = bytes.Clone(data)
	p := sitter.NewParser()
	defer p.Close()
	p.SetLanguage(language.Language())
	tree, err := p.ParseCtx(ctx, nil, data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse: %w", err)
	}

	return &parseTree{
		tree: tree,
		data: &data,
		file: &bytesFile{name: cfg.name, data: data},
		lang: language,
	}, nil
}

// bytesFile is the File of a tree that ParseBytes made.
type bytesFile struct {
	name string
	data []byte
}

func (f *bytesFile) Name() string   { return f.name }
func (f *bytesFile) IsApp() bool    { return true }
func (f *bytesFile) IsImport() bool { return false }
func (f *bytesFile) Reader() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(f.data)), nil
}
