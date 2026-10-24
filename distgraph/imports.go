package distgraph

import (
	"context"
	"fmt"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/safedep/code/core"
	"github.com/safedep/code/lang"
	"github.com/safedep/code/parser"
	"github.com/safedep/code/pkg/ts"
)

// importRef is one import of a Python file. A dynamic import names a
// module that the code computes, so the file can import any module. A
// guarded import sits in a try statement that catches an ImportError, so
// the program runs without the module.
type importRef struct {
	module  string
	line    int
	dynamic bool
	guarded bool
}

// importers are the functions that import a module by a name at run time.
var importers = map[string]bool{"importlib.import_module": true, "import_module": true, "__import__": true}

// importErrors are the exceptions that a try statement catches to run
// without a module.
var importErrors = map[string]bool{"ImportError": true, "ModuleNotFoundError": true, "Exception": true, "BaseException": true}

// importReader reads the imports of Python files with the tree-sitter
// grammar of Python.
type importReader struct {
	lang core.Language
}

func newImportReader() (*importReader, error) {
	l, err := lang.NewPythonLanguage()
	if err != nil {
		return nil, fmt.Errorf("distgraph: %w", err)
	}
	return &importReader{lang: l}, nil
}

// read returns the imports of a Python file, and whether the whole file
// parses. A file with a syntax error can hold an import that the tree does
// not show.
func (r *importReader) read(ctx context.Context, data []byte) ([]importRef, bool) {
	tree, err := parser.ParseBytes(ctx, r.lang, data)
	if err != nil {
		return nil, false
	}
	defer tree.Close()
	root := tree.Tree().RootNode()
	var out []importRef
	text := func(n *sitter.Node) string { return n.Content(data) }
	var walk func(n *sitter.Node, guarded bool)
	walk = func(n *sitter.Node, guarded bool) {
		switch n.Type() {
		case "import_statement":
			for _, child := range ts.NamedChildren(n) {
				name := child
				if child.Type() == "aliased_import" {
					name = child.ChildByFieldName("name")
				}
				if name != nil && name.Type() == "dotted_name" {
					out = append(out, importRef{module: text(name), line: int(name.StartPoint().Row) + 1, guarded: guarded})
				}
			}
			return
		case "import_from_statement":
			if name := n.ChildByFieldName("module_name"); name != nil && name.Type() == "dotted_name" {
				out = append(out, importRef{module: text(name), line: int(name.StartPoint().Row) + 1, guarded: guarded})
			}
			return
		case "future_import_statement":
			return
		case "call":
			if fn := n.ChildByFieldName("function"); fn != nil && importers[text(fn)] {
				out = append(out, r.dynamicImport(n, data, guarded))
			}
		case "try_statement":
			guarded = guarded || catchesImportError(n, data)
		}
		for _, child := range ts.NamedChildren(n) {
			walk(child, guarded)
		}
	}
	walk(root, false)
	return out, !root.HasError()
}

// dynamicImport returns the import of a call of an importer: the module of
// a plain string argument, or a dynamic import for any other argument.
func (r *importReader) dynamicImport(call *sitter.Node, data []byte, guarded bool) importRef {
	ref := importRef{line: int(call.StartPoint().Row) + 1, dynamic: true, guarded: guarded}
	args := call.ChildByFieldName("arguments")
	if args == nil || args.NamedChildCount() == 0 {
		return ref
	}
	arg := args.NamedChild(0)
	if arg.Type() != "string" {
		return ref
	}
	var content string
	for _, part := range ts.NamedChildren(arg) {
		switch part.Type() {
		case "string_content":
			content += part.Content(data)
		case "string_start", "string_end":
		default:
			return ref
		}
	}
	ref.module, ref.dynamic = content, false
	return ref
}

// catchesImportError reports whether a try statement has an except clause
// for an ImportError: a bare except, or one that names the error or a base
// of it.
func catchesImportError(try *sitter.Node, data []byte) bool {
	for _, child := range ts.NamedChildren(try) {
		if child.Type() != "except_clause" {
			continue
		}
		var caught *sitter.Node
		for _, part := range ts.NamedChildren(child) {
			if part.Type() != "block" {
				caught = part
				break
			}
		}
		if caught == nil || namesImportError(caught, data) {
			return true
		}
	}
	return false
}

// namesImportError reports whether an exception expression names an
// ImportError, alone or in a tuple, as except (ImportError, OSError).
func namesImportError(n *sitter.Node, data []byte) bool {
	switch n.Type() {
	case "identifier":
		return importErrors[n.Content(data)]
	case "tuple", "parenthesized_expression", "as_pattern":
		for _, child := range ts.NamedChildren(n) {
			if namesImportError(child, data) {
				return true
			}
		}
	}
	return false
}
