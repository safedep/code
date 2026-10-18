package lang

import (
	"fmt"
	"path/filepath"
	"slices"
	"sync"

	"github.com/safedep/code/core"
	"github.com/safedep/code/langcode"
)

// constructors builds the language of each code. langcode.All lists the
// codes, so a new language needs an entry here and nowhere else in lang.
var constructors = map[langcode.Code]func() (core.Language, error){
	langcode.Python:     func() (core.Language, error) { return NewPythonLanguage() },
	langcode.Javascript: func() (core.Language, error) { return NewJavascriptLanguage() },
	langcode.Go:         func() (core.Language, error) { return NewGoLanguage() },
	langcode.Java:       func() (core.Language, error) { return NewJavaLanguage() },
	langcode.Typescript: func() (core.Language, error) { return NewTypescriptLanguage() },
	langcode.CSharp:     func() (core.Language, error) { return NewCSharpLanguage() },
	langcode.Rust:       func() (core.Language, error) { return NewRustLanguage() },
	langcode.PHP:        func() (core.Language, error) { return NewPHPLanguage() },
	langcode.Ruby:       func() (core.Language, error) { return NewRubyLanguage() },
	langcode.Kotlin:     func() (core.Language, error) { return NewKotlinLanguage() },
}

// sharedLanguages builds each language once, in the order of langcode.All. A
// language keeps no state, so every caller shares one value.
var sharedLanguages = sync.OnceValues(func() ([]core.Language, error) {
	langs := make([]core.Language, 0, len(constructors))
	for _, code := range langcode.All() {
		newLanguage, ok := constructors[code]
		if !ok {
			return nil, fmt.Errorf("no language for the code %q", code)
		}
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
