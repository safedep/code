package depsusage

import (
	"github.com/safedep/code/core"
	sitter "github.com/smacker/go-tree-sitter"
)

// TS nodes Ignored in all languages when parsing AST
// eg. comment is useless, imports are already resolved
var commonIgnoredTypesList = []string{"comment", "import_statement", "import_from_statement", "import_declaration"}
var commonIgnoredTypes = make(map[string]bool)

type languageIgnoreRules struct {
	rule []func(node *sitter.Node, data *[]byte) bool
}

var ignoreRules = map[core.LanguageCode]languageIgnoreRules{
	core.LanguageCodePython: {
		rule: []func(node *sitter.Node, data *[]byte) bool{},
	},
	core.LanguageCodeGo: {
		rule: []func(node *sitter.Node, data *[]byte) bool{},
	},
	core.LanguageCodeJavascript: {
		rule: []func(node *sitter.Node, data *[]byte) bool{
			func(node *sitter.Node, data *[]byte) bool {
				// requires aren't identified as import by tree sitter, instead they follow
				// the pattern - variable_declarator -> call_expression -> (identifier = "require")
				if node.Type() != "variable_declarator" {
					return false
				}

				for i := range int(node.ChildCount()) {
					if node.Child(i).Type() != "call_expression" {
						continue
					}

					callExpression := node.Child(i)
					for j := range int(callExpression.ChildCount()) {
						identifier := callExpression.Child(j)
						if identifier.Type() == "identifier" && identifier.Content(*data) == "require" {
							return true
						}
					}
					break
				}

				return false
			},
		},
	},
}

func init() {
	for _, ignoredType := range commonIgnoredTypesList {
		commonIgnoredTypes[ignoredType] = true
	}
}

func isIgnoredNode(node *sitter.Node, treeLanguage *core.Language, data *[]byte) bool {
	// Ignore common ignored types like comment, import, etc in all languages
	if _, ignored := commonIgnoredTypes[node.Type()]; ignored {
		return true
	}

	ruleSet, ok := ignoreRules[(*treeLanguage).Meta().Code]
	if !ok {
		return false
	}

	for _, rule := range ruleSet.rule {
		if rule(node, data) {
			return true
		}
	}

	return false
}
