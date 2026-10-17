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

func getBuiltins(lang core.Language) []string {
	builtins, ok := loadBuiltins()[string(lang.Meta().Code)]
	if !ok {
		log.Debugf("No built-ins defined for language %s", lang.Meta().Code)
		return []string{}
	}
	return builtins
}

func getBuiltinsMap(lang core.Language) map[string]bool {
	builtins := getBuiltins(lang)
	builtinsMap := make(map[string]bool, len(builtins))
	for _, builtin := range builtins {
		builtinsMap[builtin] = true
	}
	return builtinsMap
}
