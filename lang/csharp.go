package lang

import (
	"github.com/safedep/code/core"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/csharp"
)

const csharpLanguageName = "csharp"

type csharpLanguage struct{}

var _ core.Language = (*csharpLanguage)(nil)

func NewCSharpLanguage() (*csharpLanguage, error) {
	return &csharpLanguage{}, nil
}

func (l *csharpLanguage) Name() string {
	return csharpLanguageName
}

func (l *csharpLanguage) Meta() core.LanguageMeta {
	return core.LanguageMeta{
		Name:                 csharpLanguageName,
		Code:                 core.LanguageCodeCSharp,
		ObjectOriented:       true,
		SourceFileExtensions: []string{".cs"},
	}
}

func (l *csharpLanguage) Language() *sitter.Language {
	return csharp.GetLanguage()
}

func (l *csharpLanguage) Resolvers() core.LanguageResolvers {
	return &csharpResolvers{}
}
