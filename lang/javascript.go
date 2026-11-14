package lang

import (
	"slices"

	"github.com/safedep/code/core"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
)

const javascriptLanguageName = "javascript"

type javascriptLanguage struct {
	// typescript and tsx parse the script blocks of a component file that
	// name TypeScript.
	typescript core.Language
	tsx        core.Language
}

var _ core.Language = (*javascriptLanguage)(nil)
var _ core.SourceSelector = (*javascriptLanguage)(nil)

func NewJavascriptLanguage() (*javascriptLanguage, error) {
	typescript, err := NewTypescriptWithoutJSXLanguage()
	if err != nil {
		return nil, err
	}
	tsx, err := NewTypescriptLanguage()
	if err != nil {
		return nil, err
	}
	return &javascriptLanguage{typescript: typescript, tsx: tsx}, nil
}

func (l *javascriptLanguage) Name() string {
	return javascriptLanguageName
}

// Meta lists .jsx with the JavaScript extensions, because the JavaScript
// grammar parses JSX.
func (l *javascriptLanguage) Meta() core.LanguageMeta {
	return core.LanguageMeta{
		Name:                 javascriptLanguageName,
		Code:                 core.LanguageCodeJavascript,
		ObjectOriented:       true,
		SourceFileExtensions: append([]string{".js", ".mjs", ".cjs", ".jsx"}, componentExtensions...),
	}
}

func (l *javascriptLanguage) Language() *sitter.Language {
	return javascript.GetLanguage()
}

func (l *javascriptLanguage) Resolvers() core.LanguageResolvers {
	return &javascriptResolvers{}
}

// Source gives the script blocks of a component file to the language of
// their dialect. It gives every other file to JavaScript.
func (l *javascriptLanguage) Source(ext string, data []byte) (core.Language, []byte) {
	if !slices.Contains(componentExtensions, ext) {
		return l, data
	}
	source, dialect := componentSource(ext, data)
	switch dialect {
	case dialectTSX:
		return l.tsx, source
	case dialectTypescript:
		return l.typescript, source
	default:
		return l, source
	}
}
