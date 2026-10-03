package lang

import (
	"github.com/safedep/code/core"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/rust"
)

const rustLanguageName = "rust"

type rustLanguage struct{}

var _ core.Language = (*rustLanguage)(nil)

func NewRustLanguage() (*rustLanguage, error) {
	return &rustLanguage{}, nil
}

func (l *rustLanguage) Name() string {
	return rustLanguageName
}

func (l *rustLanguage) Meta() core.LanguageMeta {
	return core.LanguageMeta{
		Name:                 rustLanguageName,
		Code:                 core.LanguageCodeRust,
		ObjectOriented:       false,
		SourceFileExtensions: []string{".rs"},
	}
}

func (l *rustLanguage) Language() *sitter.Language {
	return rust.GetLanguage()
}

func (l *rustLanguage) Resolvers() core.LanguageResolvers {
	return &rustResolvers{}
}
