package ast

import sitter "github.com/smacker/go-tree-sitter"

// TreeSitter requires the content to be available to
// get the symbols from the source code for a given node
type Content *[]byte

func ToContent(data []byte) Content {
	return &data
}

type Node struct {
	// We are using a pointer here to avoid duplicating
	// the data of entire source code across all nodes
	content Content
}

func (n *Node) contentForNode(node *sitter.Node) string {
	if node == nil || n.content == nil {
		return ""
	}

	return node.Content(*n.content)
}
