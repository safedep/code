package callgraph

import (
	"strings"

	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
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
			finalisedNamespace = moduleNamespace + namespaceSeparator + finalisedNamespace
		}

		moduleItemIdentifierKey := resolveSubmoduleIdentifier(imp.ModuleItem(), lang)
		moduleAliasIdentifierKey := resolveSubmoduleIdentifier(imp.ModuleAlias(), lang)

		identifierKey := moduleNamespace
		identifierTreeNode := imp.GetModuleNameNode()
		if moduleAliasIdentifierKey != "" {
			identifierKey = moduleAliasIdentifierKey
			identifierTreeNode = imp.GetModuleAliasNode()
		} else if moduleItemIdentifierKey != "" {
			identifierKey = moduleItemIdentifierKey
			identifierTreeNode = imp.GetModuleItemNode()
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
var submoduleSeparator = map[core.LanguageCode]string{
	core.LanguageCodeGo:         "/",
	core.LanguageCodeJavascript: "/",
	core.LanguageCodePython:     ".",
	core.LanguageCodeJava:       ".",
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
