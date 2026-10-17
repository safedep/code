package lang

import (
	"fmt"
	"path/filepath"
	"slices"
	"sync"

	"github.com/safedep/code/core"
)

// sharedLanguages builds each language once, in a fixed order. A language
// keeps no state, so every caller shares one value.
var sharedLanguages = sync.OnceValues(func() ([]core.Language, error) {
	constructors := []func() (core.Language, error){
		func() (core.Language, error) { return NewPythonLanguage() },
		func() (core.Language, error) { return NewJavascriptLanguage() },
		func() (core.Language, error) { return NewGoLanguage() },
		func() (core.Language, error) { return NewJavaLanguage() },
		func() (core.Language, error) { return NewTypescriptLanguage() },
		func() (core.Language, error) { return NewCSharpLanguage() },
		func() (core.Language, error) { return NewRustLanguage() },
		func() (core.Language, error) { return NewPHPLanguage() },
		func() (core.Language, error) { return NewRubyLanguage() },
		func() (core.Language, error) { return NewKotlinLanguage() },
	}

	langs := make([]core.Language, 0, len(constructors))
	for _, newLanguage := range constructors {
		l, err := newLanguage()
		if err != nil {
			return nil, fmt.Errorf("failed to get language: %w", err)
		}
		langs = append(langs, l)
	}
	return langs, nil
})

// languagesByExtension maps each file extension to its shared language.
var languagesByExtension = sync.OnceValues(func() (map[string]core.Language, error) {
	langs, err := sharedLanguages()
	if err != nil {
		return nil, err
	}

	byExtension := make(map[string]core.Language)
	for _, l := range langs {
		for _, ext := range l.Meta().SourceFileExtensions {
			byExtension[ext] = l
		}
	}
	return byExtension, nil
})

// AllLanguages returns the shared value of every language, in a fixed order.
func AllLanguages() ([]core.Language, error) {
	langs, err := sharedLanguages()
	if err != nil {
		return nil, err
	}
	return slices.Clone(langs), nil
}

// GetLanguage returns the shared core.Language for the given language code
func GetLanguage(languageCode string) (core.Language, error) {
	langs, err := sharedLanguages()
	if err != nil {
		return nil, err
	}

	for _, l := range langs {
		if l.Meta().Code == core.LanguageCode(languageCode) {
			return l, nil
		}
	}
	return nil, fmt.Errorf("language not found: %s", languageCode)
}

// ResolveLanguageFromPath resolves the programming language from the
// filePath and returns the shared core.Language and a boolean indicating if
// the language implementation exists for the specified file extension in
// filePath.
//
// It returns nil, false if the file extension is not supported by any implemented language.
func ResolveLanguageFromPath(filePath string) (core.Language, bool) {
	byExtension, err := languagesByExtension()
	if err != nil {
		return nil, false
	}

	l, ok := byExtension[filepath.Ext(filePath)]
	return l, ok
}
