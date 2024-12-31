package core

import (
	"context"

	sitter "github.com/smacker/go-tree-sitter"
)

type Parser interface {
	Parse(context.Context, File) (ParseTree, error)
}

type ParseTree interface {
	// Tree returns the underlying TreeSitter tree
	Tree() *sitter.Tree

	// Data returns the raw data of the source file
	// from which the tree was created
	Data() ([]byte, error)

	// The file from which the tree was created
	File() (File, error)
}
