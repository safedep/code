package callgraph

import (
	"embed"
	"encoding/json"

	"github.com/safedep/code/core"
	"github.com/safedep/dry/log"
)

//go:embed builtins.json
var builtinsFS embed.FS

// languageBuiltins holds built-in functions for each language
type languageBuiltins map[string][]string

var allBuiltins languageBuiltins

// Loads built-in functions from the embedded JSON file
func initBuiltins() {
	// Read the builtins.json file
	data, err := builtinsFS.ReadFile("builtins.json")
	if err != nil {
		log.Errorf("failed to read builtins.json: %v", err)
		panic(err)
	}

	// Parse the JSON
	if err := json.Unmarshal(data, &allBuiltins); err != nil {
		log.Errorf("failed to unmarshal builtins.json: %v", err)
		panic(err)
	}
}

func getBuiltins(lang core.Language) []string {
	builtins, ok := allBuiltins[string(lang.Meta().Code)]
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
