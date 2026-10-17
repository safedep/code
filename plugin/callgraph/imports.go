package callgraph

import (
	"strings"

	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
	"github.com/safedep/code/pkg/helpers"
	sitter "github.com/smacker/go-tree-sitter"
)

// Parses namespaces & sitter nodes for imported identifiers
// eg. import pprint is parsed as:
// pprint -> pprint
// eg. from os import listdir as listdirfn, chmod is parsed as:
// listdirfn -> os//listdir
// chmod -> os//chmod
type parsedImport struct {
	Identifier         string
	IdentifierTreeNode *sitter.Node
	Namespace          string
	NamespaceTreeNode  *sitter.Node
}
type wildcardImport struct {
	Namespace         string
	NamespaceTreeNode *sitter.Node
}

// parseImports returns the imported identifiers and the wildcard imports, in
// source order. A later import of an identifier replaces the earlier one, at
// the place of the earlier one.
func parseImports(imports []*ast.ImportNode, code core.LanguageCode) ([]parsedImport, []wildcardImport) {
	importedIdentifiers := []parsedImport{}
	importedIndex := make(map[string]int)
	wildcardImports := []wildcardImport{}

	for _, imp := range imports {
		moduleNamespace := resolveNamespaceWithSeparator(imp.ModuleName(), code)

		if imp.IsWildcardImport() {
			wildcardImports = append(wildcardImports, wildcardImport{
				Namespace:         moduleNamespace + namespaceSeparator + "*",
				NamespaceTreeNode: imp.GetModuleNameNode().Parent(),
			})
			continue
		}

		finalisedNamespace := imp.ModuleItem()

		// If not imported as item, consider entire module namespace
		if finalisedNamespace == "" {
			finalisedNamespace = moduleNamespace
		} else {
			finalisedNamespace = moduleNamespace + namespaceSeparator + resolveNamespaceWithSeparator(finalisedNamespace, code)
		}

		moduleItemIdentifierKey := resolveSubmoduleIdentifier(imp.ModuleItem(), code)
		moduleAliasIdentifierKey := resolveSubmoduleIdentifier(imp.ModuleAlias(), code)
		if code == core.LanguageCodeGo && imp.ModuleAlias() == imp.ModuleName() {
			// A Go import with no alias has the module as its alias node
			moduleAliasIdentifierKey = helpers.GoPackageName(imp.ModuleName())
		}

		identifierKey := moduleNamespace
		identifierTreeNode := imp.GetModuleNameNode()
		if moduleAliasIdentifierKey != "" {
			identifierKey = moduleAliasIdentifierKey
			identifierTreeNode = imp.GetModuleAliasNode()
		} else if moduleItemIdentifierKey != "" {
			identifierKey = moduleItemIdentifierKey
			identifierTreeNode = imp.GetModuleItemNode()
		} else if code == core.LanguageCodeGo {
			// For Go, when there's no explicit alias, use the package name of the import path
			// e.g., "net/http" -> http
			identifierKey = helpers.GoPackageName(imp.ModuleName())
		}

		parsed := parsedImport{
			Identifier:         identifierKey,
			IdentifierTreeNode: identifierTreeNode,
			Namespace:          finalisedNamespace,
			NamespaceTreeNode:  imp.GetModuleNameNode().Parent(), // The parent node is the entire module import node
		}
		if i, exists := importedIndex[identifierKey]; exists {
			importedIdentifiers[i] = parsed
			continue
		}
		importedIndex[identifierKey] = len(importedIdentifiers)
		importedIdentifiers = append(importedIdentifiers, parsed)
	}

	return importedIdentifiers, wildcardImports
}

// For submodule imports, we need to replace separator with our namespaceSeparator for consistency
// eg. in python "from os.path import abspath" -> ModuleName = os.path -> os//path
// submoduleSeparators are the separators of a qualified name in a language,
// as in os.path or OpenAI::Client.new.
var submoduleSeparators = map[core.LanguageCode][]string{
	core.LanguageCodeGo:         {"/"},
	core.LanguageCodeJavascript: {"/"},
	core.LanguageCodePython:     {"."},
	core.LanguageCodeJava:       {"."},
	core.LanguageCodeTypescript: {"/"},
	core.LanguageCodeCSharp:     {"."},
	core.LanguageCodeRust:       {"::", "."},
	core.LanguageCodePHP:        {"\\", "::", "->"},
	core.LanguageCodeRuby:       {"::", "."},
}

// splitQualifiedName splits a qualified name at every separator of the
// language.
func splitQualifiedName(name string, code core.LanguageCode) []string {
	parts := []string{name}
	for _, separator := range submoduleSeparators[code] {
		var split []string
		for _, part := range parts {
			split = append(split, strings.Split(part, separator)...)
		}
		parts = split
	}
	return parts
}

func resolveNamespaceWithSeparator(moduleName string, code core.LanguageCode) string {
	// Strip quotes for Go string literals
	if code == core.LanguageCodeGo {
		moduleName = strings.Trim(moduleName, "\"")
	}

	// A fully qualified PHP name, as in use \GuzzleHttp\Client, has a
	// leading separator that names no namespace.
	if code == core.LanguageCodePHP {
		moduleName = strings.TrimPrefix(moduleName, `\`)
	}

	return strings.Join(splitQualifiedName(moduleName, code), namespaceSeparator)
}

func resolveSubmoduleIdentifier(identifier string, code core.LanguageCode) string {
	// Strip quotes for Go string literals
	if code == core.LanguageCodeGo {
		identifier = strings.Trim(identifier, "\"")
	}

	parts := splitQualifiedName(identifier, code)
	return parts[len(parts)-1]
}
