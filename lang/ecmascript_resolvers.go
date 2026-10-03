package lang

import (
	"fmt"
	"slices"

	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
	"github.com/safedep/code/pkg/ts"
	sitter "github.com/smacker/go-tree-sitter"
)

// ecmascript_resolvers.go — Shared resolver logic for JavaScript and TypeScript.
//
// Both languages share import syntax (ES modules, require), function declaration
// patterns (regular functions, arrow functions, methods, function expressions),
// and class structure. TypeScript extends these with type annotations, interfaces,
// enums, and access modifiers, which are handled in typescript_resolvers.go.

// --- Import Queries ---

const esWholeModuleImportQuery = `
	(import_statement
		(import_clause
			(identifier) @module_alias)
		source: (string (string_fragment) @module_name))

	(import_statement
		(import_clause
			(namespace_import (identifier) @module_alias))
		source: (string (string_fragment) @module_name))

	; const xyz = await import('xyz'), and the same with var or let
	(variable_declarator
		name: (identifier) @module_alias
		value: (await_expression
			(call_expression
				function: (import)
				arguments: (arguments (string (string_fragment) @module_name)))))
`

// The require queries match a variable_declarator, so a const, let or var
// declaration gives the same import.
const esRequireModuleQuery = `
	(variable_declarator
		name: (identifier) @module_alias
		value: (call_expression
			function: (identifier) @require_function
			arguments: (arguments (string (string_fragment) @module_name))))

	(variable_declarator
		name: (object_pattern
			(pair_pattern
				key: (property_identifier) @module_item
				value: (identifier) @module_alias))
		value: (call_expression
			function: (identifier) @require_function
			arguments: (arguments (string (string_fragment) @module_name))))

	(variable_declarator
		name: (object_pattern
			(shorthand_property_identifier_pattern) @module_item)
		value: (call_expression
			function: (identifier) @require_function
			arguments: (arguments (string (string_fragment) @module_name))))

	; x = require('x'), as bundled code writes it
	(assignment_expression
		left: (identifier) @module_alias
		right: (call_expression
			function: (identifier) @require_function
			arguments: (arguments (string (string_fragment) @module_name))))
`

// const EventEmitter = require('events').EventEmitter
const esRequireMemberQuery = `
	(variable_declarator
		name: (identifier) @module_alias
		value: (member_expression
			object: (call_expression
				function: (identifier) @require_function
				arguments: (arguments (string (string_fragment) @module_name)))
			property: (property_identifier) @module_item))
`

// const debug = require('debug')('app'): the alias holds what the module
// returns, so it stands for the module.
const esRequireCallQuery = `
	(variable_declarator
		name: (identifier) @module_alias
		value: (call_expression
			function: (call_expression
				function: (identifier) @require_function
				arguments: (arguments (string (string_fragment) @module_name)))))
`

// const { default: JSZip, generate } = await import('jszip')
const esDynamicImportDestructuringQuery = `
	(variable_declarator
		name: (object_pattern
			(pair_pattern
				key: (property_identifier) @module_item
				value: (identifier) @module_alias))
		value: (await_expression
			(call_expression
				function: (import)
				arguments: (arguments (string (string_fragment) @module_name)))))

	(variable_declarator
		name: (object_pattern
			(shorthand_property_identifier_pattern) @module_item)
		value: (await_expression
			(call_expression
				function: (import)
				arguments: (arguments (string (string_fragment) @module_name)))))
`

// import 'reflect-metadata' loads a module for its side effects.
const esSideEffectImportQuery = `
	(import_statement
		source: (string (string_fragment) @module_name))
`

// require('dotenv').config() uses a module with no binding.
const esUnboundRequireQuery = `
	(call_expression
		function: (identifier) @require_function
		arguments: (arguments . (string (string_fragment) @module_name)))
`

const esSpecifiedItemImportQuery = `
	(import_statement
		(import_clause
			(named_imports
				(import_specifier
					name: (identifier) @module_item
					alias: (identifier)? @module_alias)))
		source: (string (string_fragment) @module_name))
`

// resolveESImports resolves ES module imports and CommonJS require() calls.
// This is the shared implementation used by both JavaScript and TypeScript resolvers.
func resolveESImports(lang core.Language, tree core.ParseTree) ([]*ast.ImportNode, error) {
	data, err := tree.Data()
	if err != nil {
		return nil, fmt.Errorf("failed to get data from parse tree: %w", err)
	}

	var imports []*ast.ImportNode

	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(esWholeModuleImportQuery, func(m *sitter.QueryMatch) error {
			node := ast.NewImportNode(data)
			node.SetModuleAliasNode(m.Captures[0].Node)
			node.SetModuleNameNode(m.Captures[1].Node)
			imports = append(imports, node)
			return nil
		}),
		ts.NewQueryItem(esSpecifiedItemImportQuery, func(m *sitter.QueryMatch) error {
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
		ts.NewQueryItem(esRequireModuleQuery, func(m *sitter.QueryMatch) error {
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
		ts.NewQueryItem(esRequireMemberQuery, func(m *sitter.QueryMatch) error {
			if !capturedRequire(m, data) {
				return nil
			}
			node := ast.NewImportNode(data)
			for _, capture := range m.Captures {
				switch capture.Node.Type() {
				case "string_fragment":
					node.SetModuleNameNode(capture.Node)
				case "property_identifier":
					node.SetModuleItemNode(capture.Node)
				case "identifier":
					if capture.Node.Content(*data) != "require" {
						node.SetModuleAliasNode(capture.Node)
					}
				}
			}
			imports = append(imports, node)
			return nil
		}),
		ts.NewQueryItem(esRequireCallQuery, func(m *sitter.QueryMatch) error {
			if !capturedRequire(m, data) {
				return nil
			}
			node := ast.NewImportNode(data)
			for _, capture := range m.Captures {
				switch {
				case capture.Node.Type() == "string_fragment":
					node.SetModuleNameNode(capture.Node)
				case capture.Node.Content(*data) != "require":
					node.SetModuleAliasNode(capture.Node)
				}
			}
			imports = append(imports, node)
			return nil
		}),
		ts.NewQueryItem(esDynamicImportDestructuringQuery, func(m *sitter.QueryMatch) error {
			node := ast.NewImportNode(data)
			for _, capture := range m.Captures {
				switch capture.Node.Type() {
				case "string_fragment":
					node.SetModuleNameNode(capture.Node)
				case "property_identifier":
					// The default export is the module itself.
					if capture.Node.Content(*data) != "default" {
						node.SetModuleItemNode(capture.Node)
					}
				case "shorthand_property_identifier_pattern":
					node.SetModuleItemNode(capture.Node)
					node.SetModuleAliasNode(capture.Node)
				case "identifier":
					node.SetModuleAliasNode(capture.Node)
				}
			}
			imports = append(imports, node)
			return nil
		}),
		ts.NewQueryItem(esSideEffectImportQuery, func(m *sitter.QueryMatch) error {
			statement := m.Captures[0].Node.Parent().Parent()
			for i := 0; i < int(statement.NamedChildCount()); i++ {
				if statement.NamedChild(i).Type() == "import_clause" {
					return nil
				}
			}
			node := ast.NewImportNode(data)
			node.SetModuleNameNode(m.Captures[0].Node)
			node.SetIsWildcardImport(true)
			imports = append(imports, node)
			return nil
		}),
		ts.NewQueryItem(esUnboundRequireQuery, func(m *sitter.QueryMatch) error {
			if !capturedRequire(m, data) {
				return nil
			}
			node := ast.NewImportNode(data)
			for _, capture := range m.Captures {
				if capture.Node.Type() == "string_fragment" {
					node.SetModuleNameNode(capture.Node)
				} else if boundRequire(capture.Node.Parent()) {
					return nil
				}
			}
			node.SetIsWildcardImport(true)
			imports = append(imports, node)
			return nil
		}),
	}

	err = ts.ExecuteQueries(ts.NewQueriesRequest(lang, queryRequestItems), data, tree)
	if err != nil {
		return nil, err
	}

	return imports, err
}

// capturedRequire reports a match whose require_function capture is the
// identifier require.
func capturedRequire(m *sitter.QueryMatch, data *[]byte) bool {
	for _, capture := range m.Captures {
		if capture.Node.Type() == "identifier" && capture.Node.Content(*data) == "require" {
			return true
		}
	}
	return false
}

// boundRequire reports a require call whose result a variable declarator
// keeps, directly or through a member access, a call or an await. The
// require queries resolve those.
func boundRequire(call *sitter.Node) bool {
	node := call
	for node.Parent() != nil {
		parent := node.Parent()
		switch parent.Type() {
		case "variable_declarator":
			return true
		case "assignment_expression":
			return parent.ChildByFieldName("left").Type() == "identifier"
		case "member_expression", "await_expression":
			node = parent
		case "call_expression":
			if parent.ChildByFieldName("function") != node {
				return false
			}
			node = parent
		default:
			return false
		}
	}
	return false
}

// --- Function Queries ---

const esFunctionDefinitionQuery = `
	(function_declaration
		name: (identifier) @function_name
		parameters: (formal_parameters) @function_params
		body: (statement_block) @function_body)
`

const esArrowFunctionQuery = `
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

const esMethodDefinitionQuery = `
	(method_definition
		name: (property_identifier) @method_name
		parameters: (formal_parameters) @method_params
		body: (statement_block) @method_body)
`

const esDecoratedMethodDefinitionQuery = `
	(method_definition
		(decorator) @decorator
		name: (property_identifier) @method_name
		parameters: (formal_parameters) @method_params
		body: (statement_block) @method_body)
`

const esFunctionExpressionQuery = `
	(function_expression
		name: (identifier)? @function_name
		parameters: (formal_parameters) @function_params
		body: (statement_block) @function_body)
`

// resolveESFunctions resolves function declarations from ES-family parse trees.
// Handles regular functions, arrow functions, class methods, and function expressions.
func resolveESFunctions(lang core.Language, tree core.ParseTree) ([]*ast.FunctionDeclarationNode, error) {
	data, err := tree.Data()
	if err != nil {
		return nil, fmt.Errorf("failed to get data from parse tree: %w", err)
	}

	var functions []*ast.FunctionDeclarationNode
	functionMap := make(map[string]*ast.FunctionDeclarationNode)

	err = extractESFunctions(lang, data, tree, functionMap)
	if err != nil {
		return nil, fmt.Errorf("failed to extract functions: %w", err)
	}

	err = extractESArrowFunctions(lang, data, tree, functionMap)
	if err != nil {
		return nil, fmt.Errorf("failed to extract arrow functions: %w", err)
	}

	err = extractESMethods(lang, data, tree, functionMap)
	if err != nil {
		return nil, fmt.Errorf("failed to extract methods: %w", err)
	}

	err = extractESFunctionExpressions(lang, data, tree, functionMap)
	if err != nil {
		return nil, fmt.Errorf("failed to extract function expressions: %w", err)
	}

	for _, function := range functionMap {
		functions = append(functions, function)
	}

	return functions, nil
}

func extractESFunctions(lang core.Language, data *[]byte, tree core.ParseTree,
	functionMap map[string]*ast.FunctionDeclarationNode) error {
	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(esFunctionDefinitionQuery, func(m *sitter.QueryMatch) error {
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

			isAsync := false
			current := functionNameNode.Parent()
			if current != nil && current.Type() == "function_declaration" {
				for i := 0; i < int(current.ChildCount()); i++ {
					child := current.Child(i)
					if child.Type() == "async" {
						isAsync = true
						break
					}
				}
			}

			functionKey := generateESFunctionKey(functionNameNode, "", *data)
			functionNode := ast.NewFunctionDeclarationNode(data)
			functionNode.SetFunctionNameNode(functionNameNode)

			if isAsync {
				functionNode.SetFunctionType(ast.FunctionTypeAsync)
				functionNode.SetIsAsync(true)
			} else {
				functionNode.SetFunctionType(ast.FunctionTypeFunction)
			}

			if paramsNode != nil {
				paramNodes := extractESParameterNodes(paramsNode)
				functionNode.SetFunctionParameterNodes(paramNodes)
			}

			if bodyNode != nil {
				functionNode.SetFunctionBodyNode(bodyNode)
			}

			functionNode.SetAccessModifier(ast.AccessModifierPublic)
			functionMap[functionKey] = functionNode
			return nil
		}),
	}

	return ts.ExecuteQueries(ts.NewQueriesRequest(lang, queryRequestItems), data, tree)
}

func extractESArrowFunctions(lang core.Language, data *[]byte, tree core.ParseTree,
	functionMap map[string]*ast.FunctionDeclarationNode) error {
	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(esArrowFunctionQuery, func(m *sitter.QueryMatch) error {
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

			isAsync := false
			current := functionNameNode.Parent()
			for current != nil {
				if current.Type() == "arrow_function" {
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

			functionKey := generateESFunctionKey(functionNameNode, "", *data)
			functionNode := ast.NewFunctionDeclarationNode(data)
			functionNode.SetFunctionNameNode(functionNameNode)

			if isAsync {
				functionNode.SetFunctionType(ast.FunctionTypeAsync)
				functionNode.SetIsAsync(true)
			} else {
				functionNode.SetFunctionType(ast.FunctionTypeArrow)
			}

			if paramsNode != nil {
				if paramsNode.Type() == "formal_parameters" {
					paramNodes := extractESParameterNodes(paramsNode)
					functionNode.SetFunctionParameterNodes(paramNodes)
				} else {
					functionNode.AddFunctionParameterNode(paramsNode)
				}
			}

			if bodyNode != nil {
				functionNode.SetFunctionBodyNode(bodyNode)
			}

			functionNode.SetAccessModifier(ast.AccessModifierPublic)
			functionMap[functionKey] = functionNode
			return nil
		}),
	}

	return ts.ExecuteQueries(ts.NewQueriesRequest(lang, queryRequestItems), data, tree)
}

func extractESMethods(lang core.Language, data *[]byte, tree core.ParseTree,
	functionMap map[string]*ast.FunctionDeclarationNode) error {
	methodHandler := func(m *sitter.QueryMatch) error {
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

		isAsync := false
		current := methodNameNode.Parent()
		if current != nil && current.Type() == "method_definition" {
			for i := 0; i < int(current.ChildCount()); i++ {
				child := current.Child(i)
				if child.Type() == "async" {
					isAsync = true
					break
				}
			}
		}

		parentClassName := findESParentClassName(methodNameNode, *data)

		functionKey := generateESFunctionKey(methodNameNode, parentClassName, *data)
		functionNode := ast.NewFunctionDeclarationNode(data)
		functionNode.SetFunctionNameNode(methodNameNode)

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

		if paramsNode != nil {
			paramNodes := extractESParameterNodes(paramsNode)
			functionNode.SetFunctionParameterNodes(paramNodes)
		}

		if bodyNode != nil {
			functionNode.SetFunctionBodyNode(bodyNode)
		}

		for _, decoratorNode := range decoratorNodes {
			functionNode.AddDecoratorNode(decoratorNode)
		}

		// Extract access modifier from parent method_definition node (for TypeScript)
		accessModifier := ast.AccessModifierPublic
		if current != nil && current.Type() == "method_definition" {
			accessModifier = extractESMethodAccessModifier(current, *data)
		}
		functionNode.SetAccessModifier(accessModifier)

		functionMap[functionKey] = functionNode
		return nil
	}

	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(esMethodDefinitionQuery, methodHandler),
	}

	err := ts.ExecuteQueries(ts.NewQueriesRequest(lang, queryRequestItems), data, tree)
	if err != nil {
		return err
	}

	// Try decorator-aware query separately; it may not be supported by all grammars
	decoratorQueryItems := []ts.QueryItem{
		ts.NewQueryItem(esDecoratedMethodDefinitionQuery, methodHandler),
	}

	// Ignore errors from decorator query as some grammars don't support it
	_ = ts.ExecuteQueries(ts.NewQueriesRequest(lang, decoratorQueryItems), data, tree)

	return nil
}

func extractESFunctionExpressions(lang core.Language, data *[]byte, tree core.ParseTree,
	functionMap map[string]*ast.FunctionDeclarationNode) error {
	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(esFunctionExpressionQuery, func(m *sitter.QueryMatch) error {
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

			if functionNameNode == nil {
				return nil
			}

			isAsync := false
			current := functionNameNode.Parent()
			if current != nil && current.Type() == "function_expression" {
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
			}

			functionKey := generateESFunctionKey(functionNameNode, "", *data)
			functionNode := ast.NewFunctionDeclarationNode(data)
			functionNode.SetFunctionNameNode(functionNameNode)

			if isAsync {
				functionNode.SetFunctionType(ast.FunctionTypeAsync)
				functionNode.SetIsAsync(true)
			} else {
				functionNode.SetFunctionType(ast.FunctionTypeFunction)
			}

			if paramsNode != nil {
				paramNodes := extractESParameterNodes(paramsNode)
				functionNode.SetFunctionParameterNodes(paramNodes)
			}

			if bodyNode != nil {
				functionNode.SetFunctionBodyNode(bodyNode)
			}

			functionNode.SetAccessModifier(ast.AccessModifierPublic)
			functionMap[functionKey] = functionNode
			return nil
		}),
	}

	return ts.ExecuteQueries(ts.NewQueriesRequest(lang, queryRequestItems), data, tree)
}

// --- Shared Helper Functions ---

func extractESParameterNodes(parametersNode *sitter.Node) []*sitter.Node {
	var paramNodes []*sitter.Node

	if parametersNode == nil {
		return paramNodes
	}

	for i := 0; i < int(parametersNode.ChildCount()); i++ {
		child := parametersNode.Child(i)
		if child.Type() == "identifier" || child.Type() == "assignment_pattern" ||
			child.Type() == "rest_pattern" || child.Type() == "array_pattern" ||
			child.Type() == "object_pattern" ||
			// TypeScript-specific parameter types
			child.Type() == "required_parameter" || child.Type() == "optional_parameter" ||
			child.Type() == "rest_parameter" {
			paramNodes = append(paramNodes, child)
		}
	}

	return paramNodes
}

func findESParentClassName(node *sitter.Node, data []byte) string {
	if node == nil {
		return ""
	}

	current := node.Parent()
	for current != nil {
		if current.Type() == "class_declaration" || current.Type() == "abstract_class_declaration" {
			nameNode := current.ChildByFieldName("name")
			if nameNode != nil {
				return nameNode.Content(data)
			}
		}
		current = current.Parent()
	}

	return ""
}

func generateESFunctionKey(functionNameNode *sitter.Node, parentClassName string, data []byte) string {
	functionName := functionNameNode.Content(data)

	if parentClassName != "" {
		return parentClassName + "." + functionName
	}

	lineNumber := functionNameNode.StartPoint().Row
	return fmt.Sprintf("%s:%d", functionName, lineNumber)
}

// extractESMethodAccessModifier extracts access modifier from a method_definition node.
// In JavaScript all methods are public. In TypeScript, methods can have
// accessibility_modifier children (public, private, protected).
func extractESMethodAccessModifier(methodDefNode *sitter.Node, data []byte) ast.AccessModifier {
	if methodDefNode == nil {
		return ast.AccessModifierPublic
	}

	for i := 0; i < int(methodDefNode.ChildCount()); i++ {
		child := methodDefNode.Child(i)
		if child.Type() == "accessibility_modifier" {
			switch child.Content(data) {
			case "public":
				return ast.AccessModifierPublic
			case "private":
				return ast.AccessModifierPrivate
			case "protected":
				return ast.AccessModifierProtected
			}
		}
	}

	return ast.AccessModifierPublic
}
