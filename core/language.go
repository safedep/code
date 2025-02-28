package core

import (
	"github.com/safedep/code/core/ast"
	sitter "github.com/smacker/go-tree-sitter"
)

// LanguageResolvers define the minimum contract for a language
// implementation to resolve language specific concerns
// such as imports, functions, etc.
type LanguageResolvers interface {
	// ResolveImports returns a list of import statements
	// identified from the parse tree
	ResolveImports(tree ParseTree) ([]*ast.ImportNode, error)
}

// ObjectOrientedLanguageResolvers define the additional contract
// for a language implementation to resolve object oriented
// language specific concerns such as classes, methods, etc.
type ObjectOrientedLanguageResolvers interface {
	// Object oriented language specific resolvers go here
	// We are following "Interface Segregation Principle" here to
	// prevent the Language interface from becoming too large
	// especially when a language does not support object oriented
	// programming. This is at the cost of analyzers having to
	// check for the supported resolvers before using them.

	// Placeholder operation, remove when adding real operations
	Nop() error
}

type LanguageCode string

const (
	LanguageCodePython     LanguageCode = "python"
	LanguageCodeJavascript LanguageCode = "javascript"
	LanguageCodeGo         LanguageCode = "go"
)

// LanguageMeta is exposes metadata about a language
// implementation for the framework
type LanguageMeta struct {
	// Name of the language
	Name string

	// Code of the language, used for internal comparisons
	Code LanguageCode

	// Flag to indicate if the language is object oriented
	ObjectOriented bool

	// Supported file extensions
	SourceFileExtensions []string
}

// Language is the contract for implementing a language
// supported in the framework. It should be minimal and contain
// core language specific operations such as parsing.
type Language interface {
	// Name returns the name of the language
	Meta() LanguageMeta

	// Tree Sitter Language
	Language() *sitter.Language

	// Get language specific resolvers
	Resolvers() LanguageResolvers
}
