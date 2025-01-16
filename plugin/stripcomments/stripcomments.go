package stripcomments

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/safedep/code/core"
	"github.com/safedep/dry/log"
	sitter "github.com/smacker/go-tree-sitter"
)

// StripCommentsPluginData contains the File referring to the source file originally
// processed by the plugin, and a io Reader to the stripped contents.
type StripCommentsPluginData struct {
	File   core.File
	Reader io.Reader
}

// StripCommentsCallback is the callback function that is called for
// every parsed file with the stripped file contents.
type StripCommentsCallback core.PluginCallback[*StripCommentsPluginData]

type stripCommentsPlugin struct {
	stripCommentsCallback StripCommentsCallback
}

// Verify contract
var _ core.TreePlugin = (*stripCommentsPlugin)(nil)

func newStripCommentsPluginData(f core.File, r io.Reader) *StripCommentsPluginData {
	return &StripCommentsPluginData{
		File:   f,
		Reader: r,
	}
}

// stripcomments plugin removes the comments from source code.
// It uses tree-sitter to parse the source code and reconstructs it from
// the parse tree by skipping comment nodes.
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

	// @TODO - If performance issues arise due to copying the entire file contents
	// into memory, consider storing in a temporary file and create reader on that file
	var output bytes.Buffer
	stripComments(tree.Tree().RootNode(), *treeData, &output, lang)

	err = p.stripCommentsCallback(ctx, newStripCommentsPluginData(file, &output))
	if err != nil {
		return fmt.Errorf("failed to execute stripCommentsCallback: %w", err)
	}

	return nil
}

func stripComments(node *sitter.Node, source []byte, output *bytes.Buffer, lang core.Language) {
	if node == nil {
		return
	}

	// Skip comment nodes
	if isCommentNode(node, lang) {
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
			stripComments(child, source, output, lang)
			if i < int(node.ChildCount())-1 {
				// Add whitespace or newlines between current and next node, as per the source
				intermediateStart := node.Child(i).EndByte()
				intermediateEnd := node.Child(i + 1).StartByte()
				output.Write(source[intermediateStart:intermediateEnd])
			}
		}
	}
}
