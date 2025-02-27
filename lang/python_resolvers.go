package lang

import (
	"fmt"

	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
	"github.com/safedep/code/pkg/ts"
	sitter "github.com/smacker/go-tree-sitter"
)

const pyWholeModuleImportQuery = `
	(import_statement
		name: ((dotted_name) @module_name))

	(import_statement
		name: (aliased_import
			name: ((dotted_name) @module_name)
			alias: (identifier) @module_alias))

  (import_from_statement
		module_name: (dotted_name) @module_name
		(wildcard_import) @wildcard_import)

	(import_from_statement
		module_name: (relative_import) @module_name
		(wildcard_import) @wildcard_import)
`
const pyItemImportQuery = `
	(import_from_statement
		module_name: (dotted_name) @module_name
		name: (dotted_name
			(identifier) @module_item @module_item_alias))

	(import_from_statement
		module_name: (relative_import) @module_name
		name: (dotted_name
			(identifier) @module_item @module_item_alias))

	(import_from_statement
		module_name: (dotted_name) @module_name
		name: (aliased_import
			name: (dotted_name
				(identifier) @module_item)
			 alias: (identifier) @module_item_alias))

	(import_from_statement
		module_name: (relative_import) @module_name
		name: (aliased_import
			name: ((dotted_name) @module_item)
			alias: (identifier) @module_item_alias))
	`

type pythonResolvers struct {
	language *pythonLanguage
}

var _ core.LanguageResolvers = (*pythonResolvers)(nil)

func (r *pythonResolvers) ResolveImports(tree core.ParseTree) ([]*ast.ImportNode, error) {
	data, err := tree.Data()
	if err != nil {
		return nil, fmt.Errorf("failed to get data from parse tree: %w", err)
	}

	var imports []*ast.ImportNode

	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(pyWholeModuleImportQuery, func(m *sitter.QueryMatch) error {
			node := ast.NewImportNode(data)
			node.SetModuleNameNode(m.Captures[0].Node)

			if len(m.Captures) > 1 && m.Captures[1].Node.Type() == "wildcard_import" {
				node.SetIsWildcardImport(true)
			} else {
				node.SetModuleAliasNode(m.Captures[0].Node)
				if len(m.Captures) > 1 {
					node.SetModuleAliasNode(m.Captures[1].Node)
				}
			}
			imports = append(imports, node)
			return nil
		}),
		ts.NewQueryItem(pyItemImportQuery, func(m *sitter.QueryMatch) error {
			node := ast.NewImportNode(data)
			node.SetModuleNameNode(m.Captures[0].Node)
			node.SetModuleItemNode(m.Captures[1].Node)
			node.SetModuleAliasNode(m.Captures[2].Node)
			// print node type and contents of all captures
			// fmt.Println("Node", m.Captures[0].Node.Content(*data))
			// for _, capture := range m.Captures {
			// 	fmt.Printf("Capture: %s, %s\n", capture.Node.Type(), capture.Node.Content(*data))
			// }

			imports = append(imports, node)
			return nil
		}),
	}

	err = ts.ExecuteQueries(ts.NewQueriesRequest(r.language, queryRequestItems), data, tree)
	if err != nil {
		return nil, err
	}

	return imports, err
}

func (r *pythonResolvers) ResolveImportContents(importNode *ast.ImportNode) (core.ImportContents, error) {
	return core.ImportContents{
		ModuleName:  importNode.ModuleName(),
		ModuleItem:  importNode.ModuleItem(),
		ModuleAlias: importNode.ModuleAlias(),
	}, nil
}
