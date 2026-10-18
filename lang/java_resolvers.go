package lang

import (
	"context"
	"fmt"

	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
	"github.com/safedep/code/pkg/ts"
	sitter "github.com/smacker/go-tree-sitter"
)

type javaResolvers struct{}

var _ core.LanguageResolvers = (*javaResolvers)(nil)
var _ core.ObjectOrientedLanguageResolvers = (*javaResolvers)(nil)

const javaImportQuery = `
	(import_declaration
		(scoped_identifier) @module_name
		(asterisk)? @wildcard)
`

const javaClassDefinitionQuery = `
	(class_declaration
		(modifiers)? @modifiers
		name: (identifier) @class_name
		superclass: (superclass
			(type_identifier) @superclass_name)?
		body: (class_body) @class_body)
`

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

func (r *javaResolvers) ResolveImports(ctx context.Context, tree core.ParseTree) ([]*ast.ImportNode, error) {
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

	err = ts.ExecuteQueries(ctx, tree, queryRequestItems)
	if err != nil {
		return nil, err
	}

	return imports, err
}

// resolveAliasNode returns the last part of the scoped identifier of an
// import, as MyClass in import com.example.MyClass, or the module name
// node when it has no name part.
func (r *javaResolvers) resolveAliasNode(moduleNameNode *sitter.Node) *sitter.Node {
	aliasNode := moduleNameNode.ChildByFieldName("name")
	if aliasNode == nil {
		return moduleNameNode
	}

	return aliasNode
}

func (r *javaResolvers) ResolveClasses(ctx context.Context, tree core.ParseTree) ([]*ast.ClassDeclarationNode, error) {
	data, err := tree.Data()
	if err != nil {
		return nil, fmt.Errorf("failed to get data from parse tree: %w", err)
	}

	var classes []*ast.ClassDeclarationNode

	classMap := make(map[string]*ast.ClassDeclarationNode)
	err = r.extractClassDefinitions(ctx, data, tree, &classes, classMap)
	if err != nil {
		return nil, fmt.Errorf("failed to extract class definitions: %w", err)
	}

	err = r.extractClassMethods(ctx, data, tree, classMap)
	if err != nil {
		return nil, fmt.Errorf("failed to extract class methods: %w", err)
	}

	err = r.extractClassFields(ctx, data, tree, classMap)
	if err != nil {
		return nil, fmt.Errorf("failed to extract class fields: %w", err)
	}

	err = r.extractClassAnnotations(ctx, data, tree, classMap)
	if err != nil {
		return nil, fmt.Errorf("failed to extract class annotations: %w", err)
	}

	return classes, nil
}

func (r *javaResolvers) ResolveInheritance(ctx context.Context, tree core.ParseTree) (*ast.InheritanceGraph, error) {
	data, err := tree.Data()
	if err != nil {
		return nil, fmt.Errorf("failed to get data from parse tree: %w", err)
	}

	inheritanceGraph := ast.NewInheritanceGraph()

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

	err = ts.ExecuteQueries(ctx, tree, queryRequestItems)
	if err != nil {
		return nil, fmt.Errorf("failed to execute inheritance queries: %w", err)
	}

	return inheritanceGraph, nil
}

func (r *javaResolvers) extractClassDefinitions(ctx context.Context, data *[]byte, tree core.ParseTree,
	classes *[]*ast.ClassDeclarationNode, classMap map[string]*ast.ClassDeclarationNode) error {
	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(javaClassDefinitionQuery, func(m *sitter.QueryMatch) error {
			classNode := ast.NewClassDeclarationNode(ast.ToContent(*data))

			var className string
			var modifiersNode *sitter.Node

			for _, capture := range m.Captures {
				switch capture.Node.Type() {
				case "identifier":
					if className == "" {
						className = capture.Node.Content(*data)
						classNode.SetClassNameNode(capture.Node)
					}
				case "type_identifier":
					classNode.AddBaseClassNode(capture.Node)
				case "modifiers":
					modifiersNode = capture.Node
				}
			}

			if className != "" {
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

		ts.NewQueryItem(javaInterfaceDefinitionQuery, func(m *sitter.QueryMatch) error {
			classNode := ast.NewClassDeclarationNode(ast.ToContent(*data))
			classNode.SetIsAbstract(true)

			var className string

			for _, capture := range m.Captures {
				switch capture.Node.Type() {
				case "identifier":
					if className == "" {
						className = capture.Node.Content(*data)
						classNode.SetClassNameNode(capture.Node)
					}
				case "type_identifier":
					classNode.AddBaseClassNode(capture.Node)
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

	return ts.ExecuteQueries(ctx, tree, queryRequestItems)
}

func (r *javaResolvers) extractClassMethods(ctx context.Context, data *[]byte, tree core.ParseTree, classMap map[string]*ast.ClassDeclarationNode) error {
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

			className := r.findParentClassName(methodDefNode, *data)
			if className == "" {
				return nil
			}

			if classNode, exists := classMap[className]; exists {
				classNode.AddMethodNode(methodDefNode)
			}

			return nil
		}),

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

	return ts.ExecuteQueries(ctx, tree, queryRequestItems)
}

func (r *javaResolvers) extractClassFields(ctx context.Context, data *[]byte, tree core.ParseTree, classMap map[string]*ast.ClassDeclarationNode) error {
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

	return ts.ExecuteQueries(ctx, tree, queryRequestItems)
}

func (r *javaResolvers) extractClassAnnotations(ctx context.Context, data *[]byte, tree core.ParseTree, classMap map[string]*ast.ClassDeclarationNode) error {
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

	return ts.ExecuteQueries(ctx, tree, queryRequestItems)
}

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

func (r *javaResolvers) extractAccessModifier(_ *sitter.QueryMatch) ast.AccessModifier {
	return ast.AccessModifierPublic
}

func (r *javaResolvers) hasAbstractModifier(modifiersNode *sitter.Node, data []byte) bool {
	if modifiersNode == nil {
		return false
	}

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

	for i := 0; i < int(modifiersNode.ChildCount()); i++ {
		child := modifiersNode.Child(i)
		if child != nil {
			if child.Type() == "annotation" {
				classNode.AddDecoratorNode(child)
			}
		}
	}
}

const javaFunctionDefinitionQuery = `
	(method_declaration
		(modifiers)? @modifiers
		type: (_) @return_type
		name: (identifier) @function_name
		parameters: (formal_parameters) @function_params
		body: (block) @function_body)
`

const javaConstructorDefinitionQuery = `
	(constructor_declaration
		(modifiers)? @modifiers
		name: (identifier) @constructor_name
		parameters: (formal_parameters) @constructor_params
		body: (constructor_body) @constructor_body)
`

const javaFunctionAnnotationQuery = `
	(method_declaration
		(modifiers
			(annotation) @annotation)
		name: (identifier) @function_name)
`

func (r *javaResolvers) ResolveFunctions(ctx context.Context, tree core.ParseTree) ([]*ast.FunctionDeclarationNode, error) {
	data, err := tree.Data()
	if err != nil {
		return nil, fmt.Errorf("failed to get data from parse tree: %w", err)
	}

	var functions []*ast.FunctionDeclarationNode
	functionMap := make(map[string]*ast.FunctionDeclarationNode)

	err = r.extractJavaFunctions(ctx, data, tree, functionMap)
	if err != nil {
		return nil, fmt.Errorf("failed to extract Java functions: %w", err)
	}

	err = r.extractJavaConstructors(ctx, data, tree, functionMap)
	if err != nil {
		return nil, fmt.Errorf("failed to extract Java constructors: %w", err)
	}

	err = r.extractJavaFunctionAnnotations(ctx, data, tree, functionMap)
	if err != nil {
		return nil, fmt.Errorf("failed to extract Java function annotations: %w", err)
	}

	for _, function := range functionMap {
		functions = append(functions, function)
	}

	return functions, nil
}

func (r *javaResolvers) extractJavaFunctions(ctx context.Context, data *[]byte, tree core.ParseTree,
	functionMap map[string]*ast.FunctionDeclarationNode) error {
	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(javaFunctionDefinitionQuery, func(m *sitter.QueryMatch) error {
			if len(m.Captures) < 4 {
				return nil
			}

			var functionNameNode, returnTypeNode, paramsNode, bodyNode, modifiersNode *sitter.Node

			for _, capture := range m.Captures {
				switch capture.Node.Type() {
				case "identifier":
					functionNameNode = capture.Node
				case "formal_parameters":
					paramsNode = capture.Node
				case "block":
					bodyNode = capture.Node
				case "modifiers":
					modifiersNode = capture.Node
				default:
					if returnTypeNode == nil {
						returnTypeNode = capture.Node
					}
				}
			}

			if functionNameNode == nil {
				return nil
			}

			functionKey := r.generateJavaFunctionKey(functionNameNode, *data)
			functionNode := ast.NewFunctionDeclarationNode(data)
			functionNode.SetFunctionNameNode(functionNameNode)

			if paramsNode != nil {
				paramNodes := r.extractJavaParameterNodes(paramsNode)
				functionNode.SetFunctionParameterNodes(paramNodes)
			}

			if returnTypeNode != nil {
				functionNode.SetFunctionReturnTypeNode(returnTypeNode)
			}

			if bodyNode != nil {
				functionNode.SetFunctionBodyNode(bodyNode)
			}

			parentClassName := r.findParentClassName(functionNameNode, *data)
			if parentClassName != "" {
				functionNode.SetParentClassName(parentClassName)
				functionNode.SetFunctionType(ast.FunctionTypeMethod)
			} else {
				functionNode.SetFunctionType(ast.FunctionTypeFunction)
			}

			if modifiersNode != nil {
				r.processJavaModifiers(modifiersNode, functionNode, *data)
			} else {
				functionNode.SetAccessModifier(ast.AccessModifierPackage) // Default in Java
			}

			functionMap[functionKey] = functionNode
			return nil
		}),
	}

	return ts.ExecuteQueries(ctx, tree, queryRequestItems)
}

func (r *javaResolvers) extractJavaConstructors(ctx context.Context, data *[]byte, tree core.ParseTree,
	functionMap map[string]*ast.FunctionDeclarationNode) error {
	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(javaConstructorDefinitionQuery, func(m *sitter.QueryMatch) error {
			if len(m.Captures) < 3 {
				return nil
			}

			var constructorNameNode, paramsNode, bodyNode, modifiersNode *sitter.Node

			for _, capture := range m.Captures {
				switch capture.Node.Type() {
				case "identifier":
					constructorNameNode = capture.Node
				case "formal_parameters":
					paramsNode = capture.Node
				case "constructor_body":
					bodyNode = capture.Node
				case "modifiers":
					modifiersNode = capture.Node
				}
			}

			if constructorNameNode == nil {
				return nil
			}

			functionKey := r.generateJavaFunctionKey(constructorNameNode, *data)
			functionNode := ast.NewFunctionDeclarationNode(data)
			functionNode.SetFunctionNameNode(constructorNameNode)
			functionNode.SetFunctionType(ast.FunctionTypeConstructor)

			if paramsNode != nil {
				paramNodes := r.extractJavaParameterNodes(paramsNode)
				functionNode.SetFunctionParameterNodes(paramNodes)
			}

			if bodyNode != nil {
				functionNode.SetFunctionBodyNode(bodyNode)
			}

			parentClassName := r.findParentClassName(constructorNameNode, *data)
			if parentClassName != "" {
				functionNode.SetParentClassName(parentClassName)
			}

			if modifiersNode != nil {
				r.processJavaModifiers(modifiersNode, functionNode, *data)
			} else {
				functionNode.SetAccessModifier(ast.AccessModifierPackage) // Default in Java
			}

			functionMap[functionKey] = functionNode
			return nil
		}),
	}

	return ts.ExecuteQueries(ctx, tree, queryRequestItems)
}

func (r *javaResolvers) extractJavaFunctionAnnotations(ctx context.Context, data *[]byte, tree core.ParseTree,
	functionMap map[string]*ast.FunctionDeclarationNode) error {
	queryRequestItems := []ts.QueryItem{
		ts.NewQueryItem(javaFunctionAnnotationQuery, func(m *sitter.QueryMatch) error {
			if len(m.Captures) < 2 {
				return nil
			}

			var annotationNode, functionNameNode *sitter.Node
			for _, capture := range m.Captures {
				if capture.Node.Type() == "annotation" {
					annotationNode = capture.Node
				} else if capture.Node.Type() == "identifier" {
					functionNameNode = capture.Node
				}
			}

			if annotationNode == nil || functionNameNode == nil {
				return nil
			}

			functionKey := r.generateJavaFunctionKey(functionNameNode, *data)
			if functionNode, exists := functionMap[functionKey]; exists {
				functionNode.AddDecoratorNode(annotationNode)
			}

			return nil
		}),
	}

	return ts.ExecuteQueries(ctx, tree, queryRequestItems)
}

func (r *javaResolvers) extractJavaParameterNodes(parametersNode *sitter.Node) []*sitter.Node {
	var paramNodes []*sitter.Node

	if parametersNode == nil {
		return paramNodes
	}

	for i := 0; i < int(parametersNode.ChildCount()); i++ {
		child := parametersNode.Child(i)
		if child.Type() == "formal_parameter" || child.Type() == "spread_parameter" {
			paramNodes = append(paramNodes, child)
		}
	}

	return paramNodes
}

// Note: We are not handling the modifier based on content currently.
func (r *javaResolvers) processJavaModifiers(modifiersNode *sitter.Node,
	functionNode *ast.FunctionDeclarationNode, _ []byte) {
	if modifiersNode == nil {
		return
	}

	accessModifier := ast.AccessModifierPackage

	for i := 0; i < int(modifiersNode.ChildCount()); i++ {
		child := modifiersNode.Child(i)
		if child != nil {
			switch child.Type() {
			case "public":
				accessModifier = ast.AccessModifierPublic
			case "private":
				accessModifier = ast.AccessModifierPrivate
			case "protected":
				accessModifier = ast.AccessModifierProtected
			case "static":
				functionNode.SetIsStatic(true)
				if functionNode.GetFunctionType() == ast.FunctionTypeMethod {
					functionNode.SetFunctionType(ast.FunctionTypeStaticMethod)
				}
			case "abstract":
				functionNode.SetIsAbstract(true)
			case "annotation":
				functionNode.AddDecoratorNode(child)
			}
		}
	}

	functionNode.SetAccessModifier(accessModifier)
}

func (r *javaResolvers) generateJavaFunctionKey(functionNameNode *sitter.Node, data []byte) string {
	functionName := functionNameNode.Content(data)
	parentClassName := r.findParentClassName(functionNameNode, data)

	if parentClassName != "" {
		return parentClassName + "." + functionName
	}

	lineNumber := functionNameNode.StartPoint().Row
	return fmt.Sprintf("%s:%d", functionName, lineNumber)
}
