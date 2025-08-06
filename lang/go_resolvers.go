package lang

import (
	"fmt"

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

// Tree-Sitter queries for Go function definitions
const goFunctionDefinitionQuery = `
	(function_declaration
		name: (identifier) @function_name
		parameters: (parameter_list) @function_params
		result: (_)? @return_type
		body: (block) @function_body)

	(function_declaration
		name: (identifier) @function_name
		parameters: (parameter_list) @function_params
		body: (block) @function_body)
`

const goMethodDefinitionQuery = `
	(method_declaration
		receiver: (parameter_list) @receiver
		name: (identifier) @method_name
		parameters: (parameter_list) @method_params
		result: (_)? @return_type
		body: (block) @method_body)

	(method_declaration
		receiver: (parameter_list) @receiver
		name: (identifier) @method_name
		parameters: (parameter_list) @method_params
		body: (block) @method_body)
`

// ResolveFunctions extracts function declarations from Go parse tree
func (r *goResolvers) ResolveFunctions(tree core.ParseTree) ([]*ast.FunctionDeclarationNode, error) {
	data, err := tree.Data()
	if err != nil {
		return nil, fmt.Errorf("failed to get data from parse tree: %w", err)
	}

	var functions []*ast.FunctionDeclarationNode
	functionMap := make(map[string]*ast.FunctionDeclarationNode) // To avoid duplicates

	// Extract regular function declarations
	err = r.extractGoFunctions(data, tree, functionMap)
	if err != nil {
		return nil, fmt.Errorf("failed to extract Go functions: %w", err)
	}

	// Extract method declarations (functions with receivers)
	err = r.extractGoMethods(data, tree, functionMap)
	if err != nil {
		return nil, fmt.Errorf("failed to extract Go methods: %w", err)
	}

	// Convert map to slice
	for _, function := range functionMap {
		functions = append(functions, function)
	}

	return functions, nil
}

// Helper methods for Go function extraction

func (r *goResolvers) extractGoFunctions(data *[]byte, tree core.ParseTree,
	functionMap map[string]*ast.FunctionDeclarationNode) error {
	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(goFunctionDefinitionQuery, func(m *sitter.QueryMatch) error {
			if len(m.Captures) < 3 {
				return nil // Skip incomplete matches
			}

			var functionNameNode, paramsNode, returnTypeNode, bodyNode *sitter.Node

			// Parse captures
			for _, capture := range m.Captures {
				switch capture.Node.Type() {
				case "identifier":
					functionNameNode = capture.Node
				case "parameter_list":
					paramsNode = capture.Node
				case "block":
					bodyNode = capture.Node
				default:
					// Return type can be various types
					if returnTypeNode == nil && capture.Node.Type() != "identifier" && capture.Node.Type() != "parameter_list" && capture.Node.Type() != "block" {
						returnTypeNode = capture.Node
					}
				}
			}

			if functionNameNode == nil {
				return nil
			}

			functionKey := r.generateGoFunctionKey(functionNameNode, "", *data)
			functionNode := ast.NewFunctionDeclarationNode(data)
			functionNode.SetFunctionNameNode(functionNameNode)
			functionNode.SetFunctionType(ast.FunctionTypeFunction)

			// Set parameters
			if paramsNode != nil {
				paramNodes := r.extractGoParameterNodes(paramsNode)
				functionNode.SetFunctionParameterNodes(paramNodes)
			}

			// Set return type
			if returnTypeNode != nil {
				functionNode.SetFunctionReturnTypeNode(returnTypeNode)
			}

			// Set function body
			if bodyNode != nil {
				functionNode.SetFunctionBodyNode(bodyNode)
			}

			// Go functions are typically public if they start with uppercase
			functionName := functionNameNode.Content(*data)
			if len(functionName) > 0 && functionName[0] >= 'A' && functionName[0] <= 'Z' {
				functionNode.SetAccessModifier(ast.AccessModifierPublic)
			} else {
				functionNode.SetAccessModifier(ast.AccessModifierPackage)
			}

			functionMap[functionKey] = functionNode
			return nil
		}),
	}

	return ts.ExecuteQueries(ts.NewQueriesRequest(r.language, queryRequestItems), data, tree)
}

func (r *goResolvers) extractGoMethods(data *[]byte, tree core.ParseTree,
	functionMap map[string]*ast.FunctionDeclarationNode) error {
	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(goMethodDefinitionQuery, func(m *sitter.QueryMatch) error {
			if len(m.Captures) < 4 {
				return nil
			}

			var receiverNode, methodNameNode, paramsNode, returnTypeNode, bodyNode *sitter.Node

			// Parse captures
			for _, capture := range m.Captures {
				switch capture.Node.Type() {
				case "parameter_list":
					if receiverNode == nil {
						receiverNode = capture.Node // First parameter_list is receiver
					} else {
						paramsNode = capture.Node // Second is method parameters
					}
				case "identifier":
					methodNameNode = capture.Node
				case "block":
					bodyNode = capture.Node
				default:
					// Return type
					if returnTypeNode == nil && capture.Node.Type() != "identifier" && 
					   capture.Node.Type() != "parameter_list" && capture.Node.Type() != "block" {
						returnTypeNode = capture.Node
					}
				}
			}

			if methodNameNode == nil || receiverNode == nil {
				return nil
			}

			// Extract receiver type name
			receiverTypeName := r.extractReceiverTypeName(receiverNode, *data)
			
			functionKey := r.generateGoFunctionKey(methodNameNode, receiverTypeName, *data)
			functionNode := ast.NewFunctionDeclarationNode(data)
			functionNode.SetFunctionNameNode(methodNameNode)
			functionNode.SetFunctionType(ast.FunctionTypeMethod)
			functionNode.SetParentClassName(receiverTypeName)

			// Set parameters
			if paramsNode != nil {
				paramNodes := r.extractGoParameterNodes(paramsNode)
				functionNode.SetFunctionParameterNodes(paramNodes)
			}

			// Set return type
			if returnTypeNode != nil {
				functionNode.SetFunctionReturnTypeNode(returnTypeNode)
			}

			// Set method body
			if bodyNode != nil {
				functionNode.SetFunctionBodyNode(bodyNode)
			}

			// Go methods are typically public if they start with uppercase
			methodName := methodNameNode.Content(*data)
			if len(methodName) > 0 && methodName[0] >= 'A' && methodName[0] <= 'Z' {
				functionNode.SetAccessModifier(ast.AccessModifierPublic)
			} else {
				functionNode.SetAccessModifier(ast.AccessModifierPackage)
			}

			functionMap[functionKey] = functionNode
			return nil
		}),
	}

	return ts.ExecuteQueries(ts.NewQueriesRequest(r.language, queryRequestItems), data, tree)
}

// Helper methods for Go function processing

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

	// Look for type identifier in the receiver parameter list
	for i := 0; i < int(receiverNode.ChildCount()); i++ {
		child := receiverNode.Child(i)
		if child.Type() == "parameter_declaration" {
			// Find the type part of the parameter declaration
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
		// For pointer types, get the underlying type
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
	
	// Add line number to distinguish functions with same name in different scopes
	lineNumber := functionNameNode.StartPoint().Row
	return fmt.Sprintf("%s:%d", functionName, lineNumber)
}
