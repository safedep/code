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

// NodePosition represents position information for a Tree-Sitter node
type NodePosition struct {
	StartByte   uint32
	EndByte     uint32
	StartLine   uint32
	EndLine     uint32
	StartColumn uint32
	EndColumn   uint32
}

// GetNodePosition extracts position information from a Tree-Sitter node
func GetNodePosition(node *sitter.Node) NodePosition {
	if node == nil {
		return NodePosition{}
	}

	startPoint := node.StartPoint()
	endPoint := node.EndPoint()

	return NodePosition{
		StartByte:   node.StartByte(),
		EndByte:     node.EndByte(),
		StartLine:   startPoint.Row + 1,    // Tree-sitter uses 0-based rows
		EndLine:     endPoint.Row + 1,      // Tree-sitter uses 0-based rows
		StartColumn: startPoint.Column + 1, // Tree-sitter uses 0-based columns
		EndColumn:   endPoint.Column + 1,   // Tree-sitter uses 0-based columns
	}
}

// GetNodeType returns the type of the Tree-Sitter node
func GetNodeType(node *sitter.Node) string {
	if node == nil {
		return ""
	}

	return node.Type()
}
