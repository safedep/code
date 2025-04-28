package callgraph

import "github.com/safedep/code/core"

var instanceKeywordMapping = map[string]string{
	"python":      "self",
	"javascript":  "this",
	"java":        "this",
	"csharp":      "this",
	"ruby":        "self",
	"php":         "$this",
	"golang":      "this",
	"typescript":  "this",
	"swift":       "self",
	"rust":        "self",
	"scala":       "this",
	"objective-c": "self",
	"dart":        "this",
	"elixir":      "this",
	"clojure":     "this",
	"lua":         "self",
	"perl":        "self",
	"r":           "this",
}

func resolveInstanceKeyword(language core.Language) (string, bool) {
	langCode := language.Meta().Code
	if keyword, exists := instanceKeywordMapping[string(langCode)]; exists {
		return keyword, true
	}
	return "", false
}
