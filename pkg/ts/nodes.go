package ts

import (
	"slices"

	sitter "github.com/smacker/go-tree-sitter"
)

// ChildOfType returns the first named child of one of the types, or nil.
func ChildOfType(node *sitter.Node, types ...string) *sitter.Node {
	for _, child := range NamedChildren(node) {
		if slices.Contains(types, child.Type()) {
			return child
		}
	}
	return nil
}

// HasChild reports whether one of the children of the node, named or not,
// has the type.
func HasChild(node *sitter.Node, nodeType string) bool {
	for i := 0; i < int(node.ChildCount()); i++ {
		if node.Child(i).Type() == nodeType {
			return true
		}
	}
	return false
}

// HasKeyword reports whether one of the anonymous children of the node is
// one of the words, as the static of a field.
func HasKeyword(node *sitter.Node, words ...string) bool {
	for i := 0; i < int(node.ChildCount()); i++ {
		if child := node.Child(i); !child.IsNamed() && slices.Contains(words, child.Type()) {
			return true
		}
	}
	return false
}

// HasModifier reports whether a named child of type modifier holds the
// keyword, as the static of a C# member.
func HasModifier(node *sitter.Node, keyword string) bool {
	return slices.ContainsFunc(NamedChildren(node), func(child *sitter.Node) bool {
		return child.Type() == "modifier" && HasKeyword(child, keyword)
	})
}

// SameNode reports whether two nodes span the same bytes with the same
// type. The tree gives a new *sitter.Node on each lookup, so a pointer does
// not identify a node.
func SameNode(a, b *sitter.Node) bool {
	return a.StartByte() == b.StartByte() && a.EndByte() == b.EndByte() && a.Type() == b.Type()
}
