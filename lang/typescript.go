package lang

import (
	"github.com/safedep/code/core"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/typescript/tsx"
)

const typescriptLanguageName = "typescript"

type typescriptLanguage struct{}

var _ core.Language = (*typescriptLanguage)(nil)

func NewTypescriptLanguage() (*typescriptLanguage, error) {
	return &typescriptLanguage{}, nil
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

// Language returns the TSX tree-sitter grammar, which is a superset of TypeScript
// and correctly parses both .ts and .tsx files.
func (l *typescriptLanguage) Language() *sitter.Language {
	return tsx.GetLanguage()
}

func (l *typescriptLanguage) Resolvers() core.LanguageResolvers {
	return &typescriptResolvers{
		language: l,
	}
}
