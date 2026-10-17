package callgraph

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
