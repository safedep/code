package lang

import (
	"context"
	"fmt"
	"strings"

	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
	"github.com/safedep/code/pkg/helpers"
	"github.com/safedep/code/pkg/ts"
	sitter "github.com/smacker/go-tree-sitter"
)

type goResolvers struct{}

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

func (r *goResolvers) ResolveImports(ctx context.Context, tree core.ParseTree) ([]*ast.ImportNode, error) {
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
			importPath := strings.Trim(node.ModuleName(), `"`)
			node.SetModuleName(importPath)
			if explicitModuleAliasNode != nil {
				node.SetModuleAliasNode(explicitModuleAliasNode)
			} else if !node.IsWildcardImport() {
				// The path brings the package name into scope.
				node.SetModuleAliasNode(moduleNameNode)
				node.SetModuleAlias(helpers.GoPackageName(importPath))
			}

			imports = append(imports, node)
			return nil
		}),
	}

	err = ts.ExecuteQueries(ctx, tree, queryRequestItems)
	if err != nil {
		return nil, err
	}

	return imports, err
}

const goFunctionDefinitionQuery = `
	(function_declaration
		name: (identifier) @function_name
		parameters: (parameter_list) @function_params
		body: (block) @function_body)
`

const goMethodDefinitionQuery = `
	(method_declaration
		receiver: (parameter_list) @receiver
		name: (field_identifier) @method_name
		parameters: (parameter_list) @method_params
		body: (block) @method_body)
`

func (r *goResolvers) ResolveFunctions(ctx context.Context, tree core.ParseTree) ([]*ast.FunctionDeclarationNode, error) {
	data, err := tree.Data()
	if err != nil {
		return nil, fmt.Errorf("failed to get data from parse tree: %w", err)
	}

	var functions []*ast.FunctionDeclarationNode
	functionMap := make(map[string]*ast.FunctionDeclarationNode)

	err = r.extractGoFunctions(ctx, data, tree, functionMap)
	if err != nil {
		return nil, fmt.Errorf("failed to extract Go functions: %w", err)
	}

	err = r.extractGoMethods(ctx, data, tree, functionMap)
	if err != nil {
		return nil, fmt.Errorf("failed to extract Go methods: %w", err)
	}

	for _, function := range functionMap {
		functions = append(functions, function)
	}

	return functions, nil
}

func (r *goResolvers) extractGoFunctions(ctx context.Context, data *[]byte, tree core.ParseTree,
	functionMap map[string]*ast.FunctionDeclarationNode,
) error {
	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(goFunctionDefinitionQuery, func(m *sitter.QueryMatch) error {
			if len(m.Captures) < 3 {
				return nil
			}

			var functionNameNode, paramsNode, bodyNode *sitter.Node

			for _, capture := range m.Captures {
				switch capture.Node.Type() {
				case "identifier":
					functionNameNode = capture.Node
				case "parameter_list":
					paramsNode = capture.Node
				case "block":
					bodyNode = capture.Node
				}
			}

			if functionNameNode == nil {
				return nil
			}

			functionName := functionNameNode.Content(*data)
			if !r.isValidGoIdentifier(functionName) {
				return nil
			}

			functionKey := r.generateGoFunctionKey(functionNameNode, "", *data)
			functionNode := ast.NewFunctionDeclarationNode(data)
			functionNode.SetFunctionNameNode(functionNameNode)
			functionNode.SetFunctionType(ast.FunctionTypeFunction)

			if paramsNode != nil {
				paramNodes := r.extractGoParameterNodes(paramsNode)
				functionNode.SetFunctionParameterNodes(paramNodes)
			}

			if bodyNode != nil {
				functionNode.SetFunctionBodyNode(bodyNode)
			}

			if r.isExportedGoIdentifier(functionName) {
				functionNode.SetAccessModifier(ast.AccessModifierPublic)
			} else {
				functionNode.SetAccessModifier(ast.AccessModifierPackage)
			}

			functionMap[functionKey] = functionNode
			return nil
		}),
	}

	return ts.ExecuteQueries(ctx, tree, queryRequestItems)
}

func (r *goResolvers) extractGoMethods(ctx context.Context, data *[]byte, tree core.ParseTree,
	functionMap map[string]*ast.FunctionDeclarationNode,
) error {
	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(goMethodDefinitionQuery, func(m *sitter.QueryMatch) error {
			if len(m.Captures) < 4 {
				return nil
			}

			var receiverNode, methodNameNode, paramsNode, bodyNode *sitter.Node

			for _, capture := range m.Captures {
				switch capture.Node.Type() {
				case "parameter_list":
					if receiverNode == nil {
						receiverNode = capture.Node
					} else {
						paramsNode = capture.Node
					}
				case "field_identifier":
					methodNameNode = capture.Node
				case "block":
					bodyNode = capture.Node
				}
			}

			if methodNameNode == nil || receiverNode == nil {
				return nil
			}

			methodName := methodNameNode.Content(*data)
			if !r.isValidGoIdentifier(methodName) {
				return nil
			}

			receiverTypeName := r.extractReceiverTypeName(receiverNode, *data)
			if receiverTypeName == "" {
				return nil
			}

			if !r.isValidGoIdentifier(receiverTypeName) {
				return nil
			}

			functionKey := r.generateGoFunctionKey(methodNameNode, receiverTypeName, *data)
			functionNode := ast.NewFunctionDeclarationNode(data)
			functionNode.SetFunctionNameNode(methodNameNode)
			functionNode.SetFunctionType(ast.FunctionTypeMethod)
			functionNode.SetParentClassName(receiverTypeName)

			if paramsNode != nil {
				paramNodes := r.extractGoParameterNodes(paramsNode)
				functionNode.SetFunctionParameterNodes(paramNodes)
			}

			if bodyNode != nil {
				functionNode.SetFunctionBodyNode(bodyNode)
			}

			if r.isExportedGoIdentifier(methodName) {
				functionNode.SetAccessModifier(ast.AccessModifierPublic)
			} else {
				functionNode.SetAccessModifier(ast.AccessModifierPackage)
			}

			functionMap[functionKey] = functionNode
			return nil
		}),
	}

	return ts.ExecuteQueries(ctx, tree, queryRequestItems)
}

func (r *goResolvers) extractGoParameterNodes(parametersNode *sitter.Node) []*sitter.Node {
	var paramNodes []*sitter.Node

	if parametersNode == nil {
		return paramNodes
	}

	for i := 0; i < int(parametersNode.ChildCount()); i++ {
		child := parametersNode.Child(i)
		if child.Type() == "parameter_declaration" || child.Type() == "variadic_parameter_declaration" {
			paramNodes = append(paramNodes, child)
		}
	}

	return paramNodes
}

func (r *goResolvers) extractReceiverTypeName(receiverNode *sitter.Node, data []byte) string {
	if receiverNode == nil {
		return ""
	}

	for i := 0; i < int(receiverNode.ChildCount()); i++ {
		child := receiverNode.Child(i)
		if child.Type() == "parameter_declaration" {
			for j := 0; j < int(child.ChildCount()); j++ {
				grandchild := child.Child(j)
				if grandchild.Type() == "type_identifier" ||
					grandchild.Type() == "pointer_type" {
					return r.extractTypeNameFromNode(grandchild, data)
				}
			}
		}
	}

	return ""
}

func (r *goResolvers) extractTypeNameFromNode(typeNode *sitter.Node, data []byte) string {
	if typeNode == nil {
		return ""
	}

	if typeNode.Type() == "type_identifier" {
		return typeNode.Content(data)
	} else if typeNode.Type() == "pointer_type" {
		for i := 0; i < int(typeNode.ChildCount()); i++ {
			child := typeNode.Child(i)
			if child.Type() == "type_identifier" {
				return child.Content(data)
			}
		}
	}

	return ""
}

func (r *goResolvers) generateGoFunctionKey(functionNameNode *sitter.Node, receiverType string, data []byte) string {
	functionName := functionNameNode.Content(data)

	if receiverType != "" {
		return receiverType + "." + functionName
	}

	lineNumber := functionNameNode.StartPoint().Row
	return fmt.Sprintf("%s:%d", functionName, lineNumber)
}

// isExportedGoIdentifier reports whether a Go identifier starts with an
// upper case letter.
func (r *goResolvers) isExportedGoIdentifier(identifier string) bool {
	if len(identifier) == 0 {
		return false
	}

	firstChar := identifier[0]
	return firstChar >= 'A' && firstChar <= 'Z'
}

// isValidGoIdentifier reports whether the string is a Go identifier of
// ASCII letters, digits and underscores.
func (r *goResolvers) isValidGoIdentifier(identifier string) bool {
	if len(identifier) == 0 {
		return false
	}

	firstChar := identifier[0]
	if (firstChar < 'a' || firstChar > 'z') && (firstChar < 'A' || firstChar > 'Z') && firstChar != '_' {
		return false
	}

	for i := 1; i < len(identifier); i++ {
		char := identifier[i]
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '_' {
			return false
		}
	}

	return true
}
