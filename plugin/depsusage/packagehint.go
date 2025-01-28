package depsusage

import (
	"strings"

	"github.com/safedep/code/core"
)

// resolvePackageHint returns the package name hint for an imported module
//
// eg. for a python module "foo.bar" it should return "foo"
func resolvePackageHint(moduleName string, lang core.Language) string {
	resolvers := map[core.LanguageCode]func(string) string{
		core.LanguageCodePython: resolvePythonPackageHint,
	}
	if resolver, ok := resolvers[lang.Meta().Code]; ok {
		return resolver(moduleName)
	}
	return moduleName
}

func resolvePythonPackageHint(s string) string {
	if s == "" {
		return ""
	}
	// @TODO - Resolve package name for popular top level modules
	// eg. yaml -> pyyaml, usb -> pyusb
	if strings.Contains(s, ".") {
		return s[:strings.Index(s, ".")]
	}
	return s
}
