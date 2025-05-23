package depsusage

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/safedep/code/core"
	"github.com/safedep/code/pkg/helpers"
)

var goModuleQualifierVersionSuffixRegexp *regexp.Regexp

func init() {
	goModuleQualifierVersionSuffixRegexp = regexp.MustCompile(`v\d+$`)
}

// resolvePackageHint returns the package name hint for an imported module
//
// eg. for a python module "foo.bar" it should return "foo"
func resolvePackageHint(moduleName string, lang core.Language) (string, error) {
	resolvers := map[core.LanguageCode]func(string) (string, error){
		core.LanguageCodePython:     resolvePythonPackageHint,
		core.LanguageCodeGo:         resolveGoPackageHint,
		core.LanguageCodeJavascript: resolveJavascriptPackageHint,
		core.LanguageCodeJava:       resolveJavaPackageHint,
	}
	if resolver, ok := resolvers[lang.Meta().Code]; ok {
		return resolver(moduleName)
	}
	return moduleName, nil
}

func resolvePythonPackageHint(moduleName string) (string, error) {
	if moduleName == "" {
		return "", fmt.Errorf("invalid module name: %s", moduleName)
	}
	// @TODO - Resolve package name for popular top level modules
	// eg. yaml -> pyyaml, usb -> pyusb
	if strings.Contains(moduleName, ".") {
		return moduleName[:strings.Index(moduleName, ".")], nil
	}
	return moduleName, nil
}

func resolveGoPackageHint(moduleName string) (string, error) {
	moduleName = strings.Trim(moduleName, "/")
	if moduleName == "" {
		return "", fmt.Errorf("invalid module name: %s", moduleName)
	}

	parts := strings.Split(moduleName, "/")
	if len(parts) == 0 {
		return "", fmt.Errorf("invalid module name: %s", moduleName)
	}

	domain := strings.Trim(parts[0], " ")

	// For standard library packages, return the root module as hint
	if _, exists := helpers.GoStdLibs[domain]; exists {
		return domain, nil
	}

	domainWiseQualifierCount := map[string]int{
		"github.com":    3,
		"bitbucket.org": 3,
		"gopkg.in":      3,
		"gocv.io":       3,
		"golang.org":    3,
		"go.etcd.io":    3,
	}

	if qualifiers, ok := domainWiseQualifierCount[domain]; ok {
		// If suffixed with a version eg. /v2, /v3 etc include it in the hint too
		if len(parts) > qualifiers && goModuleQualifierVersionSuffixRegexp.MatchString(parts[qualifiers]) {
			return strings.Join(parts[:qualifiers+1], "/"), nil
		}
		return strings.Join(parts[:min(qualifiers, len(parts))], "/"), nil
	}

	// For misc domains, use the first two qualifiers as hint
	if len(parts) >= 2 {
		if len(parts) > 2 && goModuleQualifierVersionSuffixRegexp.MatchString(parts[2]) {
			return strings.Join(parts[:3], "/"), nil
		}
		return strings.Join(parts[:2], "/"), nil
	}

	return moduleName, nil
}

func resolveJavascriptPackageHint(moduleName string) (string, error) {
	moduleName = strings.Trim(moduleName, "/")
	if moduleName == "" {
		return "", fmt.Errorf("invalid module name: %s", moduleName)
	}

	parts := strings.Split(moduleName, "/")

	// handle imports like "./utils", "../utils", "../../utils" etc
	if strings.HasPrefix(moduleName, ".") {
		return moduleName, nil
	}

	// handle scoped packages like "@fortawesome/fa-icon-chooser-react"
	if strings.HasPrefix(moduleName, "@") {
		return strings.Join(parts[:min(2, len(parts))], "/"), nil
	}

	return parts[0], nil
}

func resolveJavaPackageHint(moduleName string) (string, error) {
	if moduleName == "" {
		return "", fmt.Errorf("invalid module name: %s", moduleName)
	}

	builtinJavaPackageRoots := []string{"java", "jdk"}

	// If the module name starts with a builtin package root, return package name with root and next qualifier
	// eg. java.lang.Math.PI -> java.lang
	for _, root := range builtinJavaPackageRoots {
		parts := strings.Split(moduleName, ".")
		if strings.HasPrefix(moduleName, root) {
			return strings.Join(parts[:min(2, len(parts))], "."), nil
		}
	}

	// For other packages, we can't determine a hint deterministically without known external sources
	return "", fmt.Errorf("unable to resolve package hint for module: %s", moduleName)
}
