package callgraph

import (
	"embed"
	"encoding/json"
	"sync"

	"github.com/safedep/code/core"
	"github.com/safedep/dry/log"
)

//go:embed builtins.json
var builtinsFS embed.FS

// languageBuiltins holds built-in functions for each language
type languageBuiltins map[string][]string

// loadBuiltins reads the built-in functions from the embedded JSON file
// once. Every build of a call graph reads them through it, so a graph built
// without NewCallGraphPlugin knows the built-ins too.
var loadBuiltins = sync.OnceValue(func() languageBuiltins {
	data, err := builtinsFS.ReadFile("builtins.json")
	if err != nil {
		log.Errorf("failed to read builtins.json: %v", err)
		panic(err)
	}

	var builtins languageBuiltins
	if err := json.Unmarshal(data, &builtins); err != nil {
		log.Errorf("failed to unmarshal builtins.json: %v", err)
		panic(err)
	}
	return builtins
})

func getBuiltins(code core.LanguageCode) []string {
	builtins, ok := loadBuiltins()[string(code)]
	if !ok {
		log.Debugf("No built-ins defined for language %s", code)
		return []string{}
	}
	return builtins
}

func getBuiltinsMap(code core.LanguageCode) map[string]bool {
	builtins := getBuiltins(code)
	builtinsMap := make(map[string]bool, len(builtins))
	for _, builtin := range builtins {
		builtinsMap[builtin] = true
	}
	return builtinsMap
}
