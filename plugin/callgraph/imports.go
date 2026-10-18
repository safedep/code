package callgraph

import (
	"cmp"
	"slices"
	"strings"

	"github.com/safedep/code/core/ast"
	sitter "github.com/smacker/go-tree-sitter"
)

// Import is one import of a file. Module and Item are as the file writes
// them. Alias is the name that the import binds in the file, as os for
// import os.path in Python, and is empty when the import binds no name.
// Namespace is the "//" form of what the import binds: the module, or the
// item in the module. A wildcard import binds every name below Namespace.
// Position is the position of the module name.
type Import struct {
	Module    string
	Item      string
	Alias     string
	Namespace string
	Wildcard  bool
	Position  ast.NodePosition
}

// binding is a name that an import binds and its namespace. In Python,
// from os import listdir as listdirfn binds listdirfn to os//listdir. A
// wildcard binding has no identifier and the namespace module//*.
type binding struct {
	identifier     string
	identifierNode *sitter.Node
	namespace      string
	namespaceNode  *sitter.Node
}

// parseImports returns the imports in source order, the names that they
// bind and the wildcard bindings. A later import of a name replaces the
// earlier binding, at the place of the earlier one.
func parseImports(imports []*ast.ImportNode, profile *languageProfile) ([]Import, []binding, []binding) {
	imports = slices.Clone(imports)
	slices.SortStableFunc(imports, func(a, b *ast.ImportNode) int {
		return cmp.Compare(a.GetModuleNameNode().StartByte(), b.GetModuleNameNode().StartByte())
	})

	records := make([]Import, 0, len(imports))
	bindings := []binding{}
	bound := make(map[string]int)
	wildcards := []binding{}

	bind := func(b binding) {
		if i, exists := bound[b.identifier]; exists {
			bindings[i] = b
			return
		}
		bound[b.identifier] = len(bindings)
		bindings = append(bindings, b)
	}

	for _, imp := range imports {
		moduleNode := imp.GetModuleNameNode()
		moduleNamespace := profile.moduleNamespace(imp.ModuleName())
		record := Import{
			Module:    imp.ModuleName(),
			Item:      imp.ModuleItem(),
			Namespace: moduleNamespace,
			Wildcard:  imp.IsWildcardImport(),
			Position:  ast.GetNodePosition(moduleNode),
		}

		switch root, rootBound := profile.boundRoot(imp, moduleNamespace); {
		case record.Wildcard:
			wildcards = append(wildcards, binding{
				namespace:     moduleNamespace + namespaceSeparator + "*",
				namespaceNode: moduleNode.Parent(),
			})
		case rootBound:
			for _, namespace := range []string{moduleNamespace, root} {
				bind(binding{identifier: namespace, identifierNode: moduleNode, namespace: namespace, namespaceNode: moduleNode.Parent()})
			}
			record.Alias, record.Namespace = root, root
		default:
			if item := imp.ModuleItem(); item != "" && item != profile.defaultExport {
				record.Namespace = moduleNamespace + namespaceSeparator + profile.moduleNamespace(item)
			}

			b := binding{identifier: moduleNamespace, identifierNode: moduleNode, namespace: record.Namespace, namespaceNode: moduleNode.Parent()}
			if alias := profile.lastSegment(imp.ModuleAlias()); alias != "" {
				b.identifier, b.identifierNode = alias, imp.GetModuleAliasNode()
				record.Alias = alias
			} else if item := profile.lastSegment(imp.ModuleItem()); item != "" {
				b.identifier, b.identifierNode = item, imp.GetModuleItemNode()
				record.Alias = item
			}
			bind(b)
		}

		records = append(records, record)
	}

	return records, bindings, wildcards
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
