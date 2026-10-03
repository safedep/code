package lang

import (
	"fmt"

	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
	"github.com/safedep/code/pkg/ts"
	sitter "github.com/smacker/go-tree-sitter"
)

type typescriptResolvers struct {
	language *typescriptLanguage
}

var _ core.LanguageResolvers = (*typescriptResolvers)(nil)
var _ core.ObjectOrientedLanguageResolvers = (*typescriptResolvers)(nil)

// import fs = require('fs')
const tsImportRequireQuery = `
	(import_statement
		(import_require_clause
			(identifier) @module_alias
			source: (string (string_fragment) @module_name)))
`

func (r *typescriptResolvers) ResolveImports(tree core.ParseTree) ([]*ast.ImportNode, error) {
	// The TypeScript tree-sitter grammar treats `import type` as regular
	// import_statement nodes, so the shared ES queries already match them.
	// No additional type-only import resolution is needed.
	imports, err := resolveESImports(r.language, tree)
	if err != nil {
		return nil, err
	}

	data, err := tree.Data()
	if err != nil {
		return nil, fmt.Errorf("failed to get data from parse tree: %w", err)
	}
	err = ts.ExecuteQueries(ts.NewQueriesRequest(r.language, []ts.QueryItem{
		ts.NewQueryItem(tsImportRequireQuery, func(m *sitter.QueryMatch) error {
			node := ast.NewImportNode(data)
			node.SetModuleAliasNode(m.Captures[0].Node)
			node.SetModuleNameNode(m.Captures[1].Node)
			imports = append(imports, node)
			return nil
		}),
	}), data, tree)
	return imports, err
}

func (r *typescriptResolvers) ResolveFunctions(tree core.ParseTree) ([]*ast.FunctionDeclarationNode, error) {
	// Resolve standard ES functions (shared with JS)
	functions, err := resolveESFunctions(r.language, tree)
	if err != nil {
		return nil, err
	}

	// Enhance with TypeScript-specific abstract methods
	abstractMethods, err := r.resolveAbstractMethods(tree)
	if err != nil {
		return nil, err
	}

	return append(functions, abstractMethods...), nil
}

// --- TypeScript Abstract Method Query ---

const tsAbstractMethodQuery = `
	(abstract_method_signature
		name: (property_identifier) @method_name
		parameters: (formal_parameters) @method_params)
`

func (r *typescriptResolvers) resolveAbstractMethods(tree core.ParseTree) ([]*ast.FunctionDeclarationNode, error) {
	data, err := tree.Data()
	if err != nil {
		return nil, fmt.Errorf("failed to get data from parse tree: %w", err)
	}

	var functions []*ast.FunctionDeclarationNode

	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(tsAbstractMethodQuery, func(m *sitter.QueryMatch) error {
			if len(m.Captures) < 2 {
				return nil
			}

			var methodNameNode, paramsNode *sitter.Node

			for _, capture := range m.Captures {
				switch capture.Node.Type() {
				case "property_identifier":
					methodNameNode = capture.Node
				case "formal_parameters":
					paramsNode = capture.Node
				}
			}

			if methodNameNode == nil {
				return nil
			}

			parentClassName := findESParentClassName(methodNameNode, *data)

			functionNode := ast.NewFunctionDeclarationNode(data)
			functionNode.SetFunctionNameNode(methodNameNode)
			functionNode.SetFunctionType(ast.FunctionTypeMethod)
			functionNode.SetIsAbstract(true)

			if parentClassName != "" {
				functionNode.SetParentClassName(parentClassName)
			}

			if paramsNode != nil {
				paramNodes := extractESParameterNodes(paramsNode)
				functionNode.SetFunctionParameterNodes(paramNodes)
			}

			// Extract access modifier from parent node
			methodDefNode := methodNameNode.Parent()
			accessModifier := ast.AccessModifierPublic
			if methodDefNode != nil {
				accessModifier = extractESMethodAccessModifier(methodDefNode, *data)
			}
			functionNode.SetAccessModifier(accessModifier)

			functions = append(functions, functionNode)
			return nil
		}),
	}

	err = ts.ExecuteQueries(ts.NewQueriesRequest(r.language, queryRequestItems), data, tree)
	if err != nil {
		return nil, err
	}

	return functions, nil
}

// --- TypeScript Class/Interface/Enum Queries ---

const tsClassDefinitionQuery = `
	(class_declaration
		name: (type_identifier) @class_name
		body: (class_body) @class_body)
`

const tsAbstractClassDefinitionQuery = `
	(abstract_class_declaration
		name: (type_identifier) @class_name
		body: (class_body) @class_body)
`

const tsInterfaceDefinitionQuery = `
	(interface_declaration
		name: (type_identifier) @interface_name
		body: (interface_body) @interface_body)
`

const tsEnumDefinitionQuery = `
	(enum_declaration
		name: (identifier) @enum_name
		body: (enum_body) @enum_body)
`

const tsClassMethodQuery = `
	(class_declaration
		body: (class_body
			(method_definition
				name: (property_identifier) @method_name
				parameters: (formal_parameters) @method_params) @method_def))
`

const tsAbstractClassMethodQuery = `
	(abstract_class_declaration
		body: (class_body
			(method_definition
				name: (property_identifier) @method_name
				parameters: (formal_parameters) @method_params) @method_def))
`

const tsClassFieldQuery = `
	(class_declaration
		body: (class_body
			(public_field_definition
				name: (property_identifier) @field_name) @field_def))
`

const tsAbstractClassFieldQuery = `
	(abstract_class_declaration
		body: (class_body
			(public_field_definition
				name: (property_identifier) @field_name) @field_def))
`

const tsInheritanceQuery = `
	(class_declaration
		name: (type_identifier) @class_name
		(class_heritage
			(extends_clause
				value: (_) @parent_class_name)))
`

const tsAbstractInheritanceQuery = `
	(abstract_class_declaration
		name: (type_identifier) @class_name
		(class_heritage
			(extends_clause
				value: (_) @parent_class_name)))
`

const tsImplementsQuery = `
	(class_declaration
		name: (type_identifier) @class_name
		(class_heritage
			(implements_clause
				(type_identifier) @interface_name)))
`

const tsAbstractImplementsQuery = `
	(abstract_class_declaration
		name: (type_identifier) @class_name
		(class_heritage
			(implements_clause
				(type_identifier) @interface_name)))
`

const tsInterfaceExtendsQuery = `
	(interface_declaration
		name: (type_identifier) @interface_name
		(extends_type_clause
			(type_identifier) @parent_interface_name))
`

// ResolveClasses extracts class, abstract class, interface, and enum declarations from TypeScript parse tree
func (r *typescriptResolvers) ResolveClasses(tree core.ParseTree) ([]*ast.ClassDeclarationNode, error) {
	data, err := tree.Data()
	if err != nil {
		return nil, fmt.Errorf("failed to get data from parse tree: %w", err)
	}

	var classes []*ast.ClassDeclarationNode
	classMap := make(map[string]*ast.ClassDeclarationNode)

	// Step 1: Extract class definitions (regular + abstract + interface + enum)
	err = r.extractClassDefinitions(data, tree, &classes, classMap)
	if err != nil {
		return nil, fmt.Errorf("failed to extract class definitions: %w", err)
	}

	// Step 2: Extract methods and constructors
	err = r.extractClassMethods(data, tree, classMap)
	if err != nil {
		return nil, fmt.Errorf("failed to extract class methods: %w", err)
	}

	// Step 3: Extract fields
	err = r.extractClassFields(data, tree, classMap)
	if err != nil {
		return nil, fmt.Errorf("failed to extract class fields: %w", err)
	}

	// Step 4: Extract inheritance (extends + implements)
	err = r.enrichClassInheritance(data, tree, classMap)
	if err != nil {
		return nil, fmt.Errorf("failed to extract class inheritance: %w", err)
	}

	return classes, nil
}

func (r *typescriptResolvers) extractClassDefinitions(data *[]byte, tree core.ParseTree,
	classes *[]*ast.ClassDeclarationNode, classMap map[string]*ast.ClassDeclarationNode) error {
	queryRequestItems := []ts.QueryItem{
		// Regular classes
		ts.NewQueryItem(tsClassDefinitionQuery, func(m *sitter.QueryMatch) error {
			classNode := ast.NewClassDeclarationNode(ast.ToContent(*data))
			var className string

			for _, capture := range m.Captures {
				if capture.Node.Type() == "type_identifier" && className == "" {
					className = capture.Node.Content(*data)
					classNode.SetClassNameNode(capture.Node)
				}
			}

			if className != "" {
				classNode.SetAccessModifier(ast.AccessModifierPublic)
				// Check for decorators on the class
				r.extractClassDecorators(m, classNode, *data)
				*classes = append(*classes, classNode)
				classMap[className] = classNode
			}
			return nil
		}),

		// Abstract classes
		ts.NewQueryItem(tsAbstractClassDefinitionQuery, func(m *sitter.QueryMatch) error {
			classNode := ast.NewClassDeclarationNode(ast.ToContent(*data))
			classNode.SetIsAbstract(true)
			var className string

			for _, capture := range m.Captures {
				if capture.Node.Type() == "type_identifier" && className == "" {
					className = capture.Node.Content(*data)
					classNode.SetClassNameNode(capture.Node)
				}
			}

			if className != "" {
				classNode.SetAccessModifier(ast.AccessModifierPublic)
				// Check for decorators on the abstract_class_declaration parent
				r.extractClassDecorators(m, classNode, *data)
				*classes = append(*classes, classNode)
				classMap[className] = classNode
			}
			return nil
		}),

		// Interfaces (treated as abstract class-like constructs)
		ts.NewQueryItem(tsInterfaceDefinitionQuery, func(m *sitter.QueryMatch) error {
			classNode := ast.NewClassDeclarationNode(ast.ToContent(*data))
			classNode.SetIsAbstract(true)
			var className string

			for _, capture := range m.Captures {
				if capture.Node.Type() == "type_identifier" && className == "" {
					className = capture.Node.Content(*data)
					classNode.SetClassNameNode(capture.Node)
				}
			}

			if className != "" {
				classNode.SetAccessModifier(ast.AccessModifierPublic)
				*classes = append(*classes, classNode)
				classMap[className] = classNode
			}
			return nil
		}),

		// Enums (treated as class-like constructs)
		ts.NewQueryItem(tsEnumDefinitionQuery, func(m *sitter.QueryMatch) error {
			classNode := ast.NewClassDeclarationNode(ast.ToContent(*data))
			var className string

			for _, capture := range m.Captures {
				if capture.Node.Type() == "identifier" && className == "" {
					className = capture.Node.Content(*data)
					classNode.SetClassNameNode(capture.Node)
				}
			}

			if className != "" {
				classNode.SetAccessModifier(ast.AccessModifierPublic)
				*classes = append(*classes, classNode)
				classMap[className] = classNode
			}
			return nil
		}),
	}

	return ts.ExecuteQueries(ts.NewQueriesRequest(r.language, queryRequestItems), data, tree)
}

func (r *typescriptResolvers) extractClassDecorators(m *sitter.QueryMatch, classNode *ast.ClassDeclarationNode, data []byte) {
	// Walk up from the class_declaration node to find decorators
	for _, capture := range m.Captures {
		if capture.Node.Type() == "type_identifier" {
			// Found the class name node, check for decorators on the parent class_declaration
			classDecl := capture.Node.Parent()
			if classDecl == nil {
				continue
			}

			for i := 0; i < int(classDecl.ChildCount()); i++ {
				child := classDecl.Child(i)
				if child.Type() == "decorator" {
					classNode.AddDecoratorNode(child)
				}
			}
			break
		}
	}
}

func (r *typescriptResolvers) extractClassMethods(data *[]byte, tree core.ParseTree, classMap map[string]*ast.ClassDeclarationNode) error {
	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(tsClassMethodQuery, func(m *sitter.QueryMatch) error {
			var methodNameNode, methodDefNode *sitter.Node

			for _, capture := range m.Captures {
				if capture.Node.Type() == "property_identifier" {
					methodNameNode = capture.Node
				} else if capture.Node.Type() == "method_definition" {
					methodDefNode = capture.Node
				}
			}

			if methodNameNode == nil || methodDefNode == nil {
				return nil
			}

			className := r.findParentClassName(methodDefNode, *data)
			if className == "" {
				return nil
			}

			if classNode, exists := classMap[className]; exists {
				// Set constructor if applicable
				if methodNameNode.Content(*data) == "constructor" {
					classNode.SetConstructorNode(methodDefNode)
				} else {
					classNode.AddMethodNode(methodDefNode)
				}
			}
			return nil
		}),

		ts.NewQueryItem(tsAbstractClassMethodQuery, func(m *sitter.QueryMatch) error {
			var methodNameNode, methodDefNode *sitter.Node

			for _, capture := range m.Captures {
				if capture.Node.Type() == "property_identifier" {
					methodNameNode = capture.Node
				} else if capture.Node.Type() == "method_definition" {
					methodDefNode = capture.Node
				}
			}

			if methodNameNode == nil || methodDefNode == nil {
				return nil
			}

			className := r.findParentClassName(methodDefNode, *data)
			if className == "" {
				return nil
			}

			if classNode, exists := classMap[className]; exists {
				if methodNameNode.Content(*data) == "constructor" {
					classNode.SetConstructorNode(methodDefNode)
				} else {
					classNode.AddMethodNode(methodDefNode)
				}
			}
			return nil
		}),
	}

	return ts.ExecuteQueries(ts.NewQueriesRequest(r.language, queryRequestItems), data, tree)
}

func (r *typescriptResolvers) extractClassFields(data *[]byte, tree core.ParseTree, classMap map[string]*ast.ClassDeclarationNode) error {
	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(tsClassFieldQuery, func(m *sitter.QueryMatch) error {
			var fieldDefNode *sitter.Node

			for _, capture := range m.Captures {
				if capture.Node.Type() == "public_field_definition" {
					fieldDefNode = capture.Node
					break
				}
			}

			if fieldDefNode == nil {
				return nil
			}

			className := r.findParentClassName(fieldDefNode, *data)
			if className == "" {
				return nil
			}

			if classNode, exists := classMap[className]; exists {
				classNode.AddFieldNode(fieldDefNode)
			}
			return nil
		}),

		ts.NewQueryItem(tsAbstractClassFieldQuery, func(m *sitter.QueryMatch) error {
			var fieldDefNode *sitter.Node

			for _, capture := range m.Captures {
				if capture.Node.Type() == "public_field_definition" {
					fieldDefNode = capture.Node
					break
				}
			}

			if fieldDefNode == nil {
				return nil
			}

			className := r.findParentClassName(fieldDefNode, *data)
			if className == "" {
				return nil
			}

			if classNode, exists := classMap[className]; exists {
				classNode.AddFieldNode(fieldDefNode)
			}
			return nil
		}),
	}

	return ts.ExecuteQueries(ts.NewQueriesRequest(r.language, queryRequestItems), data, tree)
}

func (r *typescriptResolvers) enrichClassInheritance(data *[]byte, tree core.ParseTree, classMap map[string]*ast.ClassDeclarationNode) error {
	queryRequestItems := []ts.QueryItem{
		// Regular class extends
		ts.NewQueryItem(tsInheritanceQuery, func(m *sitter.QueryMatch) error {
			return r.processInheritanceMatch(m, data, classMap)
		}),

		// Abstract class extends
		ts.NewQueryItem(tsAbstractInheritanceQuery, func(m *sitter.QueryMatch) error {
			return r.processInheritanceMatch(m, data, classMap)
		}),

		// Regular class implements
		ts.NewQueryItem(tsImplementsQuery, func(m *sitter.QueryMatch) error {
			return r.processImplementsMatch(m, data, classMap)
		}),

		// Abstract class implements
		ts.NewQueryItem(tsAbstractImplementsQuery, func(m *sitter.QueryMatch) error {
			return r.processImplementsMatch(m, data, classMap)
		}),

		// Interface extends
		ts.NewQueryItem(tsInterfaceExtendsQuery, func(m *sitter.QueryMatch) error {
			return r.processInheritanceMatch(m, data, classMap)
		}),
	}

	return ts.ExecuteQueries(ts.NewQueriesRequest(r.language, queryRequestItems), data, tree)
}

func (r *typescriptResolvers) processInheritanceMatch(m *sitter.QueryMatch, data *[]byte, classMap map[string]*ast.ClassDeclarationNode) error {
	if len(m.Captures) < 2 {
		return nil
	}

	var className string
	var parentNode *sitter.Node

	for _, capture := range m.Captures {
		content := capture.Node.Content(*data)
		if capture.Node.Type() == "type_identifier" {
			if className == "" {
				className = content
			} else {
				parentNode = capture.Node
			}
		} else if capture.Node.Type() == "identifier" {
			// extends clause can capture an identifier instead of type_identifier
			parentNode = capture.Node
		}
	}

	if className != "" && parentNode != nil {
		if classNode, exists := classMap[className]; exists {
			classNode.AddBaseClassNode(parentNode)
		}
	}
	return nil
}

func (r *typescriptResolvers) processImplementsMatch(m *sitter.QueryMatch, data *[]byte, classMap map[string]*ast.ClassDeclarationNode) error {
	if len(m.Captures) < 2 {
		return nil
	}

	var className string
	for _, capture := range m.Captures {
		content := capture.Node.Content(*data)
		if capture.Node.Type() == "type_identifier" {
			if className == "" {
				className = content
			} else {
				// This is an implemented interface
				if classNode, exists := classMap[className]; exists {
					classNode.AddBaseClassNode(capture.Node)
				}
			}
		}
	}
	return nil
}

// ResolveInheritance builds inheritance graph from TypeScript classes, interfaces, and abstract classes
func (r *typescriptResolvers) ResolveInheritance(tree core.ParseTree) (*ast.InheritanceGraph, error) {
	data, err := tree.Data()
	if err != nil {
		return nil, fmt.Errorf("failed to get data from parse tree: %w", err)
	}

	inheritanceGraph := ast.NewInheritanceGraph()

	addRelationship := func(m *sitter.QueryMatch) error {
		if len(m.Captures) < 2 {
			return nil
		}

		var className, parentClassName string
		for _, capture := range m.Captures {
			content := capture.Node.Content(*data)
			if capture.Node.Type() == "type_identifier" || capture.Node.Type() == "identifier" {
				if className == "" {
					className = content
				} else if parentClassName == "" {
					parentClassName = content
				}
			}
		}

		if className != "" && parentClassName != "" {
			file, _ := tree.File()
			filename := ""
			if file != nil {
				filename = file.Name()
			}
			inheritanceGraph.AddRelationship(className, parentClassName, ast.RelationshipTypeInherits, filename, 0)
		}
		return nil
	}

	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(tsInheritanceQuery, func(m *sitter.QueryMatch) error {
			return addRelationship(m)
		}),
		ts.NewQueryItem(tsAbstractInheritanceQuery, func(m *sitter.QueryMatch) error {
			return addRelationship(m)
		}),
		ts.NewQueryItem(tsImplementsQuery, func(m *sitter.QueryMatch) error {
			return addRelationship(m)
		}),
		ts.NewQueryItem(tsAbstractImplementsQuery, func(m *sitter.QueryMatch) error {
			return addRelationship(m)
		}),
		ts.NewQueryItem(tsInterfaceExtendsQuery, func(m *sitter.QueryMatch) error {
			return addRelationship(m)
		}),
	}

	err = ts.ExecuteQueries(ts.NewQueriesRequest(r.language, queryRequestItems), data, tree)
	if err != nil {
		return nil, fmt.Errorf("failed to execute inheritance queries: %w", err)
	}

	return inheritanceGraph, nil
}

// Helper methods

func (r *typescriptResolvers) findParentClassName(node *sitter.Node, data []byte) string {
	return findESParentClassName(node, data)
}
