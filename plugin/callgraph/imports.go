package callgraph

import (
	"strings"

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

	bind := func(parsed parsedImport) {
		if i, exists := importedIndex[parsed.Identifier]; exists {
			importedIdentifiers[i] = parsed
			return
		}
		importedIndex[parsed.Identifier] = len(importedIdentifiers)
		importedIdentifiers = append(importedIdentifiers, parsed)
	}

	for _, imp := range imports {
		moduleNamespace := profile.moduleNamespace(imp.ModuleName())

		if imp.IsWildcardImport() {
			wildcardImports = append(wildcardImports, wildcardImport{
				Namespace:         moduleNamespace + namespaceSeparator + "*",
				NamespaceTreeNode: imp.GetModuleNameNode().Parent(),
			})
			continue
		}

		if root, ok := profile.boundRoot(imp, moduleNamespace); ok {
			for _, namespace := range []string{moduleNamespace, root} {
				bind(parsedImport{
					Identifier:         namespace,
					IdentifierTreeNode: imp.GetModuleNameNode(),
					Namespace:          namespace,
					NamespaceTreeNode:  imp.GetModuleNameNode().Parent(),
				})
			}
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

		bind(parsedImport{
			Identifier:         identifierKey,
			IdentifierTreeNode: identifierTreeNode,
			Namespace:          finalisedNamespace,
			NamespaceTreeNode:  imp.GetModuleNameNode().Parent(),
		})
	}

	return importedIdentifiers, wildcardImports
}

// boundRoot returns the first segment of a dotted module that the import
// binds, as os for import os.path in Python. The resolver gives such an
// import the module name as its alias.
func (p *languageProfile) boundRoot(imp *ast.ImportNode, moduleNamespace string) (string, bool) {
	if !p.importBindsRoot || imp.ModuleItem() != "" || imp.ModuleAlias() != imp.ModuleName() {
		return "", false
	}
	root, _, dotted := strings.Cut(moduleNamespace, namespaceSeparator)
	return root, dotted
}
