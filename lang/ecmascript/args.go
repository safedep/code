package ecmascript

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

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

// maxString is the longest string that StringValue builds from parts, as
// 'ev' + 'al'.
const maxString = 4096

// ClassifyArg returns the shape of an argument, through parentheses, so
// ('x') is a string.
func (a *Analysis) ClassifyArg(n *sitter.Node) ArgKind {
	return classify(a.Unparen(n))
}

func classify(n *sitter.Node) ArgKind {
	if n == nil {
		return ArgNone
	}
	switch t := n.Type(); {
	case t == "string":
		return ArgString
	case t == "template_string":
		if substitution(n) != nil {
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

// substitution returns the first substitution of a template, or nil.
func substitution(n *sitter.Node) *sitter.Node {
	c := sitter.NewTreeCursor(n)
	defer c.Close()
	for ok := c.GoToFirstChild(); ok; ok = c.GoToNextSibling() {
		if child := c.CurrentNode(); child.Type() == "template_substitution" {
			return child
		}
	}
	return nil
}

// StringValue returns the value of a plain string: a string literal, a
// template with no substitution, or a sum of them, as 'ev' + 'al', through
// parentheses. Escapes are decoded, so 'child\x5fprocess' gives
// child_process.
func (a *Analysis) StringValue(n *sitter.Node) (string, bool) {
	var b strings.Builder
	if !a.writeString(&b, n, 0) {
		return "", false
	}
	return b.String(), true
}

// writeString writes the value of a plain string or of a sum of them. A sum
// is a tree that grows to the left, as (('a' + 'b') + 'c'). The loop reads
// its left edge, so a long sum needs no recursion. A sum on the right, as
// 'a' + ('b' + 'c'), recurses to a bounded depth.
func (a *Analysis) writeString(b *strings.Builder, n *sitter.Node, depth int) bool {
	if depth >= maxBranch {
		return false
	}
	var rights []*sitter.Node
	n = a.Unparen(n)
	for n != nil && n.Type() == "binary_expression" {
		op := n.ChildByFieldName("operator")
		if op == nil || op.Type() != "+" {
			return false
		}
		rights = append(rights, n.ChildByFieldName("right"))
		n = a.Unparen(n.ChildByFieldName("left"))
	}
	if !a.writeLiteral(b, n) {
		return false
	}
	for i := len(rights) - 1; i >= 0; i-- {
		if !a.writeString(b, rights[i], depth+1) {
			return false
		}
	}
	return true
}

// writeLiteral writes the value of a plain string. It fails when the value
// would pass maxString.
func (a *Analysis) writeLiteral(b *strings.Builder, n *sitter.Node) bool {
	if classify(n) != ArgString {
		return false
	}
	text := n.Content(a.src)
	if len(text) < 2 || b.Len()+len(text) > maxString {
		return false
	}
	b.WriteString(unescape(text[1 : len(text)-1]))
	return true
}

// StringPrefix returns the decoded text of a string or a template up to its
// first substitution, as ./ in `./${name}`, through parentheses.
func (a *Analysis) StringPrefix(n *sitter.Node) (string, bool) {
	n = a.Unparen(n)
	if n == nil {
		return "", false
	}
	switch n.Type() {
	case "string":
		return a.StringValue(n)
	case "template_string":
		end := n.EndByte() - 1
		if sub := substitution(n); sub != nil {
			end = sub.StartByte()
		}
		start := n.StartByte() + 1
		if end < start || end-start > maxString {
			return "", false
		}
		return unescape(string(a.src[start:end])), true
	}
	return "", false
}
