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

// DefaultMaxBytes is the largest input that ParseBytes parses when the
// caller sets no limit. A caller that reads larger files sets its own limit
// and a deadline on ctx, because some inputs make the parse slow.
const DefaultMaxBytes = 4 << 20

// ErrInputTooLarge is the error of an input over the size limit.
var ErrInputTooLarge = errors.New("input is larger than the parse limit")

type bytesConfig struct {
	name     string
	maxBytes int
}

// BytesOption changes how ParseBytes parses.
type BytesOption func(*bytesConfig)

// WithFileName sets the name that the File of the tree reports. A resolver
// that keys its results by file name, such as the call graph, reads it. The
// name does not select the language.
func WithFileName(name string) BytesOption {
	return func(c *bytesConfig) { c.name = name }
}

// WithMaxBytes sets the size limit of the input. Zero selects
// DefaultMaxBytes. A negative value removes the limit.
func WithMaxBytes(n int) BytesOption {
	return func(c *bytesConfig) { c.maxBytes = n }
}

// ParseBytes parses data as the given language. It does not look at a
// file name, so a caller can parse a file of any name as any language.
// Node runs a required file of any name as JavaScript, for example.
//
// The parse stops when ctx is done and returns the error of ctx. Pass a
// deadline for hostile input, because some inputs make the parse slow, as
// a long chain of TypeScript assertions read as JavaScript. The tree keeps
// a copy of data, so the caller can use data again.
func ParseBytes(ctx context.Context, language core.Language, data []byte, opts ...BytesOption) (core.ParseTree, error) {
	if language == nil {
		return nil, errors.New("no language to parse with")
	}
	var cfg bytesConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.maxBytes == 0 {
		cfg.maxBytes = DefaultMaxBytes
	}
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
