package lang

import (
	"github.com/safedep/code/core"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
)

const goLanguageName = "go"

type goLanguage struct{}

var _ core.Language = (*goLanguage)(nil)

func NewGoLanguage() (*goLanguage, error) {
	return &goLanguage{}, nil
}

func (l *goLanguage) Name() string {
	return goLanguageName
}

func (l *goLanguage) Meta() core.LanguageMeta {
	return core.LanguageMeta{
		Name:                 goLanguageName,
		Code:                 core.LanguageCodeGo,
		ObjectOriented:       false,
		SourceFileExtensions: []string{".go"},
	}
}

func (l *goLanguage) Language() *sitter.Language {
	return golang.GetLanguage()
}

func (l *goLanguage) Resolvers() core.LanguageResolvers {
	return &goResolvers{
		language: l,
	}
}
