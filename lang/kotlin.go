package lang

import (
	"github.com/safedep/code/core"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/kotlin"
)

const kotlinLanguageName = "kotlin"

type kotlinLanguage struct{}

var _ core.Language = (*kotlinLanguage)(nil)

func NewKotlinLanguage() (*kotlinLanguage, error) {
	return &kotlinLanguage{}, nil
}

func (l *kotlinLanguage) Name() string {
	return kotlinLanguageName
}

func (l *kotlinLanguage) Meta() core.LanguageMeta {
	return core.LanguageMeta{
		Name:                 kotlinLanguageName,
		Code:                 core.LanguageCodeKotlin,
		ObjectOriented:       true,
		SourceFileExtensions: []string{".kt", ".kts"},
	}
}

func (l *kotlinLanguage) Language() *sitter.Language {
	return kotlin.GetLanguage()
}

func (l *kotlinLanguage) Resolvers() core.LanguageResolvers {
	return &kotlinResolvers{}
}
