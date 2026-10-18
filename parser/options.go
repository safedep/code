package parser

import "errors"

// DefaultMaxBytes is the largest input that a parser parses when the caller
// sets no limit. A caller that reads larger files sets its own limit and a
// deadline on ctx, because some inputs make the parse slow.
const DefaultMaxBytes = 4 << 20

// ErrInputTooLarge is the error of an input over the size limit.
var ErrInputTooLarge = errors.New("input is larger than the parse limit")

type config struct {
	name     string
	maxBytes int
}

// Option changes how a parser parses.
type Option func(*config)

// WithFileName sets the name that the File of the tree reports. A resolver
// that keys its results by file name, such as the call graph, reads it. The
// name does not select the language. Only ParseBytes reads it.
func WithFileName(name string) Option {
	return func(c *config) { c.name = name }
}

// WithMaxBytes sets the size limit of the input. Zero selects
// DefaultMaxBytes. A negative value removes the limit.
func WithMaxBytes(n int) Option {
	return func(c *config) { c.maxBytes = n }
}

func newConfig(opts []Option) config {
	var cfg config
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.maxBytes == 0 {
		cfg.maxBytes = DefaultMaxBytes
	}
	return cfg
}
