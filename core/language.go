package core

import sitter "github.com/smacker/go-tree-sitter"

// LanguageMeta is exposes metadata about a language
// implementation for the framework
type LanguageMeta struct {
	// Name of the language
	Name string

	// Supported file extensions
	SourceFileExtensions []string
}

// Language is the contract for implementing a language
// supported in the framework. It should be minimal and contain
// core language specific operations such as parsing.
type Language interface {
	// Name returns the name of the language
	Meta() LanguageMeta

	// Parser returns a TreeSitter based parser for the language
	Parser() (sitter.Parser, error)
}
