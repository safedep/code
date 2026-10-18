package depsusage

import sitter "github.com/smacker/go-tree-sitter"

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
