package parser

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/safedep/code/core"
	"github.com/safedep/code/lang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func javascript(t testing.TB) core.Language {
	l, err := lang.NewJavascriptLanguage()
	require.NoError(t, err)
	return l
}

func TestParseBytes(t *testing.T) {
	cases := []struct {
		name    string
		data    string
		opts    []Option
		wantErr error
		root    string
	}{
		{name: "parses with no file name", data: "require('x')", root: "program"},
		{name: "ignores the name for the language", data: "a()", opts: []Option{WithFileName("logo.svg")}, root: "program"},
		{name: "rejects input over the limit", data: "a()", opts: []Option{WithMaxBytes(2)}, wantErr: ErrInputTooLarge},
		{name: "no limit when negative", data: strings.Repeat("a;", 10), opts: []Option{WithMaxBytes(-1)}, root: "program"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree, err := ParseBytes(context.Background(), javascript(t), []byte(tc.data), tc.opts...)
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.root, tree.Tree().RootNode().Type())

			data, err := tree.Data()
			require.NoError(t, err)
			assert.Equal(t, tc.data, string(*data))

			l, err := tree.Language()
			require.NoError(t, err)
			assert.Equal(t, core.LanguageCodeJavascript, l.Meta().Code)
		})
	}
}

func TestParseBytesFile(t *testing.T) {
	tree, err := ParseBytes(context.Background(), javascript(t), []byte("a()"), WithFileName("dir/logo.svg"))
	require.NoError(t, err)

	f, err := tree.File()
	require.NoError(t, err)
	assert.Equal(t, "dir/logo.svg", f.Name())
	assert.True(t, f.IsApp())

	r, err := f.Reader()
	require.NoError(t, err)
	got, err := io.ReadAll(r)
	require.NoError(t, err)
	require.NoError(t, r.Close())
	assert.Equal(t, "a()", string(got))
}

func TestParseBytesStopsOnDoneContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ParseBytes(ctx, javascript(t), []byte("a()"))
	assert.ErrorIs(t, err, context.Canceled)
}

func TestParseBytesStopsAtDeadline(t *testing.T) {
	if testing.Short() {
		t.Skip("parses a large input")
	}
	data := []byte(strings.Repeat("a(b, [c, {d: e}]);\n", 1_500_000))
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := ParseBytes(ctx, javascript(t), data, WithMaxBytes(-1))
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), 5*time.Second)
}

func TestParseBytesNoLanguage(t *testing.T) {
	_, err := ParseBytes(context.Background(), nil, []byte("a"))
	assert.Error(t, err)
}

func FuzzParseBytes(f *testing.F) {
	for _, s := range []string{"", "a()", "(0, eval)('x')", "<svg/>", "x\x00y", "`${a}`", "((((((a"} {
		f.Add([]byte(s))
	}
	l := javascript(f)
	f.Fuzz(func(t *testing.T, data []byte) {
		tree, err := ParseBytes(context.Background(), l, data)
		require.NoError(t, err)
		root := tree.Tree().RootNode()
		assert.LessOrEqual(t, int(root.EndByte()), len(data))
	})
}
