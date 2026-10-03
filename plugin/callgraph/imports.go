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

// Parses the imports from AST and returns map of identified imports and a list of wildcard imports.
func parseImports(imports []*ast.ImportNode, lang core.Language) (map[string]parsedImport, []wildcardImport) {
	importedIdentifierNamespaces := make(map[string]parsedImport)
	wildcardImports := []wildcardImport{}

	for _, imp := range imports {
		moduleNamespace := resolveNamespaceWithSeparator(imp.ModuleName(), lang)

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
			finalisedNamespace = moduleNamespace + namespaceSeparator + resolveNamespaceWithSeparator(finalisedNamespace, lang)
		}

		moduleItemIdentifierKey := resolveSubmoduleIdentifier(imp.ModuleItem(), lang)
		moduleAliasIdentifierKey := resolveSubmoduleIdentifier(imp.ModuleAlias(), lang)
		if lang.Meta().Code == core.LanguageCodeGo && imp.ModuleAlias() == imp.ModuleName() {
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
		} else if lang.Meta().Code == core.LanguageCodeGo {
			// For Go, when there's no explicit alias, use the package name of the import path
			// e.g., "net/http" -> http
			identifierKey = helpers.GoPackageName(imp.ModuleName())
		}

		importedIdentifierNamespaces[identifierKey] = parsedImport{
			Identifier:         identifierKey,
			IdentifierTreeNode: identifierTreeNode,
			Namespace:          finalisedNamespace,
			NamespaceTreeNode:  imp.GetModuleNameNode().Parent(), // The parent node is the entire module import node
		}
	}

	return importedIdentifierNamespaces, wildcardImports
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

func resolveNamespaceWithSeparator(moduleName string, lang core.Language) string {
	// Strip quotes for Go string literals
	if lang.Meta().Code == core.LanguageCodeGo {
		moduleName = strings.Trim(moduleName, "\"")
	}

	return strings.Join(splitQualifiedName(moduleName, lang.Meta().Code), namespaceSeparator)
}

func resolveSubmoduleIdentifier(identifier string, lang core.Language) string {
	// Strip quotes for Go string literals
	if lang.Meta().Code == core.LanguageCodeGo {
		identifier = strings.Trim(identifier, "\"")
	}

	parts := splitQualifiedName(identifier, lang.Meta().Code)
	return parts[len(parts)-1]
}
