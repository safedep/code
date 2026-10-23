package modgraph

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/scanner"
	"go/token"
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
	// not show the end of the import block. A larger file makes its module
	// open.
	maxFileSize = 32 << 20
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

// unbuildable is the error of a package that the go command cannot build,
// such as a package with a file that does not parse.
type unbuildable struct{ error }

func (e unbuildable) Unwrap() error { return e.error }

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
	files := 0
	for _, e := range entries {
		if e.IsDir() || !countedFile(e.Name(), ref.mod.main) {
			continue
		}
		file := path.Join(ref.dir, e.Name())
		// The go command reads a link to a file, as a file of the package.
		if !e.Type().IsRegular() {
			if info, err := fs.Stat(ref.mod.src.FS, file); err != nil || !info.Mode().IsRegular() {
				continue
			}
		}
		if b.stats.Files >= b.maxFiles {
			return nil, errBudget
		}
		b.stats.Files++
		files++
		fileImps, err := b.fileImports(ref.mod.src.FS, file)
		if err != nil {
			return nil, err
		}
		imps = append(imps, fileImps...)
	}
	if files > 0 {
		b.stats.Packages++
	}
	if err := b.cache(ref, imps); err != nil {
		return nil, err
	}
	return imps, nil
}

// fileImports returns the imports of one Go file, or none when its build
// constraint can never hold. It reads the head of the file first, and the
// whole file when the head does not show the end of the import block.
func (b *builder) fileImports(fsys fs.FS, file string) (_ []importRef, err error) {
	fh, err := fsys.Open(file)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", file, err)
	}
	defer func() { err = errors.Join(err, fh.Close()) }()
	src, err := io.ReadAll(io.LimitReader(fh, headSize))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", file, err)
	}
	f, err := parser.ParseFile(b.fset, file, src, parser.ImportsOnly)
	if len(src) == headSize && (err != nil || !importsEnd(b.fset, f, src)) {
		rest, rerr := io.ReadAll(io.LimitReader(fh, maxFileSize-headSize+1))
		if rerr != nil {
			return nil, fmt.Errorf("read %s: %w", file, rerr)
		}
		if len(src)+len(rest) > maxFileSize {
			return nil, fmt.Errorf("%s is larger than %d bytes", file, maxFileSize)
		}
		src = append(src, rest...)
		f, err = parser.ParseFile(b.fset, file, src, parser.ImportsOnly)
	}
	if err != nil {
		return nil, unbuildable{fmt.Errorf("%s does not parse: %w", file, err)}
	}
	if excluded(src) {
		return nil, nil
	}
	test := strings.HasSuffix(file, "_test.go")
	out := make([]importRef, 0, len(f.Imports))
	for _, spec := range f.Imports {
		p, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return nil, unbuildable{fmt.Errorf("%s: import %s: %w", file, spec.Path.Value, err)}
		}
		out = append(out, importRef{Path: p, File: file, Line: b.fset.Position(spec.Pos()).Line, Test: test})
	}
	return out, nil
}

// importsEnd reports whether a head of a Go file holds the end of its import
// block: after the last import declaration, a token that is not import and
// that ends before the end of the head. A head can end inside a comment
// between two import declarations, or inside the word import, and still
// parse, so a parse of the head alone proves nothing.
func importsEnd(fset *token.FileSet, f *ast.File, head []byte) bool {
	end := f.Name.End()
	if n := len(f.Decls); n > 0 {
		end = f.Decls[n-1].End()
	}
	off := fset.File(f.Package).Offset(end)
	rest := head[off:]
	tf := token.NewFileSet().AddFile("", -1, len(rest))
	var sc scanner.Scanner
	bad := false
	sc.Init(tf, rest, func(token.Position, string) { bad = true }, 0)
	for {
		pos, tok, lit := sc.Scan()
		switch {
		case bad || tok == token.EOF || tok == token.ILLEGAL || tok == token.IMPORT:
			return false
		case tok == token.SEMICOLON && lit == "\n":
			continue
		}
		size := len(lit)
		if size == 0 {
			size = len(tok.String())
		}
		return tf.Offset(pos)+size < len(rest)
	}
}

// excluded reports whether no build reads a file: its build constraint can
// never hold, with each tag free except ignore, which no build sets. It
// reads the constraint as the go command does, in go/build shouldBuild: a
// //go:build line in the leading comments decides, and else each // +build
// line before the last blank line of the leading // comments must hold. A
// constraint that does not parse, two //go:build lines, and a constraint
// with more than maxConstraintTags tags leave the file in, so the graph
// loses no edge.
func excluded(src []byte) bool {
	header, goBuild, ok := fileHeader(src)
	if !ok {
		return false
	}
	if goBuild != "" {
		expr, err := constraint.Parse(goBuild)
		return err == nil && !satisfiable(expr)
	}
	var all constraint.Expr
	for line := range strings.Lines(string(header)) {
		line = strings.TrimSpace(line)
		if !constraint.IsPlusBuild(line) {
			continue
		}
		expr, err := constraint.Parse(line)
		if err != nil {
			continue
		}
		if all == nil {
			all = expr
		} else {
			all = &constraint.AndExpr{X: all, Y: expr}
		}
	}
	return all != nil && !satisfiable(all)
}

// fileHeader is go/build parseFileHeader. It returns the part of the file
// up to the last blank line of the leading run of // comments and blank
// lines, where a // +build line counts, and the //go:build line of the
// leading comments. ok is false for a file with two //go:build lines.
func fileHeader(content []byte) (header []byte, goBuild string, ok bool) {
	end := 0
	p := content
	ended := false
	inSlashStar := false
Lines:
	for len(p) > 0 {
		line := p
		if i := bytes.IndexByte(line, '\n'); i >= 0 {
			line, p = line[:i], p[i+1:]
		} else {
			p = p[len(p):]
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 && !ended {
			end = len(content) - len(p)
			continue Lines
		}
		if !bytes.HasPrefix(line, []byte("//")) {
			ended = true
		}
		if !inSlashStar && isGoBuildComment(line) {
			if goBuild != "" {
				return nil, "", false
			}
			goBuild = string(line)
		}
	Comments:
		for len(line) > 0 {
			if inSlashStar {
				if i := bytes.Index(line, []byte("*/")); i >= 0 {
					inSlashStar = false
					line = bytes.TrimSpace(line[i+2:])
					continue Comments
				}
				continue Lines
			}
			if bytes.HasPrefix(line, []byte("//")) {
				continue Lines
			}
			if bytes.HasPrefix(line, []byte("/*")) {
				inSlashStar = true
				line = bytes.TrimSpace(line[2:])
				continue Comments
			}
			break Lines
		}
	}
	return content[:end], goBuild, true
}

// isGoBuildComment is go/build isGoBuildComment.
func isGoBuildComment(line []byte) bool {
	rest, ok := bytes.CutPrefix(line, []byte("//go:build"))
	if !ok {
		return false
	}
	return len(rest) == 0 || len(bytes.TrimSpace(rest)) < len(rest)
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
