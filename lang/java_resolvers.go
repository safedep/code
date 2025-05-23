package lang

import (
	"fmt"

	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
	"github.com/safedep/code/pkg/ts"
	sitter "github.com/smacker/go-tree-sitter"
)

type javaResolvers struct {
	language *javaLanguage
}

var _ core.LanguageResolvers = (*javaResolvers)(nil)

const javaImportQuery = `
	(import_declaration
		(scoped_identifier) @module_name
		(asterisk)? @wildcard)
`

func (r *javaResolvers) ResolveImports(tree core.ParseTree) ([]*ast.ImportNode, error) {
	data, err := tree.Data()
	if err != nil {
		return nil, fmt.Errorf("failed to get data from parse tree: %w", err)
	}

	var imports []*ast.ImportNode

	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(javaImportQuery, func(m *sitter.QueryMatch) error {
			node := ast.NewImportNode(data)

			var moduleNameNode *sitter.Node = nil
			isWildcard := false

			for _, capture := range m.Captures {
				switch capture.Node.Type() {
				case "scoped_identifier":
					moduleNameNode = capture.Node
				case "asterisk":
					isWildcard = true
				}
			}

			node.SetModuleNameNode(moduleNameNode)
			if isWildcard {
				node.SetIsWildcardImport(true)
			} else {
				node.SetModuleAliasNode(r.resolveAliasNode(moduleNameNode))
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

func (r *javaResolvers) resolveAliasNode(moduleNameNode *sitter.Node) *sitter.Node {
	aliasNode := moduleNameNode.ChildByFieldName("name")
	if aliasNode == nil {
		return moduleNameNode
	}

	return aliasNode
}
