package depsusage

import (
	"fmt"

	"github.com/safedep/code/core"
	sitter "github.com/smacker/go-tree-sitter"
)

// usageProfile is what the usage plugin knows of one language.
type usageProfile struct {
	code core.LanguageCode

	// evidentNodeTypes are the node types of a name that can use an import.
	evidentNodeTypes map[string]bool

	// ignoredNodeTypes are the node types that hold no usage, as an import.
	ignoredNodeTypes map[string]bool

	// ignoreRules find the other nodes that hold no usage.
	ignoreRules []ignoreRule

	// packageHint gives the package of a module name. With no packageHint,
	// the hint is the module name.
	packageHint func(moduleName string) (string, error)
}

// ignoreRule reports that the walk must skip node.
type ignoreRule func(node *sitter.Node, data *[]byte) bool

var identifierNodeTypes = map[string]bool{"identifier": true, "type_identifier": true}

// usageProfiles are the languages of the usage plugin, in a fixed order.
var usageProfiles = []*usageProfile{
	{
		code:             core.LanguageCodePython,
		evidentNodeTypes: identifierNodeTypes,
		ignoredNodeTypes: map[string]bool{"import_statement": true, "import_from_statement": true},
		packageHint:      resolvePythonPackageHint,
	},
	{
		code:             core.LanguageCodeGo,
		evidentNodeTypes: identifierNodeTypes,
		ignoredNodeTypes: map[string]bool{"import_declaration": true},
		packageHint:      resolveGoPackageHint,
	},
	{
		code:             core.LanguageCodeJavascript,
		evidentNodeTypes: identifierNodeTypes,
		ignoredNodeTypes: map[string]bool{"import_statement": true},
		ignoreRules:      []ignoreRule{isRequireCallIgnoreRule},
		packageHint:      resolveJavascriptPackageHint,
	},
	{
		code:             core.LanguageCodeJava,
		evidentNodeTypes: identifierNodeTypes,
		ignoredNodeTypes: map[string]bool{"import_declaration": true},
		packageHint:      resolveJavaPackageHint,
	},
	{
		code:             core.LanguageCodeTypescript,
		evidentNodeTypes: identifierNodeTypes,
		ignoredNodeTypes: map[string]bool{
			"import_statement":       true,
			"type_alias_declaration": true,
			"interface_declaration":  true,
		},
		ignoreRules: []ignoreRule{isRequireCallIgnoreRule},
		packageHint: resolveJavascriptPackageHint,
	},
	{
		code:             core.LanguageCodeCSharp,
		evidentNodeTypes: identifierNodeTypes,
		ignoredNodeTypes: map[string]bool{"using_directive": true},
	},
	{
		code:             core.LanguageCodeRust,
		evidentNodeTypes: identifierNodeTypes,
		ignoredNodeTypes: map[string]bool{"use_declaration": true, "extern_crate_declaration": true},
		packageHint:      resolveRustPackageHint,
	},
	{
		code:             core.LanguageCodePHP,
		evidentNodeTypes: map[string]bool{"name": true},
		// A use declaration imports a namespace, or a trait into a class.
		ignoredNodeTypes: map[string]bool{"namespace_use_declaration": true, "use_declaration": true},
	},
	{
		code:             core.LanguageCodeRuby,
		evidentNodeTypes: identifierNodeTypes,
		packageHint:      resolveRubyPackageHint,
	},
	{
		code:             core.LanguageCodeKotlin,
		evidentNodeTypes: map[string]bool{"simple_identifier": true, "type_identifier": true},
		ignoredNodeTypes: map[string]bool{"import_list": true},
		packageHint:      resolveKotlinPackageHint,
	},
}

var usageProfilesByCode = indexUsageProfiles(usageProfiles)

func indexUsageProfiles(profiles []*usageProfile) map[core.LanguageCode]*usageProfile {
	byCode := make(map[core.LanguageCode]*usageProfile, len(profiles))
	for _, profile := range profiles {
		byCode[profile.code] = profile
	}
	return byCode
}

func supportedLanguages() []core.LanguageCode {
	codes := make([]core.LanguageCode, len(usageProfiles))
	for i, profile := range usageProfiles {
		codes[i] = profile.code
	}
	return codes
}

// usageProfileOf returns the profile of code. A language with no profile
// gets the identifier node types and no ignore rules.
func usageProfileOf(code core.LanguageCode) *usageProfile {
	if profile, ok := usageProfilesByCode[code]; ok {
		return profile
	}
	return &usageProfile{code: code, evidentNodeTypes: identifierNodeTypes}
}

// commonIgnoredTypes are the node types that hold no usage in any grammar.
var commonIgnoredTypes = map[string]bool{"comment": true}

func (p *usageProfile) ignores(node *sitter.Node, data *[]byte) bool {
	if commonIgnoredTypes[node.Type()] || p.ignoredNodeTypes[node.Type()] {
		return true
	}
	for _, rule := range p.ignoreRules {
		if rule(node, data) {
			return true
		}
	}
	return false
}

// packageHintOf returns the package name hint of an imported module, as foo
// for the Python module foo.bar.
func (p *usageProfile) packageHintOf(moduleName string) (string, error) {
	if moduleName == "" {
		return "", fmt.Errorf("invalid module name: %s", moduleName)
	}
	if p.packageHint == nil {
		return moduleName, nil
	}
	return p.packageHint(moduleName)
}
