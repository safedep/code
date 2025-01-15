package stripcomments

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"slices"

	"github.com/safedep/code/core"
	"github.com/safedep/dry/log"
	sitter "github.com/smacker/go-tree-sitter"
)

type StripCommentsCallback func(f core.File, r io.Reader) error

type stripCommentsPlugin struct {
	stripCommentsCallback StripCommentsCallback
}

// Verify contract
var _ core.TreePlugin = (*stripCommentsPlugin)(nil)

func NewStripCommentsPlugin(stripCommentsCallback StripCommentsCallback) *stripCommentsPlugin {
	return &stripCommentsPlugin{
		stripCommentsCallback: stripCommentsCallback,
	}
}

func (p *stripCommentsPlugin) Name() string {
	return "StripCommentsPlugin"
}

var supportedLanguages = []core.LanguageCode{core.LanguageCodePython, core.LanguageCodeJavascript}

func (p *stripCommentsPlugin) SupportedLanguages() []core.LanguageCode {
	return supportedLanguages
}

func (p *stripCommentsPlugin) AnalyzeTree(ctx context.Context, tree core.ParseTree) error {
	lang, err := tree.Language()
	if err != nil {
		return fmt.Errorf("failed to get language: %w", err)
	}

	file, err := tree.File()
	if err != nil {
		return fmt.Errorf("failed to get file: %w", err)
	}

	treeData, err := tree.Data()
	if err != nil {
		return fmt.Errorf("failed to get tree data: %w", err)
	}

	log.Debugf("stripcomments - Analyzing tree for language: %s, file: %s\n",
		lang.Meta().Code, file.Name())

	// @TODO - If performance issues arise due to copying the entire file contents into memory,
	// consider storing in a temporary file and returning a reader to that file
	var output bytes.Buffer
	stripComments(tree.Tree().RootNode(), *treeData, &output)

	return p.stripCommentsCallback(file, &output)
}

func stripComments(node *sitter.Node, source []byte, output *bytes.Buffer) {
	if node == nil {
		return
	}

	// Skip comments and standalone doc strings
	if node.Type() == "comment" || (node.Type() == "string" && isStandaloneDocstring(node)) {
		return
	}

	// Preserve leading whitespace and newlines for non-comment nodes
	start := node.StartByte()
	end := node.EndByte()
	if node.ChildCount() == 0 {
		output.Write(source[start:end])
	} else {
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			stripComments(child, source, output)
			if i < int(node.ChildCount())-1 {
				// Add whitespace or newlines between current and next node, as per the source
				intermediateStart := node.Child(i).EndByte()
				intermediateEnd := node.Child(i + 1).StartByte()
				output.Write(source[intermediateStart:intermediateEnd])
			}
		}
	}
}

func isStandaloneDocstring(node *sitter.Node) bool {
	parent := node.Parent()
	if parent == nil {
		return true // Root-level strings are standalone
	}

	// If the immediate parent is a function'class/module/expression_statement the string is docstring
	parentType := parent.Type()
	return slices.Contains([]string{"function_definition", "class_definition", "module", "expression_statement"}, parentType)
}
