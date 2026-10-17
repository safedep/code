package projectgraph

import (
	"strings"

	"github.com/safedep/code/langcode"
	"github.com/safedep/code/projectgraph/modules"
)

var javascriptProfile = ecmascriptProfile(langcode.Javascript, false)

var typescriptProfile = ecmascriptProfile(langcode.Typescript, true)

func ecmascriptProfile(code langcode.Code, typescript bool) profile {
	return profile{
		code:            code,
		family:          langcode.Javascript,
		resolver:        func(_ *builder, sp *space) modules.Resolver { return sp.keyed(sp.js.For(typescript)) },
		dependencyCode:  true,
		instance:        []string{"this"},
		superCalls:      true,
		literals:        map[string]bool{"undefined": true, "null": true, "true": true, "false": true, "NaN": true, "Infinity": true},
		defaultExport:   "default",
		openBases:       true,
		structuralTypes: true,
		memberFields:    true,
	}
}

// isDeclarationFile reports whether a TypeScript file only declares types.
func isDeclarationFile(p string) bool {
	return strings.HasSuffix(p, ".d.ts")
}
