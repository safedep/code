package lang

import (
	"github.com/safedep/code/core"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"
)

const pythonLanguageName = "python"

type pythonLanguage struct{}

var _ core.Language = (*pythonLanguage)(nil)

func NewPythonLanguage() (*pythonLanguage, error) {
	return &pythonLanguage{}, nil
}

func (l *pythonLanguage) Name() string {
	return pythonLanguageName
}

func (l *pythonLanguage) Meta() core.LanguageMeta {
	return core.LanguageMeta{
		Name:                 pythonLanguageName,
		Code:                 core.LanguageCodePython,
		ObjectOriented:       true,
		SourceFileExtensions: []string{".py"},
	}
}

func (l *pythonLanguage) Language() *sitter.Language {
	return python.GetLanguage()
}

func (l *pythonLanguage) Resolvers() core.LanguageResolvers {
	return &pythonResolvers{
		language: l,
	}
}
