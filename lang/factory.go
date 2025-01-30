package lang

import (
	"fmt"
	"path/filepath"
	"slices"

	"github.com/safedep/code/core"
	"github.com/safedep/dry/log"
)

var languages = map[string]func() (core.Language, error){
	"python": func() (core.Language, error) {
		return NewPythonLanguage()
	},
	"javascript": func() (core.Language, error) {
		return NewJavascriptLanguage()
	},
}

func AllLanguages() ([]core.Language, error) {
	langs := make([]core.Language, 0, len(languages))
	for _, getLang := range languages {
		lang, err := getLang()
		if err != nil {
			log.Debugf("failed to get language: %v", err)
			return nil, err
		}
		langs = append(langs, lang)
	}
	return langs, nil
}

func GetLanguage(name string) (core.Language, error) {
	if f, ok := languages[name]; ok {
		return f()
	}

	return nil, fmt.Errorf("language not found: %s", name)
}

// ResolveLanguageFromPath resolves the programming language from the
// filePath and returns the core.Language and a boolean indicating if the
// language implementation exists for the specified file extension in filePath.
//
// It returns nil, false if the file extension is not supported by any implemented language.
func ResolveLanguageFromPath(filePath string) (core.Language, bool) {
	extension := filepath.Ext(filePath)

	for _, f := range languages {
		l, err := f()
		if err != nil {
			return nil, false
		}

		if slices.Contains(l.Meta().SourceFileExtensions, extension) {
			return l, true
		}
	}
	return nil, false
}
