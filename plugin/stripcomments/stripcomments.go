package stripcomments

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"runtime"

	"github.com/safedep/code/core"
	"github.com/safedep/code/pkg/ds"
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
	err = stripComments(tree.Tree().RootNode(), treeData, &output, lang)
	if err != nil {
		return fmt.Errorf("failed to strip comments: %w", err)
	}

	err = p.stripCommentsCallback(ctx, newStripCommentsPluginData(file, &output))
	if err != nil {
		return fmt.Errorf("failed to execute stripCommentsCallback: %w", err)
	}

	return nil
}

// We use pointer to source to avoid copying the entire file contents across function calls
func stripComments(node *sitter.Node, source *[]byte, output *bytes.Buffer, lang core.Language) error {
	if node == nil {
		return fmt.Errorf("node is nil")
	}

	if source == nil {
		return fmt.Errorf("source is nil")
	}

	stack := ds.NewStack[*sitter.Node]()
	stack.Push(node)

	var prevNode *sitter.Node
	for !stack.IsEmpty() {
		currentNode, _ := stack.Pop()

		if prevNode != nil {
			prevStart := prevNode.StartByte()
			prevEnd := prevNode.EndByte()
			currStart := currentNode.StartByte()

			// Copy leading whitespace and newlines between parent's start and first child's start
			// eg. Python Hierarchy: match_statement -> cases block -> case -> ...
			// match str:
			// 	 case "hello":
			// 	 	 print("Hello")
			// Here, the whitespace between "str:" and "case" is preserved since "case" is a child of cases 'block' which starts just after "str:"
			//
			// Note - prevNode.Child() consumes lesser time than currentNode.Parent()
			if prevNode.Child(0) == currentNode && prevStart < currStart {
				output.Write((*source)[prevStart:currStart])
			}

			// Preserve whitespace and newlines between previous node's end and current node
			if currStart > prevEnd {
				output.Write((*source)[prevEnd:currStart])
			}
		}

		// Skip comment nodes
		if isCommentNode(currentNode, lang) {
			prevNode = currentNode
			continue
		}

		if currentNode.ChildCount() == 0 {
			start := currentNode.StartByte()
			end := currentNode.EndByte()
			output.Write((*source)[start:end])
		} else {
			// Push children onto the stack in reverse order due to LIFO nature
			for i := int(currentNode.ChildCount()) - 1; i >= 0; i-- {
				child := currentNode.Child(i)
				stack.Push(child)
			}
		}

		// Let Go switch to other goroutines if there are any
		// This is to avoid CPU spikes for this Go routine while avoiding
		// starvation of other goroutines
		runtime.Gosched()

		prevNode = currentNode
	}

	return nil
}
