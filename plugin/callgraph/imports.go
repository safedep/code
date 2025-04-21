package callgraph

import (
	"strings"

	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
	"github.com/safedep/code/pkg/helpers"
)

// Parses namespaces for imported identifiers
// eg. import pprint is parsed as:
// pprint -> pprint
// eg. from os import listdir as listdirfn, chmod is parsed as:
// listdirfn -> os//listdir
// chmod -> os//chmod
func parseImportedIdentifierNamespaces(imports []*ast.ImportNode, lang core.Language) map[string]string {
	importedIdentifierNamespaces := make(map[string]string)
	for _, imp := range imports {
		if imp.IsWildcardImport() {
			continue
		}
		itemNamespace := imp.ModuleItem()
		moduleNamespace := resolveNamespaceWithSeparator(imp.ModuleName(), lang)
		if itemNamespace == "" {
			itemNamespace = moduleNamespace
		} else {
			itemNamespace = moduleNamespace + namespaceSeparator + itemNamespace
		}

		moduleItemIdentifierKey := resolveSubmoduleIdentifier(imp.ModuleItem(), lang)
		moduleAliasIdentifierKey := resolveSubmoduleIdentifier(imp.ModuleAlias(), lang)
		identifierKey := helpers.GetFirstNonEmptyString(moduleAliasIdentifierKey, moduleItemIdentifierKey, moduleNamespace)
		importedIdentifierNamespaces[identifierKey] = itemNamespace
	}
	return importedIdentifierNamespaces
}

// For submodule imports, we need to replace separator with our namespaceSeparator for consistency
// eg. in python "from os.path import abspath" -> ModuleName = os.path -> os//path
var submoduleSeparator = map[core.LanguageCode]string{
	core.LanguageCodeGo:         "/",
	core.LanguageCodeJavascript: "/",
	core.LanguageCodePython:     ".",
}

func resolveNamespaceWithSeparator(moduleName string, lang core.Language) string {
	separator, exists := submoduleSeparator[lang.Meta().Code]
	if exists {
		return strings.Join(strings.Split(moduleName, separator), namespaceSeparator)
	}
	return moduleName
}

func resolveSubmoduleIdentifier(identifier string, lang core.Language) string {
	separator, exists := submoduleSeparator[lang.Meta().Code]
	if exists && strings.Contains(identifier, separator) {
		parts := strings.Split(identifier, separator)
		return parts[len(parts)-1]
	}
	return identifier
}
