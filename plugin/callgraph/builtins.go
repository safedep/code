package callgraph

import (
	"embed"
	"encoding/json"
	"fmt"

	"github.com/safedep/code/core"
	"github.com/safedep/dry/log"
)

//go:embed builtins.json
var builtinsFS embed.FS

// languageBuiltins holds built-in functions for each language
type languageBuiltins map[string]map[string]string

var allBuiltins languageBuiltins

// LoadBuiltins loads built-in functions from the embedded JSON file
func init() {
	fmt.Println("Loading built-ins...")

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

func GetBuiltins(lang core.Language) map[string]string {
	builtins, ok := allBuiltins[string(lang.Meta().Code)]
	if !ok {
		log.Debugf("No built-ins defined for language %s", lang.Meta().Code)
		return map[string]string{}
	}
	return builtins
}
