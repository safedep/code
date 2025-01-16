package stripcomments

import (
	"slices"

	"github.com/safedep/code/core"
	sitter "github.com/smacker/go-tree-sitter"
)

type isCommentNodeCheck func(node *sitter.Node) bool

// Verify contract
var _ isCommentNodeCheck = isJavascriptCommentNode

func isJavascriptCommentNode(node *sitter.Node) bool {
	return node.Type() == "comment"
}

// Verify contract
var _ isCommentNodeCheck = isPythonCommentNode

func isPythonCommentNode(node *sitter.Node) bool {
	if node.Type() == "comment" {
		return true
	}

	if node.Type() != "string" {
		return false
	}

	parent := node.Parent()
	if parent == nil {
		return true // Root-level strings are standalone
	}

	// If the immediate parent of a string is one of these
	// then it is a standalone docstring eg. '''docstring''', """docstring"""
	return slices.Contains([]string{"function_definition", "class_definition", "module", "expression_statement"}, parent.Type())
}

func isCommentNode(node *sitter.Node, lang core.Language) bool {
	commentNodeChecks := map[core.LanguageCode]isCommentNodeCheck{
		core.LanguageCodeJavascript: isJavascriptCommentNode,
		core.LanguageCodePython:     isPythonCommentNode,
	}
	if check, ok := commentNodeChecks[lang.Meta().Code]; ok {
		return check(node)
	}
	return false
}
