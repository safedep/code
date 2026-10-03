package parser

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/lang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// trackedFile is a source file whose reader records that it was closed.
type trackedFile struct {
	name     string
	content  string
	closeErr error
	closed   bool
}

func (f *trackedFile) Name() string   { return f.name }
func (f *trackedFile) IsApp() bool    { return true }
func (f *trackedFile) IsImport() bool { return false }
func (f *trackedFile) Reader() (io.ReadCloser, error) {
	return &trackedReader{Reader: strings.NewReader(f.content), file: f}, nil
}

type trackedReader struct {
	io.Reader
	file *trackedFile
}

func (r *trackedReader) Close() error {
	r.file.closed = true
	return r.file.closeErr
}

func TestParseClosesTheFile(t *testing.T) {
	python, err := lang.NewPythonLanguage()
	require.NoError(t, err)
	p, err := NewParser([]core.Language{python})
	require.NoError(t, err)

	f := &trackedFile{name: "app.py", content: "import os\n"}
	tree, err := p.Parse(context.Background(), f)
	require.NoError(t, err)
	assert.NotNil(t, tree)
	assert.True(t, f.closed, "an open reader keeps the file in use")

	failing := &trackedFile{name: "app.py", content: "import os\n", closeErr: errors.New("close failed")}
	_, err = p.Parse(context.Background(), failing)
	assert.ErrorContains(t, err, "close failed")
}
