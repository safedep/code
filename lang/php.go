package lang

import (
	"github.com/safedep/code/core"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/php"
)

const phpLanguageName = "php"

type phpLanguage struct{}

var _ core.Language = (*phpLanguage)(nil)

func NewPHPLanguage() (*phpLanguage, error) {
	return &phpLanguage{}, nil
}

func (l *phpLanguage) Name() string {
	return phpLanguageName
}

func (l *phpLanguage) Meta() core.LanguageMeta {
	return core.LanguageMeta{
		Name:                 phpLanguageName,
		Code:                 core.LanguageCodePHP,
		ObjectOriented:       true,
		SourceFileExtensions: []string{".php"},
	}
}

func (l *phpLanguage) Language() *sitter.Language {
	return php.GetLanguage()
}

func (l *phpLanguage) Resolvers() core.LanguageResolvers {
	return &phpResolvers{}
}
