package indexer

import (
	"bytes"
	"context"
	"go/build"
	"io"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/pkg/ts"
	sitter "github.com/smacker/go-tree-sitter"
)

// goOS and goArch are the GOOS and GOARCH values of the Go toolchain. A
// reach answer must hold for every platform that a project builds for, so
// the index keeps a file of a dependency unit when the build of any pair
// reads it. An index of one platform drops the other twin of a
// thing_linux.go and thing_darwin.go pair, and a call that only the darwin
// build makes then reads as no-such-target on a darwin host.
var (
	goOS   = []string{"aix", "android", "darwin", "dragonfly", "freebsd", "hurd", "illumos", "ios", "js", "linux", "netbsd", "openbsd", "plan9", "solaris", "wasip1", "windows", "zos"}
	goArch = []string{"386", "amd64", "arm", "arm64", "loong64", "mips", "mipsle", "mips64", "mips64le", "ppc64", "ppc64le", "riscv64", "s390x", "sparc64", "wasm"}
)

// goExcluded reports whether no Go build reads a Go file of a dependency
// unit: a test file, a file with //go:build ignore, or a file whose
// constraint needs a tag that the go command never sets by itself, as
// purego. Such a file holds no code that an importer runs by default. Its
// package clause can differ from the clause of its directory, as package
// main of a gen.go, and its index would hold calls that no program makes.
// A build constraint that the build cannot read leaves the file in, so the
// index loses no code.
func goExcluded(file core.File, data []byte) bool {
	if !file.IsImport() || path.Ext(file.Name()) != ".go" {
		return false
	}
	name := path.Base(file.Name())
	if strings.HasSuffix(name, "_test.go") {
		return true
	}
	ctxt := build.Default
	ctxt.CgoEnabled = true
	ctxt.BuildTags, ctxt.ToolTags = nil, nil
	ctxt.OpenFile = func(string) (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(data)), nil }
	for _, os := range goOS {
		for _, arch := range goArch {
			ctxt.GOOS, ctxt.GOARCH = os, arch
			if match, err := ctxt.MatchFile(".", name); err != nil || match {
				return false
			}
		}
	}
	return true
}

// generatedHeader matches the line that marks a generated Go file.
var generatedHeader = regexp.MustCompile(`(?m)^// Code generated .* DO NOT EDIT\.$`)

// isGeneratedTable reports whether a Go file of a dependency unit is a
// generated file or a table file, as idna/tables15.0.0.go of x/net.
func isGeneratedTable(name string, data []byte) bool {
	if unit, _ := fileindex.SplitUnit(name); unit == "" || path.Ext(name) != ".go" {
		return false
	}
	if base := path.Base(name); strings.HasPrefix(base, "tables") {
		return true
	}
	head := data
	if i := bytes.Index(data, []byte("\npackage ")); i >= 0 {
		head = data[:i]
	}
	return generatedHeader.Match(head)
}

// goSkeleton returns the bytes of a Go file with each value of a
// top-level var that holds only data replaced by 0 and spaces. The
// declarations keep their lines and columns, and the file still parses. The
// data values of a generated table file are what hit the limits of the
// index, as a string of thousands of concatenated parts. Function bodies and
// any value with a call or a function literal stay, because a generated
// file can hold code that a program runs, as a .pb.go file.
func goSkeleton(tree *sitter.Tree, data []byte) []byte {
	out := bytes.Clone(data)
	clear := func(from, to uint32) {
		for i := from; i < to; i++ {
			if out[i] != '\n' {
				out[i] = ' '
			}
		}
	}
	var visit func(node *sitter.Node)
	visit = func(node *sitter.Node) {
		switch node.Type() {
		case "function_declaration", "method_declaration":
			return
		case "var_spec":
			if values := node.ChildByFieldName("value"); values != nil {
				for _, v := range ts.NamedChildren(values) {
					if !holdsCode(v) {
						clear(v.StartByte(), v.EndByte())
						out[v.StartByte()] = '0'
					}
				}
			}
			return
		}
		for _, child := range ts.NamedChildren(node) {
			visit(child)
		}
	}
	visit(tree.RootNode())
	return out
}

// holdsCode reports whether an expression holds a call or a function
// literal, which a program runs when it initializes the package.
func holdsCode(node *sitter.Node) bool {
	if t := node.Type(); t == "call_expression" || t == "func_literal" {
		return true
	}
	return slices.ContainsFunc(ts.NamedChildren(node), holdsCode)
}

// skeletonIndex indexes the skeleton of a generated table file of a
// dependency unit whose index hit a limit. The index keeps the hash of the
// bytes of the file, so the store serves it on the next run.
func skeletonIndex(ctx context.Context, p contentParser, file pending, tree core.ParseTree, ix *fileindex.Index) (*fileindex.Index, error) {
	if ix == nil || ix.Language != langcode.Go || len(ix.Truncated) == 0 || partial(ix) || !isGeneratedTable(file.file.Name(), file.data) {
		return ix, nil
	}
	skeleton, err := p.ParseContent(ctx, file.file, goSkeleton(tree.Tree(), file.data))
	if err != nil {
		return nil, err
	}
	defer skeleton.Close()
	out, err := project(ctx, skeleton)
	if err != nil {
		return nil, err
	}
	out.ContentHash = file.key.ContentHash
	return out, nil
}
