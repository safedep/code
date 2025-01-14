package lang

import (
	"fmt"
	"slices"

	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
	"github.com/safedep/code/pkg/ts"
	sitter "github.com/smacker/go-tree-sitter"
)

type javascriptResolvers struct {
	language *javascriptLanguage
}

var _ core.LanguageResolvers = (*javascriptResolvers)(nil)

const wholeModuleImportQuery = `
	(import_statement
		(import_clause
			(identifier) @module_alias)
		source: (string (string_fragment) @module_name))

	(import_statement
		(import_clause
			(namespace_import (identifier) @module_alias))
		source: (string (string_fragment) @module_name))
	
	(lexical_declaration
		(variable_declarator
			name: (identifier) @module_alias
			value: (call_expression
				function: (identifier)
				arguments: (arguments (string (string_fragment) @module_name)))))
	
	; const xyz = await import('xyz)
	(lexical_declaration
		(variable_declarator
			name: (identifier) @module_alias
			value: (await_expression
				(call_expression
					function: (import)
					arguments: (arguments (string (string_fragment) @module_name))))))
`

const specifiedItemImportQuery = `
	(import_statement
		(import_clause
			(named_imports 
				(import_specifier 
					name: (identifier) @module_item 
					alias: (identifier)? @module_alias)))
		source: (string (string_fragment) @module_name))
	
	(lexical_declaration
		(variable_declarator
			name: (object_pattern
				(pair_pattern
					key: (property_identifier) @module_item
					value: (identifier) @module_alias))
			value: (call_expression
				function: (identifier)
				arguments: (arguments (string (string_fragment) @module_name)))))

	(lexical_declaration
		(variable_declarator
			name: (object_pattern
				(shorthand_property_identifier_pattern) @module_item)
			value: (call_expression
				function: (identifier)
				arguments: (arguments (string (string_fragment) @module_name)))))
`

func (r *javascriptResolvers) ResolveImports(tree core.ParseTree) ([]*ast.ImportNode, error) {
	data, err := tree.Data()
	if err != nil {
		return nil, fmt.Errorf("failed to get data from parse tree: %w", err)
	}

	var imports []*ast.ImportNode

	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(wholeModuleImportQuery, func(m *sitter.QueryMatch) error {
			node := ast.NewImportNode(data)
			node.SetModuleAliasNode(m.Captures[0].Node)
			node.SetModuleNameNode(m.Captures[1].Node)
			imports = append(imports, node)
			return nil
		}),
		ts.NewQueryItem(specifiedItemImportQuery, func(m *sitter.QueryMatch) error {
			node := ast.NewImportNode(data)
			alreadyEncounteredIdentifier := false
			for _, capture := range m.Captures {
				if capture.Node.Type() == "string_fragment" {
					node.SetModuleNameNode(capture.Node)
				} else if slices.Contains([]string{"identifier", "property_identifier", "shorthand_property_identifier_pattern"}, capture.Node.Type()) {
					if alreadyEncounteredIdentifier {
						node.SetModuleAliasNode(capture.Node)
					} else {
						node.SetModuleItemNode(capture.Node)
						node.SetModuleAliasNode(capture.Node)
						alreadyEncounteredIdentifier = true
					}
				}
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
