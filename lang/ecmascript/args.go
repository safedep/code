package ecmascript

import sitter "github.com/smacker/go-tree-sitter"

// ArgKind is the shape of an argument, as a detection reads it.
type ArgKind int

const (
	// ArgNone is a missing argument.
	ArgNone ArgKind = iota

	// ArgString is a string literal or a template with no substitution.
	ArgString

	// ArgTemplate is a template with a substitution, as `${a}.js`.
	ArgTemplate

	// ArgName is an identifier.
	ArgName

	// ArgMember is a member expression, as cfg.path.
	ArgMember

	// ArgLiteral is a literal that is not a string: a number, a regular
	// expression, true, false, null or undefined.
	ArgLiteral

	// ArgComputed is any other expression, as a call, a sum or a subscript.
	ArgComputed
)

var literalTypes = map[string]bool{"number": true, "regex": true, "true": true, "false": true, "null": true, "undefined": true}

// ClassifyArg returns the shape of an argument. It does not look through
// parentheses. Call File.Unparen first to read ('x') as a string.
func ClassifyArg(n *sitter.Node) ArgKind {
	if n == nil {
		return ArgNone
	}
	switch t := n.Type(); {
	case t == "string":
		return ArgString
	case t == "template_string":
		if hasSubstitution(n) {
			return ArgTemplate
		}
		return ArgString
	case t == "identifier":
		return ArgName
	case t == "member_expression":
		return ArgMember
	case literalTypes[t]:
		return ArgLiteral
	}
	return ArgComputed
}

func hasSubstitution(n *sitter.Node) bool {
	c := sitter.NewTreeCursor(n)
	defer c.Close()
	for ok := c.GoToFirstChild(); ok; ok = c.GoToNextSibling() {
		if c.CurrentNode().Type() == "template_substitution" {
			return true
		}
	}
	return false
}
