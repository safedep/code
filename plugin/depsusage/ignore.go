package depsusage

import (
	"github.com/safedep/code/core"
	sitter "github.com/smacker/go-tree-sitter"
)

// TS nodes Ignored in all languages when parsing AST
// eg. comment is useless, imports are already resolved
var commonIgnoredTypesList = []string{
	"comment", "import_statement", "import_from_statement", "import_declaration",
	// C#, Rust, PHP and Kotlin imports
	"using_directive", "use_declaration", "extern_crate_declaration", "namespace_use_declaration", "import_list",
}
var commonIgnoredTypes = make(map[string]bool)

type languageIgnoreRules struct {
	rule []func(node *sitter.Node, data *[]byte) bool
}

// isRequireCallIgnoreRule checks if a variable_declarator or an assignment keeps the result of
// a require() call or of an awaited import(), directly or through a member
// access or a call, as in require('events').EventEmitter or
// require('debug')('app'). The resolvers report these as imports, so the
// declared name is not a usage.
func isRequireCallIgnoreRule(node *sitter.Node, data *[]byte) bool {
	var value *sitter.Node
	switch node.Type() {
	case "variable_declarator":
		value = node.ChildByFieldName("value")
	case "assignment_expression":
		value = node.ChildByFieldName("right")
	default:
		return false
	}
	for value != nil {
		switch value.Type() {
		case "member_expression":
			value = value.ChildByFieldName("object")
		case "await_expression":
			value = value.NamedChild(0)
		case "call_expression":
			function := value.ChildByFieldName("function")
			if function == nil {
				return false
			}
			if function.Type() == "import" || (function.Type() == "identifier" && function.Content(*data) == "require") {
				return true
			}
			value = function
		default:
			return false
		}
	}
	return false
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
			isRequireCallIgnoreRule,
		},
	},
	core.LanguageCodeTypescript: {
		rule: []func(node *sitter.Node, data *[]byte) bool{
			isRequireCallIgnoreRule,
			// Skip type_alias_declaration nodes (e.g., type Foo = Bar)
			func(node *sitter.Node, _ *[]byte) bool {
				return node.Type() == "type_alias_declaration"
			},
			// Skip interface_declaration nodes
			func(node *sitter.Node, _ *[]byte) bool {
				return node.Type() == "interface_declaration"
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
