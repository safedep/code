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

const jsWholeModuleImportQuery = `
	(import_statement
		(import_clause
			(identifier) @module_alias)
		source: (string (string_fragment) @module_name))

	(import_statement
		(import_clause
			(namespace_import (identifier) @module_alias))
		source: (string (string_fragment) @module_name))

	; const xyz = await import('xyz)
	(lexical_declaration
		(variable_declarator
			name: (identifier) @module_alias
			value: (await_expression
				(call_expression
					function: (import)
					arguments: (arguments (string (string_fragment) @module_name))))))
`

const jsRequireModuleQuery = `
	(lexical_declaration
	(variable_declarator
		name: (identifier) @module_alias
		value: (call_expression
			function: (identifier) @require_function
			arguments: (arguments (string (string_fragment) @module_name)))))

	(lexical_declaration
		(variable_declarator
			name: (object_pattern
				(pair_pattern
					key: (property_identifier) @module_item
					value: (identifier) @module_alias))
			value: (call_expression
				function: (identifier) @require_function
				arguments: (arguments (string (string_fragment) @module_name)))))

	(lexical_declaration
		(variable_declarator
			name: (object_pattern
				(shorthand_property_identifier_pattern) @module_item)
			value: (call_expression
				function: (identifier) @require_function
				arguments: (arguments (string (string_fragment) @module_name)))))
`

const jsSpecifiedItemImportQuery = `
	(import_statement
		(import_clause
			(named_imports
				(import_specifier
					name: (identifier) @module_item
					alias: (identifier)? @module_alias)))
		source: (string (string_fragment) @module_name))
`

func (r *javascriptResolvers) ResolveImports(tree core.ParseTree) ([]*ast.ImportNode, error) {
	data, err := tree.Data()
	if err != nil {
		return nil, fmt.Errorf("failed to get data from parse tree: %w", err)
	}

	var imports []*ast.ImportNode

	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(jsWholeModuleImportQuery, func(m *sitter.QueryMatch) error {
			node := ast.NewImportNode(data)
			node.SetModuleAliasNode(m.Captures[0].Node)
			node.SetModuleNameNode(m.Captures[1].Node)
			imports = append(imports, node)
			return nil
		}),
		ts.NewQueryItem(jsSpecifiedItemImportQuery, func(m *sitter.QueryMatch) error {
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
		ts.NewQueryItem(jsRequireModuleQuery, func(m *sitter.QueryMatch) error {
			if len(m.Captures) < 3 {
				return nil
			}

			node := ast.NewImportNode(data)

			identifierCaptures := []sitter.QueryCapture{}
			for _, capture := range m.Captures {
				switch capture.Node.Type() {
				case "string_fragment":
					node.SetModuleNameNode(capture.Node)
				case "identifier", "shorthand_property_identifier_pattern", "property_identifier":
					identifierCaptures = append(identifierCaptures, capture)
				}
			}

			if len(identifierCaptures) < 2 || identifierCaptures[len(identifierCaptures)-1].Node.Content(*data) != "require" {
				return nil
			}

			// Skip the last identifier ie. require
			for _, capture := range identifierCaptures[:len(identifierCaptures)-1] {
				switch capture.Node.Type() {
				case "identifier":
					node.SetModuleAliasNode(capture.Node)
				case "shorthand_property_identifier_pattern", "property_identifier":
					node.SetModuleItemNode(capture.Node)
					node.SetModuleAliasNode(capture.Node)
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

// Tree-Sitter queries for JavaScript function definitions based on actual grammar
const jsFunctionDefinitionQuery = `
	(function_declaration
		name: (identifier) @function_name
		parameters: (formal_parameters) @function_params
		body: (statement_block) @function_body)
`

const jsArrowFunctionQuery = `
	(variable_declarator
		name: (identifier) @function_name
		value: (arrow_function
			parameters: (_) @function_params
			body: (_) @function_body))

	(assignment_expression
		left: (identifier) @function_name
		right: (arrow_function
			parameters: (_) @function_params
			body: (_) @function_body))
`

const jsMethodDefinitionQuery = `
	(method_definition
		name: (property_identifier) @method_name
		parameters: (formal_parameters) @method_params
		body: (statement_block) @method_body)

	(method_definition
		(decorator)* @decorator
		name: (property_identifier) @method_name
		parameters: (formal_parameters) @method_params
		body: (statement_block) @method_body)
`

const jsAsyncFunctionQuery = `
	(function_declaration
		name: (identifier) @function_name
		parameters: (formal_parameters) @function_params
		body: (statement_block) @function_body)
`

const jsFunctionExpressionQuery = `
	(function_expression
		name: (identifier)? @function_name
		parameters: (formal_parameters) @function_params
		body: (statement_block) @function_body)
`

// ResolveFunctions extracts function declarations from JavaScript parse tree
func (r *javascriptResolvers) ResolveFunctions(tree core.ParseTree) ([]*ast.FunctionDeclarationNode, error) {
	data, err := tree.Data()
	if err != nil {
		return nil, fmt.Errorf("failed to get data from parse tree: %w", err)
	}

	var functions []*ast.FunctionDeclarationNode
	functionMap := make(map[string]*ast.FunctionDeclarationNode) // To avoid duplicates

	// Extract regular function declarations
	err = r.extractJSFunctions(data, tree, functionMap)
	if err != nil {
		return nil, fmt.Errorf("failed to extract JavaScript functions: %w", err)
	}

	// Extract arrow functions
	err = r.extractJSArrowFunctions(data, tree, functionMap)
	if err != nil {
		return nil, fmt.Errorf("failed to extract JavaScript arrow functions: %w", err)
	}

	// Extract method definitions (class methods)
	err = r.extractJSMethods(data, tree, functionMap)
	if err != nil {
		return nil, fmt.Errorf("failed to extract JavaScript methods: %w", err)
	}

	// Extract async functions using Tree-sitter queries
	err = r.extractJSAsyncFunctions(data, tree, functionMap)
	if err != nil {
		return nil, fmt.Errorf("failed to extract JavaScript async functions: %w", err)
	}

	// Extract function expressions
	err = r.extractJSFunctionExpressions(data, tree, functionMap)
	if err != nil {
		return nil, fmt.Errorf("failed to extract JavaScript function expressions: %w", err)
	}

	// Convert map to slice
	for _, function := range functionMap {
		functions = append(functions, function)
	}

	return functions, nil
}

// Helper methods for JavaScript function extraction

func (r *javascriptResolvers) extractJSFunctions(data *[]byte, tree core.ParseTree,
	functionMap map[string]*ast.FunctionDeclarationNode) error {
	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(jsFunctionDefinitionQuery, func(m *sitter.QueryMatch) error {
			if len(m.Captures) < 3 {
				return nil
			}

			var functionNameNode, paramsNode, bodyNode *sitter.Node

			for _, capture := range m.Captures {
				switch capture.Node.Type() {
				case "identifier":
					functionNameNode = capture.Node
				case "formal_parameters":
					paramsNode = capture.Node
				case "statement_block":
					bodyNode = capture.Node
				}
			}

			if functionNameNode == nil {
				return nil
			}

			// Check for async function by looking at the function_declaration parent for async keyword
			isAsync := false
			current := functionNameNode.Parent()
			for current != nil && current.Type() == "function_declaration" {
				// Check if any child node is "async"
				for i := 0; i < int(current.ChildCount()); i++ {
					child := current.Child(i)
					if child.Type() == "async" {
						isAsync = true
						break
					}
				}
				break
			}

			functionKey := r.generateJSFunctionKey(functionNameNode, "", *data)
			functionNode := ast.NewFunctionDeclarationNode(data)
			functionNode.SetFunctionNameNode(functionNameNode)

			if isAsync {
				functionNode.SetFunctionType(ast.FunctionTypeAsync)
				functionNode.SetIsAsync(true)
			} else {
				functionNode.SetFunctionType(ast.FunctionTypeFunction)
			}

			// Set parameters
			if paramsNode != nil {
				paramNodes := r.extractJSParameterNodes(paramsNode)
				functionNode.SetFunctionParameterNodes(paramNodes)
			}

			// Set function body
			if bodyNode != nil {
				functionNode.SetFunctionBodyNode(bodyNode)
			}

			// JavaScript functions are typically public
			functionNode.SetAccessModifier(ast.AccessModifierPublic)

			functionMap[functionKey] = functionNode
			return nil
		}),
	}

	return ts.ExecuteQueries(ts.NewQueriesRequest(r.language, queryRequestItems), data, tree)
}

func (r *javascriptResolvers) extractJSArrowFunctions(data *[]byte, tree core.ParseTree,
	functionMap map[string]*ast.FunctionDeclarationNode) error {
	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(jsArrowFunctionQuery, func(m *sitter.QueryMatch) error {
			if len(m.Captures) < 3 {
				return nil
			}

			var functionNameNode, paramsNode, bodyNode *sitter.Node

			for _, capture := range m.Captures {
				switch capture.Node.Type() {
				case "identifier":
					if functionNameNode == nil {
						functionNameNode = capture.Node
					} else if paramsNode == nil && capture.Node != functionNameNode {
						paramsNode = capture.Node
					}
				case "formal_parameters":
					if paramsNode == nil {
						paramsNode = capture.Node
					}
				default:
					if bodyNode == nil && capture.Node != functionNameNode && capture.Node != paramsNode {
						bodyNode = capture.Node
					}
				}
			}

			if functionNameNode == nil {
				return nil
			}

			// Check for async arrow function by looking at the arrow_function node for async keyword
			isAsync := false
			current := functionNameNode.Parent()
			for current != nil {
				if current.Type() == "arrow_function" {
					// Check if the arrow function has async keyword
					parent := current.Parent()
					if parent != nil {
						for i := 0; i < int(parent.ChildCount()); i++ {
							child := parent.Child(i)
							if child.Type() == "async" {
								isAsync = true
								break
							}
						}
					}
					break
				}
				current = current.Parent()
			}

			functionKey := r.generateJSFunctionKey(functionNameNode, "", *data)
			functionNode := ast.NewFunctionDeclarationNode(data)
			functionNode.SetFunctionNameNode(functionNameNode)

			if isAsync {
				functionNode.SetFunctionType(ast.FunctionTypeAsync)
				functionNode.SetIsAsync(true)
			} else {
				functionNode.SetFunctionType(ast.FunctionTypeArrow)
			}

			// Set parameters
			if paramsNode != nil {
				if paramsNode.Type() == "formal_parameters" {
					paramNodes := r.extractJSParameterNodes(paramsNode)
					functionNode.SetFunctionParameterNodes(paramNodes)
				} else {
					// Single parameter without parentheses
					functionNode.AddFunctionParameterNode(paramsNode)
				}
			}

			// Set function body
			if bodyNode != nil {
				functionNode.SetFunctionBodyNode(bodyNode)
			}

			functionNode.SetAccessModifier(ast.AccessModifierPublic)
			functionMap[functionKey] = functionNode
			return nil
		}),
	}

	return ts.ExecuteQueries(ts.NewQueriesRequest(r.language, queryRequestItems), data, tree)
}

func (r *javascriptResolvers) extractJSMethods(data *[]byte, tree core.ParseTree,
	functionMap map[string]*ast.FunctionDeclarationNode) error {
	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(jsMethodDefinitionQuery, func(m *sitter.QueryMatch) error {
			if len(m.Captures) < 3 {
				return nil
			}

			var methodNameNode, paramsNode, bodyNode *sitter.Node
			var decoratorNodes []*sitter.Node

			for _, capture := range m.Captures {
				switch capture.Node.Type() {
				case "property_identifier":
					methodNameNode = capture.Node
				case "formal_parameters":
					paramsNode = capture.Node
				case "statement_block":
					bodyNode = capture.Node
				case "decorator":
					decoratorNodes = append(decoratorNodes, capture.Node)
				}
			}

			if methodNameNode == nil {
				return nil
			}

			// Check for async method by looking at the method_definition parent for async keyword
			isAsync := false
			current := methodNameNode.Parent()
			for current != nil && current.Type() == "method_definition" {
				// Check if any child node is "async"
				for i := 0; i < int(current.ChildCount()); i++ {
					child := current.Child(i)
					if child.Type() == "async" {
						isAsync = true
						break
					}
				}
				break
			}

			// Find parent class name
			parentClassName := r.findJSParentClassName(methodNameNode, *data)

			functionKey := r.generateJSFunctionKey(methodNameNode, parentClassName, *data)
			functionNode := ast.NewFunctionDeclarationNode(data)
			functionNode.SetFunctionNameNode(methodNameNode)

			// Check for constructor first, then async, then default to method
			methodName := methodNameNode.Content(*data)
			if methodName == "constructor" {
				functionNode.SetFunctionType(ast.FunctionTypeConstructor)
			} else if isAsync {
				functionNode.SetFunctionType(ast.FunctionTypeAsync)
				functionNode.SetIsAsync(true)
			} else {
				functionNode.SetFunctionType(ast.FunctionTypeMethod)
			}

			if parentClassName != "" {
				functionNode.SetParentClassName(parentClassName)
			}

			// Set parameters
			if paramsNode != nil {
				paramNodes := r.extractJSParameterNodes(paramsNode)
				functionNode.SetFunctionParameterNodes(paramNodes)
			}

			// Set method body
			if bodyNode != nil {
				functionNode.SetFunctionBodyNode(bodyNode)
			}

			// Add decorators
			for _, decoratorNode := range decoratorNodes {
				functionNode.AddDecoratorNode(decoratorNode)
			}

			functionNode.SetAccessModifier(ast.AccessModifierPublic)
			functionMap[functionKey] = functionNode
			return nil
		}),
	}

	return ts.ExecuteQueries(ts.NewQueriesRequest(r.language, queryRequestItems), data, tree)
}

func (r *javascriptResolvers) extractJSAsyncFunctions(data *[]byte, tree core.ParseTree,
	functionMap map[string]*ast.FunctionDeclarationNode) error {
	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(jsAsyncFunctionQuery, func(m *sitter.QueryMatch) error {
			if len(m.Captures) < 3 {
				return nil
			}

			var functionNameNode, paramsNode, bodyNode *sitter.Node

			for _, capture := range m.Captures {
				switch capture.Node.Type() {
				case "identifier":
					functionNameNode = capture.Node
				case "formal_parameters":
					paramsNode = capture.Node
				case "statement_block":
					bodyNode = capture.Node
				}
			}

			if functionNameNode == nil {
				return nil
			}

			// Check if this is an async function by looking at the function_declaration parent for async keyword
			isAsync := false
			current := functionNameNode.Parent()
			for current != nil && current.Type() == "function_declaration" {
				// Check if any child node is "async"
				for i := 0; i < int(current.ChildCount()); i++ {
					child := current.Child(i)
					if child.Type() == "async" {
						isAsync = true
						break
					}
				}
				break
			}

			// Only process if it's actually an async function
			if !isAsync {
				return nil
			}

			functionKey := r.generateJSFunctionKey(functionNameNode, "", *data)
			if functionNode, exists := functionMap[functionKey]; exists {
				// Update existing function to be async
				functionNode.SetIsAsync(true)
				functionNode.SetFunctionType(ast.FunctionTypeAsync)
			} else {
				// Create new async function
				functionNode := ast.NewFunctionDeclarationNode(data)
				functionNode.SetFunctionNameNode(functionNameNode)
				functionNode.SetIsAsync(true)
				functionNode.SetFunctionType(ast.FunctionTypeAsync)
				functionNode.SetAccessModifier(ast.AccessModifierPublic)

				// Set parameters
				if paramsNode != nil {
					paramNodes := r.extractJSParameterNodes(paramsNode)
					functionNode.SetFunctionParameterNodes(paramNodes)
				}

				// Set function body
				if bodyNode != nil {
					functionNode.SetFunctionBodyNode(bodyNode)
				}

				functionMap[functionKey] = functionNode
			}

			return nil
		}),
	}

	return ts.ExecuteQueries(ts.NewQueriesRequest(r.language, queryRequestItems), data, tree)
}

func (r *javascriptResolvers) extractJSFunctionExpressions(data *[]byte, tree core.ParseTree,
	functionMap map[string]*ast.FunctionDeclarationNode) error {
	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(jsFunctionExpressionQuery, func(m *sitter.QueryMatch) error {
			if len(m.Captures) < 2 {
				return nil
			}

			var functionNameNode, paramsNode, bodyNode *sitter.Node

			for _, capture := range m.Captures {
				switch capture.Node.Type() {
				case "identifier":
					if functionNameNode == nil {
						functionNameNode = capture.Node
					}
				case "formal_parameters":
					paramsNode = capture.Node
				case "statement_block":
					bodyNode = capture.Node
				}
			}

			// Anonymous functions don't have names, skip them
			if functionNameNode == nil {
				return nil
			}

			// Check for async function expression by looking for async keyword
			isAsync := false
			current := functionNameNode.Parent()
			for current != nil && current.Type() == "function_expression" {
				parent := current.Parent()
				if parent != nil {
					for i := 0; i < int(parent.ChildCount()); i++ {
						child := parent.Child(i)
						if child.Type() == "async" {
							isAsync = true
							break
						}
					}
				}
				break
			}

			functionKey := r.generateJSFunctionKey(functionNameNode, "", *data)
			functionNode := ast.NewFunctionDeclarationNode(data)
			functionNode.SetFunctionNameNode(functionNameNode)

			if isAsync {
				functionNode.SetFunctionType(ast.FunctionTypeAsync)
				functionNode.SetIsAsync(true)
			} else {
				functionNode.SetFunctionType(ast.FunctionTypeFunction)
			}

			// Set parameters
			if paramsNode != nil {
				paramNodes := r.extractJSParameterNodes(paramsNode)
				functionNode.SetFunctionParameterNodes(paramNodes)
			}

			// Set function body
			if bodyNode != nil {
				functionNode.SetFunctionBodyNode(bodyNode)
			}

			functionNode.SetAccessModifier(ast.AccessModifierPublic)
			functionMap[functionKey] = functionNode
			return nil
		}),
	}

	return ts.ExecuteQueries(ts.NewQueriesRequest(r.language, queryRequestItems), data, tree)
}

// Helper methods for JavaScript function processing

func (r *javascriptResolvers) extractJSParameterNodes(parametersNode *sitter.Node) []*sitter.Node {
	var paramNodes []*sitter.Node

	if parametersNode == nil {
		return paramNodes
	}

	for i := 0; i < int(parametersNode.ChildCount()); i++ {
		child := parametersNode.Child(i)
		if child.Type() == "identifier" || child.Type() == "assignment_pattern" ||
			child.Type() == "rest_pattern" || child.Type() == "array_pattern" ||
			child.Type() == "object_pattern" {
			paramNodes = append(paramNodes, child)
		}
	}

	return paramNodes
}

func (r *javascriptResolvers) findJSParentClassName(node *sitter.Node, data []byte) string {
	if node == nil {
		return ""
	}

	current := node.Parent()
	for current != nil {
		if current.Type() == "class_declaration" {
			nameNode := current.ChildByFieldName("name")
			if nameNode != nil {
				return nameNode.Content(data)
			}
		}
		current = current.Parent()
	}

	return ""
}

func (r *javascriptResolvers) generateJSFunctionKey(functionNameNode *sitter.Node, parentClassName string, data []byte) string {
	functionName := functionNameNode.Content(data)

	if parentClassName != "" {
		return parentClassName + "." + functionName
	}

	// Add line number to distinguish functions with same name in different scopes
	lineNumber := functionNameNode.StartPoint().Row
	return fmt.Sprintf("%s:%d", functionName, lineNumber)
}
