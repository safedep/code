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
	err = r.extractClassDefinitions(data, tree, classes, classMap)
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

func (r *pythonResolvers) extractClassDefinitions(data *[]byte, tree core.ParseTree, classes []*ast.ClassDeclarationNode, classMap map[string]*ast.ClassDeclarationNode) error {
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

func (r *pythonResolvers) extractClassMethods(data *[]byte, tree core.ParseTree, classMap map[string]*ast.ClassDeclarationNode) error {
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

func (r *pythonResolvers) extractClassFields(data *[]byte, tree core.ParseTree, classMap map[string]*ast.ClassDeclarationNode) error {
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

func (r *pythonResolvers) extractClassDecorators(data *[]byte, tree core.ParseTree, classMap map[string]*ast.ClassDeclarationNode) error {
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

