package lang

import (
	"github.com/safedep/code/core"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
)

const javascriptLanguageName = "javascript"

type javascriptLanguage struct{}

var _ core.Language = (*javascriptLanguage)(nil)

func NewJavascriptLanguage() (*javascriptLanguage, error) {
	return &javascriptLanguage{}, nil
}

func (l *javascriptLanguage) Name() string {
	return javascriptLanguageName
}

func (l *javascriptLanguage) Meta() core.LanguageMeta {
	return core.LanguageMeta{
		Name:                 javascriptLanguageName,
		Code:                 core.LanguageCodeJavascript,
		ObjectOriented:       true,
		SourceFileExtensions: []string{".js", ".mjs", ".cjs"},
	}
}

func (l *javascriptLanguage) Language() *sitter.Language {
	return javascript.GetLanguage()
}

func (l *javascriptLanguage) Resolvers() core.LanguageResolvers {
	return &javascriptResolvers{
		language: l,
	}
}
