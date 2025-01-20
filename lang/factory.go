package lang

import (
	"fmt"
	"path/filepath"
	"slices"

	"github.com/safedep/code/core"
)

var languages = map[string]func() (core.Language, error){
	"python": func() (core.Language, error) {
		return NewPythonLanguage()
	},
	"javascript": func() (core.Language, error) {
		return NewJavascriptLanguage()
	},
}

func GetLanguage(name string) (core.Language, error) {
	if f, ok := languages[name]; ok {
		return f()
	}

	return nil, fmt.Errorf("language not found: %s", name)
}

func ResolveLanguage(filePath string) (core.Language, error) {
	extension := filepath.Ext(filePath)

	for _, f := range languages {
		l, err := f()
		if err != nil {
			return nil, err
		}

		if slices.Contains(l.Meta().SourceFileExtensions, extension) {
			return l, nil
		}
	}
	return nil, fmt.Errorf("language not found for file: %s", filePath)
}
