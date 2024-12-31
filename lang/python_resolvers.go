package lang

import (
	"fmt"

	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
	"github.com/safedep/code/pkg/ts"
	sitter "github.com/smacker/go-tree-sitter"
)

const pythonImportQuery = `
	(import_statement
		name: ((dotted_name) @module_name))

	(import_from_statement
		module_name: (dotted_name) @module_name
		name: (dotted_name
			(identifier) @submodule_name @submodule_alias))

	(import_from_statement
		module_name: (relative_import) @module_name
		name: (dotted_name
			(identifier) @submodule_name @submodule_alias))

	(import_statement
		name: (aliased_import
			name: ((dotted_name) @module_name @submodule_name)
			alias: (identifier) @submodule_alias))

	(import_from_statement
		module_name: (dotted_name) @module_name
		name: (aliased_import
			name: (dotted_name
				(identifier) @submodule_name)
			 alias: (identifier) @submodule_alias))

	(import_from_statement
		module_name: (relative_import) @module_name
		name: (aliased_import
			name: ((dotted_name) @submodule_name)
			alias: (identifier) @submodule_alias))
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

	qx := ts.NewQueryExecutor(r.language.Language(), data)
	matches, err := qx.Execute(tree.Tree().RootNode(), pythonImportQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	defer matches.Close()

	var imports []*ast.ImportNode
	err = matches.ForEach(func(m *sitter.QueryMatch) error {
		node := ast.NewImportNode(data)
		node.SetModuleNameNode(m.Captures[0].Node)

		if len(m.Captures) > 1 {
			node.SetModuleItemNode(m.Captures[1].Node)
		}

		if len(m.Captures) > 2 {
			node.SetModuleAliasNode(m.Captures[2].Node)
		}

		imports = append(imports, node)
		return nil
	})

	return imports, err
}
