package parser

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fs"
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
func (f *trackedFile) Path() string   { return f.name }
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
	t.Cleanup(p.Close)

	f := &trackedFile{name: "app.py", content: "import os\n"}
	tree, err := p.Parse(context.Background(), f)
	require.NoError(t, err)
	assert.NotNil(t, tree)
	assert.True(t, f.closed, "an open reader keeps the file in use")

	failing := &trackedFile{name: "app.py", content: "import os\n", closeErr: errors.New("close failed")}
	_, err = p.Parse(context.Background(), failing)
	assert.ErrorContains(t, err, "close failed")
}

func typescriptLanguages(t *testing.T) (withJSX, withoutJSX core.Language) {
	withJSX, err := lang.NewTypescriptLanguage()
	require.NoError(t, err)
	withoutJSX, err = lang.NewTypescriptWithoutJSXLanguage()
	require.NoError(t, err)
	return withJSX, withoutJSX
}

func TestParseLimit(t *testing.T) {
	python, err := lang.NewPythonLanguage()
	require.NoError(t, err)
	source := strings.Repeat("a = 1\n", 10)

	cases := []struct {
		name    string
		opts    []Option
		wantErr error
	}{
		{name: "default limit", opts: nil},
		{name: "over the limit", opts: []Option{WithMaxBytes(len(source) - 1)}, wantErr: ErrInputTooLarge},
		{name: "at the limit", opts: []Option{WithMaxBytes(len(source))}},
		{name: "no limit when negative", opts: []Option{WithMaxBytes(-1)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := NewParser([]core.Language{python}, tc.opts...)
			require.NoError(t, err)
			t.Cleanup(p.Close)

			f := &trackedFile{name: "app.py", content: source}
			tree, err := p.Parse(context.Background(), f)
			assert.True(t, f.closed)
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			defer tree.Close()
			assert.False(t, tree.Tree().RootNode().HasError())
		})
	}
}

func TestParseRejectsAFileWithNoLanguage(t *testing.T) {
	python, err := lang.NewPythonLanguage()
	require.NoError(t, err)
	p, err := NewParser([]core.Language{python})
	require.NoError(t, err)
	t.Cleanup(p.Close)

	f := &trackedFile{name: "app.js", content: "a()"}
	_, err = p.Parse(context.Background(), f)
	assert.ErrorIs(t, err, core.ErrNoLanguage)
	assert.False(t, f.closed, "the parser does not read a file that it cannot parse")
}

func TestNewParserRejectsTwoLanguagesForOneExtension(t *testing.T) {
	withJSX, withoutJSX := typescriptLanguages(t)
	_, err := NewParser([]core.Language{withJSX, withoutJSX})
	assert.ErrorContains(t, err, "two languages parse the extension .ts")
}

func TestParsePicksTheGrammarOfTheExtension(t *testing.T) {
	withJSX, withoutJSX := typescriptLanguages(t)
	const assertion = "const x = <string>y;\n"
	const element = "const e = <div>{x}</div>;\n"

	cases := []struct {
		name      string
		language  core.Language
		file      string
		content   string
		wantError bool
	}{
		{name: "ts gets the grammar without JSX", language: withJSX, file: "a.ts", content: assertion},
		{name: "mts gets the grammar without JSX", language: withJSX, file: "a.mts", content: assertion},
		{name: "cts gets the grammar without JSX", language: withJSX, file: "a.cts", content: assertion},
		{name: "tsx gets the TSX grammar", language: withJSX, file: "a.tsx", content: element},
		{name: "the language without JSX parses tsx without JSX", language: withoutJSX, file: "a.tsx", content: element, wantError: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := NewParser([]core.Language{tc.language})
			require.NoError(t, err)
			t.Cleanup(p.Close)

			tree, err := p.Parse(context.Background(), &trackedFile{name: tc.file, content: tc.content})
			require.NoError(t, err)
			defer tree.Close()
			assert.Equal(t, tc.wantError, tree.Tree().RootNode().HasError())

			treeLanguage, err := tree.Language()
			require.NoError(t, err)
			assert.Equal(t, core.LanguageCodeTypescript, treeLanguage.Meta().Code)
		})
	}
}

func TestTreeReportsTheLanguageThatParsedIt(t *testing.T) {
	_, withoutJSX := typescriptLanguages(t)
	p, err := NewParser([]core.Language{withoutJSX})
	require.NoError(t, err)
	t.Cleanup(p.Close)

	tree, err := p.Parse(context.Background(), &trackedFile{name: "a.ts", content: "let a = 1;"})
	require.NoError(t, err)
	defer tree.Close()

	treeLanguage, err := tree.Language()
	require.NoError(t, err)
	assert.Same(t, withoutJSX, treeLanguage)
}

func TestWalkingParserParsesTypescriptWithoutJSX(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cast.ts"), []byte("const x = <string>y;\nfunction f() {}\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "view.tsx"), []byte("const e = <div>{x}</div>;\n"), 0o600))

	withJSX, _ := typescriptLanguages(t)
	languages := []core.Language{withJSX}
	fileSystem, err := fs.NewLocalFileSystem(fs.LocalFileSystemConfig{AppDirectories: []string{dir}})
	require.NoError(t, err)
	sourceWalker, err := fs.NewSourceWalker(fs.SourceWalkerConfig{}, languages)
	require.NoError(t, err)
	walker, err := NewWalkingParser(sourceWalker, languages)
	require.NoError(t, err)

	visitor := &errorRecorder{hasError: map[string]bool{}}
	require.NoError(t, walker.Walk(context.Background(), fileSystem, visitor))
	assert.Equal(t, map[string]bool{"cast.ts": false, "view.tsx": false}, visitor.hasError)
}

type errorRecorder struct {
	hasError map[string]bool
}

func (r *errorRecorder) VisitTree(_ context.Context, tree core.ParseTree) error {
	file, err := tree.File()
	if err != nil {
		return err
	}
	r.hasError[filepath.Base(file.Name())] = tree.Tree().RootNode().HasError()
	return nil
}

func TestCloseTwice(t *testing.T) {
	tree, err := ParseBytes(context.Background(), javascript(t), []byte("a()"))
	require.NoError(t, err)
	assert.NotPanics(t, func() {
		tree.Close()
		tree.Close()
	})
}

// unreadableFile is a file whose reader fails.
type unreadableFile struct{ trackedFile }

func (f *unreadableFile) Reader() (io.ReadCloser, error) { return nil, errors.New("permission denied") }

// filesWalker is a source walker that visits the given files.
type filesWalker []core.File

func (w filesWalker) Walk(ctx context.Context, _ core.ImportAwareFileSystem, visitor core.SourceVisitor) error {
	for _, f := range w {
		if err := visitor.VisitFile(ctx, f); err != nil {
			return err
		}
	}
	return nil
}

type fileErrorRecorder struct {
	errorRecorder
	stages map[string]core.Stage
	errs   map[string]error
}

func (r *fileErrorRecorder) VisitFileError(_ context.Context, file core.File, stage core.Stage, err error) error {
	r.stages[file.Name()] = stage
	r.errs[file.Name()] = err
	return nil
}

func TestWalkingParserReportsTheStageOfAFailure(t *testing.T) {
	python, err := lang.NewPythonLanguage()
	require.NoError(t, err)
	files := filesWalker{
		&unreadableFile{trackedFile{name: "locked.py"}},
		&trackedFile{name: "notes.txt", content: "text"},
		&trackedFile{name: "large.py", content: strings.Repeat("a = 1\n", 10)},
		&trackedFile{name: "good.py", content: "import os\n"},
	}

	walker, err := NewWalkingParser(files, []core.Language{python}, WithMaxBytes(32))
	require.NoError(t, err)
	visitor := &fileErrorRecorder{
		errorRecorder: errorRecorder{hasError: map[string]bool{}},
		stages:        map[string]core.Stage{},
		errs:          map[string]error{},
	}
	require.NoError(t, walker.Walk(t.Context(), nil, visitor))

	assert.Equal(t, map[string]bool{"good.py": false}, visitor.hasError)
	assert.Equal(t, map[string]core.Stage{
		"locked.py": core.StageRead,
		"notes.txt": core.StageParse,
		"large.py":  core.StageParse,
	}, visitor.stages)
	assert.ErrorContains(t, visitor.errs["locked.py"], "permission denied")
	assert.ErrorIs(t, visitor.errs["notes.txt"], core.ErrNoLanguage)
	assert.ErrorIs(t, visitor.errs["large.py"], ErrInputTooLarge)

	stopping, err := NewWalkingParser(files, []core.Language{python})
	require.NoError(t, err)
	err = stopping.Walk(t.Context(), nil, &errorRecorder{hasError: map[string]bool{}})
	assert.ErrorContains(t, err, "failed to read file locked.py", "a visitor without VisitFileError stops the walk")
}

func TestCloseKeepsTheTreesOfTheParser(t *testing.T) {
	withJSX, _ := typescriptLanguages(t)
	p, err := NewParser([]core.Language{withJSX})
	require.NoError(t, err)

	tree, err := p.Parse(t.Context(), &trackedFile{name: "app.ts", content: "run()\n"})
	require.NoError(t, err)
	defer tree.Close()

	assert.NotPanics(t, func() {
		p.Close()
		p.Close()
	})
	assert.Equal(t, "program", tree.Tree().RootNode().Type())
}

func TestWalkingParserWalksMoreThanOnce(t *testing.T) {
	python, err := lang.NewPythonLanguage()
	require.NoError(t, err)
	files := filesWalker{&trackedFile{name: "good.py", content: "import os\n"}}

	walker, err := NewWalkingParser(files, []core.Language{python})
	require.NoError(t, err)
	for range 2 {
		visitor := &errorRecorder{hasError: map[string]bool{}}
		require.NoError(t, walker.Walk(t.Context(), nil, visitor))
		assert.Equal(t, map[string]bool{"good.py": false}, visitor.hasError)
	}
}

func TestNewWalkingParserChecksTheLanguages(t *testing.T) {
	withJSX, withoutJSX := typescriptLanguages(t)
	_, err := NewWalkingParser(filesWalker{}, []core.Language{withJSX, withoutJSX})
	assert.ErrorContains(t, err, "two languages parse the extension")
}
