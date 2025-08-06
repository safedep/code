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
var _ core.ObjectOrientedLanguageResolvers = (*javaResolvers)(nil)

const javaImportQuery = `
	(import_declaration
		(scoped_identifier) @module_name
		(asterisk)? @wildcard)
`

// Java class analysis queries
const javaClassDefinitionQuery = `
	(class_declaration
		(modifiers)? @modifiers
		name: (identifier) @class_name
		superclass: (superclass
			(type_identifier) @superclass_name)?
		body: (class_body) @class_body)
`

// Separate query for annotations on classes
const javaClassAnnotationQuery = `
	[
		(class_declaration
			(modifiers
				(annotation) @annotation
			)
			name: (identifier) @class_name)
		(class_declaration
			(modifiers
				(marker_annotation) @annotation
			)
			name: (identifier) @class_name)
	]
`

const javaInterfaceDefinitionQuery = `
	(interface_declaration
		name: (identifier) @interface_name
		body: (interface_body) @interface_body)
`

const javaClassMethodQuery = `
	(class_declaration
		body: (class_body
			(method_declaration
				name: (identifier) @method_name
				parameters: (formal_parameters) @method_params) @method_def))
`

const javaClassConstructorQuery = `
	(class_declaration
		name: (identifier) @class_name
		body: (class_body
			(constructor_declaration
				name: (identifier) @constructor_name
				parameters: (formal_parameters) @constructor_params) @constructor_def))
`

const javaClassFieldQuery = `
	(class_declaration
		body: (class_body
			(field_declaration
				declarator: (variable_declarator
					name: (identifier) @field_name)) @field_def))
`


const javaInheritanceQuery = `
	(class_declaration
		name: (identifier) @class_name
		superclass: (superclass
			(type_identifier) @parent_class_name))
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

// resolveAliasNode resolves the alias node for the import declaration.
// In Java, the alias is typically the last part of the scoped identifier.
// For example, in `import com.example.MyClass`, the alias is `MyClass`.
// If the alias is not present, it returns the module name node itself.
func (r *javaResolvers) resolveAliasNode(moduleNameNode *sitter.Node) *sitter.Node {
	aliasNode := moduleNameNode.ChildByFieldName("name")
	if aliasNode == nil {
		return moduleNameNode
	}

	return aliasNode
}

// ResolveClasses extracts class declarations from Java parse tree
func (r *javaResolvers) ResolveClasses(tree core.ParseTree) ([]*ast.ClassDeclarationNode, error) {
	data, err := tree.Data()
	if err != nil {
		return nil, fmt.Errorf("failed to get data from parse tree: %w", err)
	}

	var classes []*ast.ClassDeclarationNode

	// Step 1: Extract basic class definitions
	classMap := make(map[string]*ast.ClassDeclarationNode)
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

	// Step 4: Extract annotations with separate query
	err = r.extractClassAnnotations(data, tree, classMap)
	if err != nil {
		return nil, fmt.Errorf("failed to extract class annotations: %w", err)
	}

	return classes, nil
}

// ResolveInheritance builds inheritance graph from Java classes
func (r *javaResolvers) ResolveInheritance(tree core.ParseTree) (*ast.InheritanceGraph, error) {
	data, err := tree.Data()
	if err != nil {
		return nil, fmt.Errorf("failed to get data from parse tree: %w", err)
	}

	inheritanceGraph := ast.NewInheritanceGraph()

	// Extract inheritance relationships from class definitions
	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(javaInheritanceQuery, func(m *sitter.QueryMatch) error {
			if len(m.Captures) < 2 {
				return nil
			}

			var className, parentClassName string
			for _, capture := range m.Captures {
				if capture.Node.Type() == "identifier" {
					content := capture.Node.Content(*data)
					if className == "" {
						className = content
					}
				} else if capture.Node.Type() == "type_identifier" {
					parentClassName = capture.Node.Content(*data)
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
		}),
	}

	err = ts.ExecuteQueries(ts.NewQueriesRequest(r.language, queryRequestItems), data, tree)
	if err != nil {
		return nil, fmt.Errorf("failed to execute inheritance queries: %w", err)
	}

	return inheritanceGraph, nil
}

// Helper methods for Java class extraction

func (r *javaResolvers) extractClassDefinitions(data *[]byte, tree core.ParseTree, classes *[]*ast.ClassDeclarationNode, classMap map[string]*ast.ClassDeclarationNode) error {
	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(javaClassDefinitionQuery, func(m *sitter.QueryMatch) error {
			classNode := ast.NewClassDeclarationNode(ast.ToContent(*data))
			
			var className string
			var modifiersNode *sitter.Node
			
			for _, capture := range m.Captures {
				switch capture.Node.Type() {
				case "identifier":
					// First identifier is the class name
					if className == "" {
						className = capture.Node.Content(*data)
						classNode.SetClassNameNode(capture.Node)
					}
				case "type_identifier":
					// Subsequent type_identifiers are superclass or interfaces
					classNode.AddBaseClassNode(capture.Node)
				case "modifiers":
					modifiersNode = capture.Node
				}
			}
			
			if className != "" {
				// Check for abstract modifier and extract annotations
				if modifiersNode != nil {
					if r.hasAbstractModifier(modifiersNode, *data) {
						classNode.SetIsAbstract(true)
					}
					r.extractAnnotationsFromModifiers(modifiersNode, classNode)
				}
				
				classNode.SetAccessModifier(r.extractAccessModifier(m))
				*classes = append(*classes, classNode)
				classMap[className] = classNode
			}

			return nil
		}),
		
		// Also handle interfaces as classes (Java interfaces are class-like)
		ts.NewQueryItem(javaInterfaceDefinitionQuery, func(m *sitter.QueryMatch) error {
			classNode := ast.NewClassDeclarationNode(ast.ToContent(*data))
			classNode.SetIsAbstract(true) // Interfaces are abstract by nature
			
			var className string
			
			for _, capture := range m.Captures {
				switch capture.Node.Type() {
				case "identifier":
					if className == "" {
						className = capture.Node.Content(*data)
						classNode.SetClassNameNode(capture.Node)
					}
				case "type_identifier":
					// Parent interfaces
					classNode.AddBaseClassNode(capture.Node)
				}
			}
			
			if className != "" {
				classNode.SetAccessModifier(ast.AccessModifierPublic) // Interfaces are public by default
				*classes = append(*classes, classNode)
				classMap[className] = classNode
			}

			return nil
		}),
	}

	return ts.ExecuteQueries(ts.NewQueriesRequest(r.language, queryRequestItems), data, tree)
}

func (r *javaResolvers) extractClassMethods(data *[]byte, tree core.ParseTree, classMap map[string]*ast.ClassDeclarationNode) error {
	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(javaClassMethodQuery, func(m *sitter.QueryMatch) error {
			var methodNameNode, methodDefNode *sitter.Node
			
			for _, capture := range m.Captures {
				if capture.Node.Type() == "identifier" {
					methodNameNode = capture.Node
				} else if capture.Node.Type() == "method_declaration" {
					methodDefNode = capture.Node
				}
			}
			
			if methodNameNode == nil || methodDefNode == nil {
				return nil
			}

			// Find the class this method belongs to
			className := r.findParentClassName(methodDefNode, *data)
			if className == "" {
				return nil
			}

			if classNode, exists := classMap[className]; exists {
				classNode.AddMethodNode(methodDefNode)
			}

			return nil
		}),
		
		// Handle constructors separately
		ts.NewQueryItem(javaClassConstructorQuery, func(m *sitter.QueryMatch) error {
			var constructorDefNode *sitter.Node
			var className string
			
			for _, capture := range m.Captures {
				if capture.Node.Type() == "identifier" {
					className = capture.Node.Content(*data)
				} else if capture.Node.Type() == "constructor_declaration" {
					constructorDefNode = capture.Node
				}
			}
			
			if className != "" && constructorDefNode != nil {
				if classNode, exists := classMap[className]; exists {
					classNode.SetConstructorNode(constructorDefNode)
				}
			}

			return nil
		}),
	}

	return ts.ExecuteQueries(ts.NewQueriesRequest(r.language, queryRequestItems), data, tree)
}

func (r *javaResolvers) extractClassFields(data *[]byte, tree core.ParseTree, classMap map[string]*ast.ClassDeclarationNode) error {
	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(javaClassFieldQuery, func(m *sitter.QueryMatch) error {
			var fieldDefNode *sitter.Node
			
			for _, capture := range m.Captures {
				if capture.Node.Type() == "field_declaration" {
					fieldDefNode = capture.Node
					break
				}
			}
			
			if fieldDefNode == nil {
				return nil
			}

			// Find the class this field belongs to
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

func (r *javaResolvers) extractClassAnnotations(data *[]byte, tree core.ParseTree, classMap map[string]*ast.ClassDeclarationNode) error {
	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(javaClassAnnotationQuery, func(m *sitter.QueryMatch) error {
			var annotationNode *sitter.Node
			var className string
			
			for _, capture := range m.Captures {
				if capture.Node.Type() == "annotation" || capture.Node.Type() == "marker_annotation" {
					annotationNode = capture.Node
				} else if capture.Node.Type() == "identifier" {
					className = capture.Node.Content(*data)
				}
			}
			
			if className != "" && annotationNode != nil {
				if classNode, exists := classMap[className]; exists {
					classNode.AddDecoratorNode(annotationNode)
				}
			}

			return nil
		}),
	}

	return ts.ExecuteQueries(ts.NewQueriesRequest(r.language, queryRequestItems), data, tree)
}

// Helper methods

func (r *javaResolvers) findParentClassName(node *sitter.Node, data []byte) string {
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

func (r *javaResolvers) extractAccessModifier(m *sitter.QueryMatch) ast.AccessModifier {
	// In Java, we need to look for modifiers in the parent nodes
	// For now, default to public (can be enhanced later)
	return ast.AccessModifierPublic
}


func (r *javaResolvers) hasAbstractModifier(modifiersNode *sitter.Node, data []byte) bool {
	if modifiersNode == nil {
		return false
	}
	
	// Traverse child nodes looking for "abstract" keyword
	for i := 0; i < int(modifiersNode.ChildCount()); i++ {
		child := modifiersNode.Child(i)
		if child != nil && child.Type() == "abstract" {
			return true
		}
	}
	return false
}

func (r *javaResolvers) extractAnnotationsFromModifiers(modifiersNode *sitter.Node, classNode *ast.ClassDeclarationNode) {
	if modifiersNode == nil {
		return
	}
	
	// Traverse child nodes looking for annotations
	for i := 0; i < int(modifiersNode.ChildCount()); i++ {
		child := modifiersNode.Child(i)
		if child != nil {
			if child.Type() == "annotation" {
				classNode.AddDecoratorNode(child)
			}
		}
	}
}
