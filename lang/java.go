package lang

import (
	"github.com/safedep/code/core"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/java"
)

const javaLanguageName = "java"

type javaLanguage struct{}

var _ core.Language = (*javaLanguage)(nil)

func NewJavaLanguage() (*javaLanguage, error) {
	return &javaLanguage{}, nil
}

func (l *javaLanguage) Name() string {
	return javaLanguageName
}

func (l *javaLanguage) Meta() core.LanguageMeta {
	return core.LanguageMeta{
		Name:                 javaLanguageName,
		Code:                 core.LanguageCodeJava,
		ObjectOriented:       true,
		SourceFileExtensions: []string{".java"},
	}
}

func (l *javaLanguage) Language() *sitter.Language {
	return java.GetLanguage()
}

func (l *javaLanguage) Resolvers() core.LanguageResolvers {
	return &javaResolvers{
		language: l,
	}
}
