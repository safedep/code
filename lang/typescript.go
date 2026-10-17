package lang

import (
	"github.com/safedep/code/core"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/typescript/tsx"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

const typescriptLanguageName = "typescript"

type typescriptLanguage struct {
	grammar func() *sitter.Language
	noJSX   *typescriptLanguage
}

var _ core.Language = (*typescriptLanguage)(nil)
var _ core.GrammarSelector = (*typescriptLanguage)(nil)

// NewTypescriptLanguage returns TypeScript with the TSX grammar. A parser
// built with it parses .ts, .mts and .cts files with the grammar that has no
// JSX, through LanguageForExtension.
func NewTypescriptLanguage() (*typescriptLanguage, error) {
	return &typescriptLanguage{noJSX: &typescriptLanguage{grammar: typescript.GetLanguage}}, nil
}

// NewTypescriptWithoutJSXLanguage returns TypeScript with the grammar that
// has no JSX. The TSX grammar reads the type assertion <T>x as JSX, so an
// ERROR node takes the rest of a .ts file. A detection that parses .ts,
// .mts and .cts files uses this language. It has the code of TypeScript.
func NewTypescriptWithoutJSXLanguage() (*typescriptLanguage, error) {
	return &typescriptLanguage{grammar: typescript.GetLanguage}, nil
}

func (l *typescriptLanguage) Name() string {
	return typescriptLanguageName
}

func (l *typescriptLanguage) Meta() core.LanguageMeta {
	return core.LanguageMeta{
		Name:                 typescriptLanguageName,
		Code:                 core.LanguageCodeTypescript,
		ObjectOriented:       true,
		SourceFileExtensions: []string{".ts", ".tsx", ".mts", ".cts"},
	}
}

// Language returns the grammar of the language. NewTypescriptLanguage uses
// the TSX grammar, which parses both .ts and .tsx files, except the type
// assertion <T>x.
func (l *typescriptLanguage) Language() *sitter.Language {
	if l.grammar == nil {
		return tsx.GetLanguage()
	}
	return l.grammar()
}

// LanguageForExtension returns the language without JSX for every extension
// but .tsx. The language without JSX returns itself for every extension.
func (l *typescriptLanguage) LanguageForExtension(ext string) core.Language {
	if l.noJSX == nil || ext == ".tsx" {
		return l
	}
	return l.noJSX
}

func (l *typescriptLanguage) Resolvers() core.LanguageResolvers {
	return &typescriptResolvers{}
}
