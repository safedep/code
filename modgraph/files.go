package modgraph

import (
	"errors"
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"io"
	"io/fs"
	"path"
	"strconv"
	"strings"
)

const (
	// headSize is the part of a Go file that Build reads first. The import
	// block of a Go file is near its top, so the head almost always holds it.
	headSize = 64 << 10
	// maxFileSize caps a Go file that Build reads whole, when its head does
	// not parse. A larger file makes its module open.
	maxFileSize = 16 << 20
	// maxConstraintTags caps the tags of a build constraint that Build tests
	// one by one. A constraint with more tags counts as one that can hold.
	maxConstraintTags = 16
)

var errBudget = errors.New("the file budget ran out")

// importRef is one import of a Go file.
type importRef struct {
	Path string `json:"path"`
	// File is the path of the file, relative to the root of its module.
	File string `json:"file"`
	Line int    `json:"line"`
	// Test marks an import of a _test.go file. Only the main module reads
	// its test files.
	Test bool `json:"test,omitempty"`
}

// skippedDir reports whether the go command skips a directory of a module:
// testdata, vendor, and a name that starts with _ or a dot.
func skippedDir(name string) bool {
	return name == "testdata" || name == "vendor" || strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".")
}

// countedFile reports whether the go command can build a file of a package
// for an importer, by its name: a .go file whose name does not start with _
// or a dot. A _test.go file counts only for the main module, whose tests the
// build of the project runs. A GOOS or GOARCH suffix of the name never drops
// a file, because the graph holds the edges of each platform.
func countedFile(name string, tests bool) bool {
	if path.Ext(name) != ".go" || strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {
		return false
	}
	return tests || !strings.HasSuffix(name, "_test.go")
}

// packageImports returns the imports of the Go files of one package. It
// reads the cache first for an immutable source.
func (b *builder) packageImports(ref pkgRef) ([]importRef, error) {
	if imps, ok, err := b.cached(ref); err != nil || ok {
		return imps, err
	}
	entries, err := fs.ReadDir(ref.mod.src.FS, ref.dir)
	if err != nil {
		return nil, fmt.Errorf("read the directory %s: %w", ref.dir, err)
	}
	var imps []importRef
	for _, e := range entries {
		if !e.Type().IsRegular() || !countedFile(e.Name(), ref.mod.main) {
			continue
		}
		if b.stats.Files >= b.maxFiles {
			return nil, errBudget
		}
		b.stats.Files++
		file := path.Join(ref.dir, e.Name())
		fileImps, err := b.fileImports(ref.mod.src.FS, file)
		if err != nil {
			return nil, err
		}
		imps = append(imps, fileImps...)
	}
	b.stats.Packages++
	if err := b.cache(ref, imps); err != nil {
		return nil, err
	}
	return imps, nil
}

// fileImports returns the imports of one Go file, or none when its build
// constraint can never hold.
func (b *builder) fileImports(fsys fs.FS, file string) ([]importRef, error) {
	f, err := b.parse(fsys, file)
	if err != nil {
		return nil, err
	}
	if excluded(f) {
		return nil, nil
	}
	test := strings.HasSuffix(file, "_test.go")
	out := make([]importRef, 0, len(f.Imports))
	for _, spec := range f.Imports {
		p, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return nil, fmt.Errorf("%s: import %s: %w", file, spec.Path.Value, err)
		}
		out = append(out, importRef{Path: p, File: file, Line: b.fset.Position(spec.Pos()).Line, Test: test})
	}
	return out, nil
}

// parse reads the package clause, the comments before it and the import
// block of a Go file. It parses the head of the file first, and the whole
// file when the head does not parse.
func (b *builder) parse(fsys fs.FS, file string) (_ *ast.File, err error) {
	fh, err := fsys.Open(file)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", file, err)
	}
	defer func() { err = errors.Join(err, fh.Close()) }()
	head := make([]byte, headSize)
	n, err := io.ReadFull(fh, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("read %s: %w", file, err)
	}
	whole := n < headSize
	src := head[:n]
	f, perr := parser.ParseFile(b.fset, file, src, parser.ImportsOnly|parser.ParseComments)
	if perr == nil || whole {
		if perr != nil {
			return nil, fmt.Errorf("%s does not parse: %w", file, perr)
		}
		return f, nil
	}
	rest, err := io.ReadAll(io.LimitReader(fh, maxFileSize-headSize+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", file, err)
	}
	if len(src)+len(rest) > maxFileSize {
		return nil, fmt.Errorf("%s is larger than %d bytes and its head does not parse", file, maxFileSize)
	}
	f, perr = parser.ParseFile(b.fset, file, append(src, rest...), parser.ImportsOnly|parser.ParseComments)
	if perr != nil {
		return nil, fmt.Errorf("%s does not parse: %w", file, perr)
	}
	return f, nil
}

// excluded reports whether no build reads a file: its build constraint can
// never hold, with each tag free except ignore, which no build sets. A
// //go:build line decides when the file has one. Else each // +build line
// must hold, as the go command reads them. The go command reads a
// constraint only in the comments before the blank line that ends them, so
// a line of the doc comment of the package does not count. A constraint that
// does not parse, or that has more than maxConstraintTags tags, leaves the
// file in, so the graph loses no edge.
func excluded(f *ast.File) bool {
	var plus []constraint.Expr
	for _, group := range f.Comments {
		if group.End() >= f.Package {
			break
		}
		if group == f.Doc {
			continue
		}
		for _, c := range group.List {
			switch {
			case constraint.IsGoBuild(c.Text):
				expr, err := constraint.Parse(c.Text)
				return err == nil && !satisfiable(expr)
			case constraint.IsPlusBuild(c.Text):
				expr, err := constraint.Parse(c.Text)
				if err != nil {
					return false
				}
				plus = append(plus, expr)
			}
		}
	}
	if len(plus) == 0 {
		return false
	}
	all := plus[0]
	for _, e := range plus[1:] {
		all = &constraint.AndExpr{X: all, Y: e}
	}
	return !satisfiable(all)
}

// satisfiable reports whether some set of tags makes a constraint hold,
// with ignore false.
func satisfiable(expr constraint.Expr) bool {
	var tags []string
	seen := map[string]bool{}
	collectTags(expr, func(tag string) {
		if tag != "ignore" && !seen[tag] {
			seen[tag] = true
			tags = append(tags, tag)
		}
	})
	if len(tags) > maxConstraintTags {
		return true
	}
	for set := 0; set < 1<<len(tags); set++ {
		on := map[string]bool{}
		for i, t := range tags {
			on[t] = set&(1<<i) != 0
		}
		if expr.Eval(func(tag string) bool { return on[tag] }) {
			return true
		}
	}
	return false
}

func collectTags(expr constraint.Expr, fn func(string)) {
	switch e := expr.(type) {
	case *constraint.TagExpr:
		fn(e.Tag)
	case *constraint.NotExpr:
		collectTags(e.X, fn)
	case *constraint.AndExpr:
		collectTags(e.X, fn)
		collectTags(e.Y, fn)
	case *constraint.OrExpr:
		collectTags(e.X, fn)
		collectTags(e.Y, fn)
	}
}
