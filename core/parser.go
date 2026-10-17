package core

import (
	"context"
	"errors"

	sitter "github.com/smacker/go-tree-sitter"
)

// ErrNoLanguage is the error of a file that no language of a parser parses.
var ErrNoLanguage = errors.New("no language parses the file")

type Parser interface {
	Parse(context.Context, File) (ParseTree, error)
}

type ParseTree interface {
	// Tree returns the underlying TreeSitter tree
	Tree() *sitter.Tree

	// Data returns the raw data of the source file
	// from which the tree was created.
	Data() (*[]byte, error)

	// The file from which the tree was created
	File() (File, error)

	// The language used to parse the tree
	Language() (Language, error)

	// Close frees the tree. A node of the tree is not valid after Close.
	// A second call does nothing.
	Close()
}
