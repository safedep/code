package core

import "context"

// Plugin is the contract for a base plugin
type Plugin interface {
	Name() string

	SupportedLanguages() []LanguageCode
}

// TreePlugin is the contract for a plugin that can analyze a
// a parse tree (CST in Tree Sitter)
type TreePlugin interface {
	Plugin

	AnalyzeTree(context.Context, ParseTree) error
}

// FilePlugin is the contract for a plugin that can analyze a
// source file
type FilePlugin interface {
	Plugin

	AnalyzeSource(context.Context, File) error
}
