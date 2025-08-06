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
var _ core.ObjectOrientedLanguageResolvers = (*pythonResolvers)(nil)

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

// Tree-Sitter queries for Python class definitions
const pyClassDefinitionQuery = `
	(class_definition
		name: (identifier) @class_name
		superclasses: (argument_list) @superclasses
		body: (block) @class_body)

	(class_definition
		name: (identifier) @class_name
		body: (block) @class_body)
`

const pyClassMethodQuery = `
	(class_definition
		body: (block
			(function_definition
				name: (identifier) @method_name) @method_def))
`

const pyClassFieldQuery = `
	(class_definition
		body: (block
			(expression_statement
				(assignment
					left: (attribute
						object: (identifier) @instance_ref
						attribute: (identifier) @field_name)) @field_assignment)))
`

const pyClassDecoratorQuery = `
	(decorated_definition
		(decorator
			(identifier) @decorator_name) @decorator
		definition: (class_definition
			name: (identifier) @class_name))
`

const pyInheritanceQuery = `
	(class_definition
		name: (identifier) @class_name
		superclasses: (argument_list
			(identifier) @base_class))

	(class_definition
		name: (identifier) @class_name
		superclasses: (argument_list
			(attribute
				object: (identifier) @module_name
				attribute: (identifier) @base_class)))
`

// ResolveClasses extracts class declarations from the parse tree
func (r *pythonResolvers) ResolveClasses(tree core.ParseTree) ([]*ast.ClassDeclarationNode, error) {
	data, err := tree.Data()
	if err != nil {
		return nil, fmt.Errorf("failed to get data from parse tree: %w", err)
	}

	var classes []*ast.ClassDeclarationNode
	classMap := make(map[string]*ast.ClassDeclarationNode) // To avoid duplicates and allow enhancement

	// Extract basic class definitions
	err = r.extractClassDefinitions(data, tree, classMap)
	if err != nil {
		return nil, fmt.Errorf("failed to extract class definitions: %w", err)
	}

	// Enhance classes with methods
	err = r.extractClassMethods(data, tree, classMap)
	if err != nil {
		return nil, fmt.Errorf("failed to extract class methods: %w", err)
	}

	// Enhance classes with fields
	err = r.extractClassFields(data, tree, classMap)
	if err != nil {
		return nil, fmt.Errorf("failed to extract class fields: %w", err)
	}

	// Enhance classes with decorators
	err = r.extractClassDecorators(data, tree, classMap)
	if err != nil {
		return nil, fmt.Errorf("failed to extract class decorators: %w", err)
	}

	// Convert map to slice
	for _, class := range classMap {
		classes = append(classes, class)
	}

	return classes, nil
}

// ResolveInheritance constructs the inheritance graph from the parse tree
func (r *pythonResolvers) ResolveInheritance(tree core.ParseTree) (*ast.InheritanceGraph, error) {
	data, err := tree.Data()
	if err != nil {
		return nil, fmt.Errorf("failed to get data from parse tree: %w", err)
	}

	file, err := tree.File()
	if err != nil {
		return nil, fmt.Errorf("failed to get file from parse tree: %w", err)
	}

	ig := ast.NewInheritanceGraph()

	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(pyInheritanceQuery, func(m *sitter.QueryMatch) error {
			if len(m.Captures) < 2 {
				return nil // Skip incomplete matches
			}

			childClass := m.Captures[0].Node.Content(*data)
			baseClass := ""

			// Handle different base class patterns
			if len(m.Captures) == 2 {
				// Simple inheritance: class Child(Parent)
				baseClass = m.Captures[1].Node.Content(*data)
			} else if len(m.Captures) == 3 {
				// Module-qualified inheritance: class Child(module.Parent)
				moduleName := m.Captures[1].Node.Content(*data)
				className := m.Captures[2].Node.Content(*data)
				baseClass = moduleName + "." + className
			}

			if childClass != "" && baseClass != "" {
				lineNumber := m.Captures[0].Node.StartPoint().Row + 1
				ig.AddRelationship(childClass, baseClass, ast.RelationshipTypeInherits, file.Name(), uint32(lineNumber))
			}

			return nil
		}),
	}

	err = ts.ExecuteQueries(ts.NewQueriesRequest(r.language, queryRequestItems), data, tree)
	if err != nil {
		return nil, fmt.Errorf("failed to execute inheritance queries: %w", err)
	}

	return ig, nil
}

// Helper methods for class extraction

func (r *pythonResolvers) extractClassDefinitions(data *[]byte, tree core.ParseTree,
	classMap map[string]*ast.ClassDeclarationNode) error {
	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(pyClassDefinitionQuery, func(m *sitter.QueryMatch) error {
			if len(m.Captures) < 2 {
				return nil // Skip incomplete matches
			}

			classNameNode := m.Captures[0].Node
			className := classNameNode.Content(*data)

			// Create or get existing class
			var classNode *ast.ClassDeclarationNode
			if existing, exists := classMap[className]; exists {
				classNode = existing
			} else {
				classNode = ast.NewClassDeclarationNode(data)
				classNode.SetClassNameNode(classNameNode)
				classMap[className] = classNode
			}

			// Handle superclasses if present
			if len(m.Captures) >= 3 && m.Captures[1].Node.Type() == "argument_list" {
				superclassesNode := m.Captures[1].Node
				baseClassNodes := r.extractBaseClassNodes(superclassesNode)
				classNode.SetBaseClassNodes(baseClassNodes)
			}

			// Class body will be processed by other extraction methods

			return nil
		}),
	}

	return ts.ExecuteQueries(ts.NewQueriesRequest(r.language, queryRequestItems), data, tree)
}

func (r *pythonResolvers) extractClassMethods(data *[]byte, tree core.ParseTree,
	classMap map[string]*ast.ClassDeclarationNode) error {
	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(pyClassMethodQuery, func(m *sitter.QueryMatch) error {
			if len(m.Captures) < 2 {
				return nil
			}

			var methodNameNode, methodDefNode *sitter.Node

			// Find the correct nodes by type
			for _, capture := range m.Captures {
				if capture.Node.Type() == "identifier" {
					methodNameNode = capture.Node
				} else if capture.Node.Type() == "function_definition" {
					methodDefNode = capture.Node
				}
			}

			if methodNameNode == nil || methodDefNode == nil {
				return nil
			}

			// Find the class this method belongs to by traversing up the tree
			className := r.findParentClassName(methodDefNode, *data)
			if className == "" {
				return nil
			}

			if classNode, exists := classMap[className]; exists {
				classNode.AddMethodNode(methodDefNode)

				// Check if this is a constructor
				methodName := methodNameNode.Content(*data)
				if methodName == "__init__" {
					classNode.SetConstructorNode(methodDefNode)
				}
			}

			return nil
		}),
	}

	return ts.ExecuteQueries(ts.NewQueriesRequest(r.language, queryRequestItems), data, tree)
}

func (r *pythonResolvers) extractClassFields(data *[]byte, tree core.ParseTree,
	classMap map[string]*ast.ClassDeclarationNode) error {
	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(pyClassFieldQuery, func(m *sitter.QueryMatch) error {
			if len(m.Captures) < 3 {
				return nil
			}

			instanceRef := m.Captures[0].Node.Content(*data)
			fieldAssignmentNode := m.Captures[2].Node

			// Only process self.field assignments
			if instanceRef == "self" {
				className := r.findParentClassName(fieldAssignmentNode, *data)
				if className == "" {
					return nil
				}

				if classNode, exists := classMap[className]; exists {
					classNode.AddFieldNode(fieldAssignmentNode)
				}
			}

			return nil
		}),
	}

	return ts.ExecuteQueries(ts.NewQueriesRequest(r.language, queryRequestItems), data, tree)
}

func (r *pythonResolvers) extractClassDecorators(data *[]byte, tree core.ParseTree,
	classMap map[string]*ast.ClassDeclarationNode) error {
	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(pyClassDecoratorQuery, func(m *sitter.QueryMatch) error {
			if len(m.Captures) < 3 {
				return nil
			}

			decoratorNode := m.Captures[0].Node
			decoratorIdentifier := m.Captures[1].Node
			className := m.Captures[2].Node.Content(*data)

			if classNode, exists := classMap[className]; exists {
				classNode.AddDecoratorNode(decoratorNode)

				// Check for abstract decorator - use the identifier node
				decoratorName := decoratorIdentifier.Content(*data)
				if decoratorName == "abstractmethod" || decoratorName == "ABC" {
					classNode.SetIsAbstract(true)
				}
			}

			return nil
		}),
	}

	return ts.ExecuteQueries(ts.NewQueriesRequest(r.language, queryRequestItems), data, tree)
}

// Helper methods

func (r *pythonResolvers) extractBaseClassNodes(superclassesNode *sitter.Node) []*sitter.Node {
	var baseClassNodes []*sitter.Node

	if superclassesNode == nil {
		return baseClassNodes
	}

	for i := 0; i < int(superclassesNode.ChildCount()); i++ {
		child := superclassesNode.Child(i)
		if child.Type() == "identifier" || child.Type() == "attribute" {
			baseClassNodes = append(baseClassNodes, child)
		}
	}

	return baseClassNodes
}

// Tree-Sitter queries for Python function definitions
const pyFunctionDefinitionQuery = `
	(function_definition
		name: (identifier) @function_name
		parameters: (parameters) @function_params
		body: (block) @function_body
		return_type: (type)? @return_type)

	(function_definition
		name: (identifier) @function_name
		parameters: (parameters) @function_params
		body: (block) @function_body)
`

const pyAsyncFunctionQuery = `
	(function_definition
		(async) @async_keyword
		name: (identifier) @function_name
		parameters: (parameters) @function_params
		body: (block) @function_body)
`

const pyFunctionDecoratorQuery = `
	(decorated_definition
		(decorator
			(identifier) @decorator_name) @decorator
		definition: (function_definition
			name: (identifier) @function_name))
`

// ResolveFunctions extracts function declarations from the parse tree
func (r *pythonResolvers) ResolveFunctions(tree core.ParseTree) ([]*ast.FunctionDeclarationNode, error) {
	data, err := tree.Data()
	if err != nil {
		return nil, fmt.Errorf("failed to get data from parse tree: %w", err)
	}

	var functions []*ast.FunctionDeclarationNode
	functionMap := make(map[string]*ast.FunctionDeclarationNode) // To avoid duplicates and allow enhancement

	// Extract basic function definitions
	err = r.extractFunctionDefinitions(data, tree, functionMap)
	if err != nil {
		return nil, fmt.Errorf("failed to extract function definitions: %w", err)
	}

	// Async functions are now handled in the main function extraction

	// Extract function decorators
	err = r.extractFunctionDecorators(data, tree, functionMap)
	if err != nil {
		return nil, fmt.Errorf("failed to extract function decorators: %w", err)
	}

	// Convert map to slice and ensure all functions have proper access modifiers
	for _, function := range functionMap {
		// Ensure every function has a proper access modifier (not Unknown)
		currentModifier := function.GetAccessModifier()
		if currentModifier == ast.AccessModifierUnknown || currentModifier == "" {
			function.SetAccessModifier(ast.AccessModifierPublic)
		}
		functions = append(functions, function)
	}

	return functions, nil
}

// Helper methods for function extraction

func (r *pythonResolvers) extractFunctionDefinitions(data *[]byte, tree core.ParseTree,
	functionMap map[string]*ast.FunctionDeclarationNode) error {
	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(pyFunctionDefinitionQuery, func(m *sitter.QueryMatch) error {
			if len(m.Captures) < 3 {
				return nil // Skip incomplete matches
			}

			functionNameNode := m.Captures[0].Node
			functionName := functionNameNode.Content(*data)

			// Create or get existing function
			var functionNode *ast.FunctionDeclarationNode
			functionKey := r.generateFunctionKey(functionNameNode, *data)
			if existing, exists := functionMap[functionKey]; exists {
				functionNode = existing
			} else {
				functionNode = ast.NewFunctionDeclarationNode(data)
				functionNode.SetFunctionNameNode(functionNameNode)
				functionMap[functionKey] = functionNode
			}

			// Set function parameters
			if len(m.Captures) >= 2 && m.Captures[1].Node.Type() == "parameters" {
				paramsNode := m.Captures[1].Node
				paramNodes := r.extractParameterNodes(paramsNode)
				functionNode.SetFunctionParameterNodes(paramNodes)
			}

			// Set function body
			if len(m.Captures) >= 3 && m.Captures[2].Node.Type() == "block" {
				functionNode.SetFunctionBodyNode(m.Captures[2].Node)
			}

			// Set return type if present
			if len(m.Captures) >= 4 && m.Captures[3].Node.Type() == "type" {
				functionNode.SetFunctionReturnTypeNode(m.Captures[3].Node)
			}

			// Check for async function by looking at the function_definition parent for async keyword
			isAsync := false
			current := functionNameNode.Parent()
			for current != nil && current.Type() == "function_definition" {
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

			// Determine function type based on context
			parentClassName := r.findParentClassName(functionNameNode, *data)
			if parentClassName != "" {
				functionNode.SetParentClassName(parentClassName)
				if functionName == "__init__" {
					functionNode.SetFunctionType(ast.FunctionTypeConstructor)
				} else {
					functionNode.SetFunctionType(ast.FunctionTypeMethod)
				}
			} else {
				if isAsync {
					functionNode.SetFunctionType(ast.FunctionTypeAsync)
					functionNode.SetIsAsync(true)
				} else {
					functionNode.SetFunctionType(ast.FunctionTypeFunction)
				}
			}

			// Python functions are typically public by default
			functionNode.SetAccessModifier(ast.AccessModifierPublic)

			return nil
		}),
	}

	return ts.ExecuteQueries(ts.NewQueriesRequest(r.language, queryRequestItems), data, tree)
}

func (r *pythonResolvers) extractAsyncFunctions(data *[]byte, tree core.ParseTree,
	functionMap map[string]*ast.FunctionDeclarationNode) error {
	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(pyAsyncFunctionQuery, func(m *sitter.QueryMatch) error {
			if len(m.Captures) < 4 {
				return nil
			}

			var functionNameNode *sitter.Node
			for _, capture := range m.Captures {
				if capture.Node.Type() == "identifier" {
					functionNameNode = capture.Node
					break
				}
			}

			if functionNameNode == nil {
				return nil
			}

			// Get or create function node
			functionKey := r.generateFunctionKey(functionNameNode, *data)
			var functionNode *ast.FunctionDeclarationNode
			if existing, exists := functionMap[functionKey]; exists {
				functionNode = existing
			} else {
				functionNode = ast.NewFunctionDeclarationNode(data)
				functionNode.SetFunctionNameNode(functionNameNode)
				functionMap[functionKey] = functionNode
			}

			// Mark as async
			functionNode.SetIsAsync(true)
			functionNode.SetFunctionType(ast.FunctionTypeAsync)

			return nil
		}),
	}

	return ts.ExecuteQueries(ts.NewQueriesRequest(r.language, queryRequestItems), data, tree)
}

func (r *pythonResolvers) extractFunctionDecorators(data *[]byte, tree core.ParseTree,
	functionMap map[string]*ast.FunctionDeclarationNode) error {
	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(pyFunctionDecoratorQuery, func(m *sitter.QueryMatch) error {
			if len(m.Captures) < 3 {
				return nil
			}

			var decoratorNode, functionNameNode *sitter.Node
			for _, capture := range m.Captures {
				if capture.Node.Type() == "decorator" {
					decoratorNode = capture.Node
				} else if capture.Node.Type() == "identifier" {
					functionNameNode = capture.Node
				}
			}

			if decoratorNode == nil || functionNameNode == nil {
				return nil
			}

			functionKey := r.generateFunctionKey(functionNameNode, *data)
			if functionNode, exists := functionMap[functionKey]; exists {
				functionNode.AddDecoratorNode(decoratorNode)

				// Check for special decorators
				decoratorName := ""
				for _, capture := range m.Captures {
					if capture.Node.Type() == "identifier" && capture.Node.Parent().Type() == "decorator" {
						decoratorName = capture.Node.Content(*data)
						break
					}
				}

				if decoratorName == "staticmethod" {
					functionNode.SetIsStatic(true)
					functionNode.SetFunctionType(ast.FunctionTypeStaticMethod)
				} else if decoratorName == "abstractmethod" {
					functionNode.SetIsAbstract(true)
				}

				// Ensure access modifier is set to public for decorated functions
				functionNode.SetAccessModifier(ast.AccessModifierPublic)
			}

			return nil
		}),
	}

	return ts.ExecuteQueries(ts.NewQueriesRequest(r.language, queryRequestItems), data, tree)
}

// Helper methods

func (r *pythonResolvers) extractParameterNodes(parametersNode *sitter.Node) []*sitter.Node {
	var paramNodes []*sitter.Node

	if parametersNode == nil {
		return paramNodes
	}

	for i := 0; i < int(parametersNode.ChildCount()); i++ {
		child := parametersNode.Child(i)
		if child.Type() == "identifier" || child.Type() == "typed_parameter" || child.Type() == "default_parameter" {
			paramNodes = append(paramNodes, child)
		}
	}

	return paramNodes
}

func (r *pythonResolvers) generateFunctionKey(functionNameNode *sitter.Node, data []byte) string {
	functionName := functionNameNode.Content(data)
	parentClassName := r.findParentClassName(functionNameNode, data)

	if parentClassName != "" {
		return parentClassName + "." + functionName
	}

	// Add line number to distinguish functions with same name in different scopes
	lineNumber := functionNameNode.StartPoint().Row
	return fmt.Sprintf("%s:%d", functionName, lineNumber)
}

func (r *pythonResolvers) findParentClassName(node *sitter.Node, data []byte) string {
	if node == nil {
		return ""
	}

	current := node.Parent()
	for current != nil {
		if current.Type() == "class_definition" {
			nameNode := current.ChildByFieldName("name")
			if nameNode != nil {
				return nameNode.Content(data)
			}
		}

		current = current.Parent()
	}

	return ""
}
