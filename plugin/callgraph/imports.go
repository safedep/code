package callgraph

import (
	"github.com/safedep/code/core/ast"
	sitter "github.com/smacker/go-tree-sitter"
)

// parsedImport is a name that an import binds and its namespace. In Python,
// from os import listdir as listdirfn binds listdirfn to os//listdir.
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
func parseImports(imports []*ast.ImportNode, profile *languageProfile) ([]parsedImport, []wildcardImport) {
	importedIdentifiers := []parsedImport{}
	importedIndex := make(map[string]int)
	wildcardImports := []wildcardImport{}

	for _, imp := range imports {
		moduleNamespace := profile.moduleNamespace(imp.ModuleName())

		if imp.IsWildcardImport() {
			wildcardImports = append(wildcardImports, wildcardImport{
				Namespace:         moduleNamespace + namespaceSeparator + "*",
				NamespaceTreeNode: imp.GetModuleNameNode().Parent(),
			})
			continue
		}

		finalisedNamespace := moduleNamespace
		if item := imp.ModuleItem(); item != "" {
			finalisedNamespace = moduleNamespace + namespaceSeparator + profile.moduleNamespace(item)
		}

		moduleItemIdentifierKey := profile.lastSegment(imp.ModuleItem())
		moduleAliasIdentifierKey := profile.lastSegment(imp.ModuleAlias())

		identifierKey := moduleNamespace
		identifierTreeNode := imp.GetModuleNameNode()
		if moduleAliasIdentifierKey != "" {
			identifierKey = moduleAliasIdentifierKey
			identifierTreeNode = imp.GetModuleAliasNode()
		} else if moduleItemIdentifierKey != "" {
			identifierKey = moduleItemIdentifierKey
			identifierTreeNode = imp.GetModuleItemNode()
		}

		parsed := parsedImport{
			Identifier:         identifierKey,
			IdentifierTreeNode: identifierTreeNode,
			Namespace:          finalisedNamespace,
			NamespaceTreeNode:  imp.GetModuleNameNode().Parent(),
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
