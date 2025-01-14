package lang

import (
	"fmt"

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
