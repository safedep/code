package lang

import (
	"fmt"
	"strings"

	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
	"github.com/safedep/code/pkg/ts"
	sitter "github.com/smacker/go-tree-sitter"
)

type goResolvers struct {
	language *goLanguage
}

var _ core.LanguageResolvers = (*goResolvers)(nil)

const goWholeModuleImportQuery = `
	(import_declaration 
		(import_spec 
			name: (package_identifier)? @module_alias
			name: (blank_identifier)? @blank_identifier
			name: (dot)? @dot_identifier
			path: (interpreted_string_literal) @module_name))

	(import_declaration 
		(import_spec_list 
			(import_spec 
				name: (package_identifier)? @module_alias
				name: (blank_identifier)? @blank_identifier
				name: (dot)? @dot_identifier
				path: (interpreted_string_literal) @module_name)))
`

func (r *goResolvers) ResolveImports(tree core.ParseTree) ([]*ast.ImportNode, error) {
	data, err := tree.Data()
	if err != nil {
		return nil, fmt.Errorf("failed to get data from parse tree: %w", err)
	}

	var imports []*ast.ImportNode

	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(goWholeModuleImportQuery, func(m *sitter.QueryMatch) error {
			node := ast.NewImportNode(data)

			var explicitModuleAliasNode *sitter.Node = nil
			var moduleNameNode *sitter.Node = nil

			// Find module name and alias from captures
			for _, capture := range m.Captures {
				switch capture.Node.Type() {
				case "interpreted_string_literal":
					moduleNameNode = capture.Node
				case "package_identifier":
					explicitModuleAliasNode = capture.Node
				case "blank_identifier", "dot":
					node.SetIsWildcardImport(true)
				}
			}
			node.SetModuleNameNode(moduleNameNode)
			if explicitModuleAliasNode != nil {
				node.SetModuleAliasNode(explicitModuleAliasNode)
			} else if !node.IsWildcardImport() {
				node.SetModuleAliasNode(moduleNameNode)
			}

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

func (r *goResolvers) ResolveImportContents(importNode *ast.ImportNode) (core.ImportContents, error) {
	moduleName := strings.Trim(importNode.ModuleName(), `"`)
	moduleItem := strings.Trim(importNode.ModuleItem(), `"`)
	moduleAlias := strings.Trim(importNode.ModuleAlias(), `"`)

	moduleAliasParts := strings.Split(moduleAlias, "/")
	moduleAlias = moduleAliasParts[len(moduleAliasParts)-1]

	return core.ImportContents{
		ModuleName:  moduleName,
		ModuleItem:  moduleItem,
		ModuleAlias: moduleAlias,
	}, nil
}
