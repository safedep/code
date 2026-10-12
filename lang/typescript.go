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
}

var _ core.Language = (*typescriptLanguage)(nil)

func NewTypescriptLanguage() (*typescriptLanguage, error) {
	return &typescriptLanguage{}, nil
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

func (l *typescriptLanguage) Resolvers() core.LanguageResolvers {
	return &typescriptResolvers{
		language: l,
	}
}
