package lang

import (
	"github.com/safedep/code/core"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/ruby"
)

const rubyLanguageName = "ruby"

type rubyLanguage struct{}

var _ core.Language = (*rubyLanguage)(nil)

func NewRubyLanguage() (*rubyLanguage, error) {
	return &rubyLanguage{}, nil
}

func (l *rubyLanguage) Name() string {
	return rubyLanguageName
}

func (l *rubyLanguage) Meta() core.LanguageMeta {
	return core.LanguageMeta{
		Name:                 rubyLanguageName,
		Code:                 core.LanguageCodeRuby,
		ObjectOriented:       true,
		SourceFileExtensions: []string{".rb"},
	}
}

func (l *rubyLanguage) Language() *sitter.Language {
	return ruby.GetLanguage()
}

func (l *rubyLanguage) Resolvers() core.LanguageResolvers {
	return &rubyResolvers{}
}
