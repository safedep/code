package depsusage

import (
	"regexp"
	"strings"

	"github.com/safedep/code/core"
)

// resolvePackageHint returns the package name hint for an imported module
//
// eg. for a python module "foo.bar" it should return "foo"
func resolvePackageHint(moduleName string, lang core.Language) string {
	resolvers := map[core.LanguageCode]func(string) string{
		core.LanguageCodePython:     resolvePythonPackageHint,
		core.LanguageCodeGo:         resolveGoPackageHint,
		core.LanguageCodeJavascript: reolveJavascriptPackageHint,
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

func resolveGoPackageHint(moduleName string) string {
	moduleName = strings.Trim(moduleName, "/")
	if moduleName == "" {
		return ""
	}

	parts := strings.Split(moduleName, "/")
	if len(parts) == 0 {
		return ""
	}

	domain := strings.Trim(parts[0], " ")

	domainWiseQualifierCount := map[string]int{
		"github.com":    3,
		"bitbucket.org": 3,
		"gopkg.in":      3,
		"gocv.io":       3,
		"golang.org":    3,
		"go.etcd.io":    3,
	}

	versionSuffixRegexp := regexp.MustCompile(`v\d+$`)

	if qualifiers, ok := domainWiseQualifierCount[domain]; ok {
		// If suffixed with a version eg. /v2, /v3 etc include it in the hint too
		if len(parts) > qualifiers && versionSuffixRegexp.MatchString(parts[qualifiers]) {
			return strings.Join(parts[:qualifiers+1], "/")
		}
		return strings.Join(parts[:min(qualifiers, len(parts))], "/")
	}

	// For misc domains, use the first two qualifiers as hint
	if len(parts) >= 2 {
		if len(parts) > 2 && versionSuffixRegexp.MatchString(parts[2]) {
			return strings.Join(parts[:3], "/")
		}
		return strings.Join(parts[:2], "/")
	}

	return moduleName
}

func reolveJavascriptPackageHint(moduleName string) string {
	moduleName = strings.Trim(moduleName, "/")
	if moduleName == "" {
		return ""
	}

	parts := strings.Split(moduleName, "/")

	// handle imports like "./utils", "../utils", "../../utils" etc
	if strings.HasPrefix(moduleName, ".") {
		return moduleName
	}

	// handle scoped packages like "@fortawesome/fa-icon-chooser-react"
	if strings.HasPrefix(moduleName, "@") {
		return strings.Join(parts[:min(2, len(parts))], "/")
	}

	return parts[0]
}
