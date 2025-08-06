package scan

import (
	"fmt"

	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
	"github.com/safedep/code/examples/astdb/ent"
	"github.com/safedep/code/examples/astdb/ent/inheritancerelationship"
	"github.com/safedep/code/examples/astdb/ent/symbol"
)

func (fp *fileProcessor) extractAndPersistSymbols(tree core.ParseTree, fileRecord *ent.File) error {
	// Get language from tree
	language, err := tree.Language()
	if err != nil {
		return fmt.Errorf("failed to get language: %w", err)
	}

	// Extract classes if the language supports object-oriented features
	if resolvers, ok := language.Resolvers().(core.ObjectOrientedLanguageResolvers); ok {
		err = fp.extractClasses(tree, resolvers, fileRecord)
		if err != nil {
			return fmt.Errorf("failed to extract classes: %w", err)
		}
	}

	// Extract functions
	if resolvers := language.Resolvers(); resolvers != nil {
		err = fp.extractFunctions(tree, resolvers, fileRecord)
		if err != nil {
			return fmt.Errorf("failed to extract functions: %w", err)
		}
	}

	return nil
}

func (fp *fileProcessor) extractClasses(tree core.ParseTree, resolvers core.ObjectOrientedLanguageResolvers, fileRecord *ent.File) error {
	// Use existing CAF class resolution
	classes, err := resolvers.ResolveClasses(tree)
	if err != nil {
		return fmt.Errorf("failed to resolve classes: %w", err)
	}

	for _, class := range classes {
		err := fp.persistClass(class, fileRecord)
		if err != nil {
			return fmt.Errorf("failed to persist class %s: %w", class.ClassName(), err)
		}
	}

	// Build and persist inheritance relationships
	if len(classes) > 0 {
		err := fp.extractInheritanceRelationships(tree, resolvers, fileRecord)
		if err != nil {
			return fmt.Errorf("failed to extract inheritance: %w", err)
		}
	}

	return nil
}

func (fp *fileProcessor) persistClass(class *ast.ClassDeclarationNode, fileRecord *ent.File) error {
	// Get position from class name node
	var lineNumber, columnNumber int = 1, 1 // Default values
	if classNameNode := class.GetClassNameNode(); classNameNode != nil {
		position := ast.GetNodePosition(classNameNode)
		lineNumber = int(position.StartLine)
		columnNumber = int(position.StartColumn)
	}

	// Create class symbol
	classBuilder := fp.db.Symbol.Create().
		SetName(class.ClassName()).
		SetQualifiedName(fp.buildQualifiedClassName(class, fileRecord)). // Enhanced qualified name
		SetSymbolType(SymbolTypeClass).
		SetScopeType(symbol.ScopeType(fp.determineClassScope(class))). // Proper scope detection
		SetLineNumber(lineNumber).
		SetColumnNumber(columnNumber).
		SetFileID(fileRecord.ID)

	// Set access modifier if available (enhanced)
	classAccessMod := class.AccessModifier()
	if classAccessMod != "" && classAccessMod != ast.AccessModifierUnknown {
		dbAccessMod := fp.convertAccessModifier(classAccessMod)
		if dbAccessMod != "" {
			classBuilder = classBuilder.SetAccessModifier(symbol.AccessModifier(dbAccessMod))
		}
	}

	// Set abstract flag
	if class.IsAbstract() {
		classBuilder = classBuilder.SetIsAbstract(true)
	}

	// Add metadata
	metadata := map[string]interface{}{
		"has_constructor": class.GetConstructorNode() != nil,
		"method_count":    len(class.GetMethodNodes()),
		"field_count":     len(class.GetFieldNodes()),
		"decorator_count": len(class.GetDecoratorNodes()),
		"base_classes":    class.BaseClasses(),
	}
	classBuilder = classBuilder.SetMetadata(metadata)

	classSymbol, err := classBuilder.Save(fp.ctx)
	if err != nil {
		return fmt.Errorf("failed to save class symbol: %w", err)
	}

	// Extract and persist methods
	methods := class.GetMethodNodes()
	for _, method := range methods {
		err := fp.persistMethod(method, classSymbol, fileRecord)
		if err != nil {
			if fp.scanner.config.Verbose {
				fmt.Printf("Warning: failed to persist method: %v\n", err)
			}
		}
	}

	return nil
}

func (fp *fileProcessor) persistMethod(methodNode interface{}, parentClass *ent.Symbol, fileRecord *ent.File) error {
	// NOTE: Methods are now primarily extracted through ResolveFunctions() which properly
	// identifies methods using the FunctionDeclarationNode.IsMethod() functionality.
	// This legacy method extraction is kept for backward compatibility with any methods
	// that might be detected through class resolution but not caught by function resolution.
	
	// In practice, this should rarely be called since ResolveFunctions() handles method extraction
	// comprehensively with proper metadata, access modifiers, and parent class relationships.
	// If this method is frequently called, it indicates a gap in the function resolver.

	// Skip legacy method extraction - rely on ResolveFunctions() for comprehensive method handling
	if fp.scanner.config.Verbose {
		fmt.Printf("Note: Legacy method extraction bypassed - methods handled by ResolveFunctions()\n")
	}
	return nil
}

func (fp *fileProcessor) extractFunctions(tree core.ParseTree, resolvers core.LanguageResolvers, fileRecord *ent.File) error {
	// Use existing CAF function resolution
	functions, err := resolvers.ResolveFunctions(tree)
	if err != nil {
		return fmt.Errorf("failed to resolve functions: %w", err)
	}

	for _, function := range functions {
		err := fp.persistFunction(function, fileRecord)
		if err != nil {
			return fmt.Errorf("failed to persist function %s: %w", function.FunctionName(), err)
		}
	}

	return nil
}

func (fp *fileProcessor) persistFunction(function *ast.FunctionDeclarationNode, fileRecord *ent.File) error {
	// Extract position information from function name node if available
	var lineNumber, columnNumber int = 1, 1 // Default values
	if functionNameNode := function.GetFunctionNameNode(); functionNameNode != nil {
		position := ast.GetNodePosition(functionNameNode)
		lineNumber = int(position.StartLine)
		columnNumber = int(position.StartColumn)
	}

	// Determine function type and symbol type
	functionType := function.GetFunctionType()
	symbolType := symbol.SymbolTypeFunction

	// Classify based on enhanced function type
	switch functionType {
	case ast.FunctionTypeMethod:
		symbolType = symbol.SymbolTypeMethod
	case ast.FunctionTypeConstructor:
		symbolType = symbol.SymbolTypeMethod // Constructor is a special method
	case ast.FunctionTypeStaticMethod:
		symbolType = symbol.SymbolTypeMethod
	case ast.FunctionTypeFunction, ast.FunctionTypeAsync, ast.FunctionTypeArrow:
		symbolType = symbol.SymbolTypeFunction
	}

	// Create enhanced qualified name
	qualifiedName := fp.buildQualifiedFunctionName(function, fileRecord)

	// Create function symbol with enhanced metadata
	functionBuilder := fp.db.Symbol.Create().
		SetName(function.FunctionName()).
		SetQualifiedName(qualifiedName).
		SetSymbolType(symbolType).
		SetScopeType(symbol.ScopeType(fp.determineFunctionScope(function))).
		SetLineNumber(lineNumber).
		SetColumnNumber(columnNumber).
		SetFileID(fileRecord.ID)

	// Set access modifier if available (enhanced)
	accessMod := function.GetAccessModifier()
	if accessMod != "" && accessMod != ast.AccessModifierUnknown {
		// Convert AST access modifier to database enum
		dbAccessMod := fp.convertAccessModifier(accessMod)
		if dbAccessMod != "" {
			functionBuilder = functionBuilder.SetAccessModifier(symbol.AccessModifier(dbAccessMod))
		}
	}

	// Set function flags
	if function.IsAsync() {
		functionBuilder = functionBuilder.SetIsAsync(true)
	}
	if function.IsStatic() {
		functionBuilder = functionBuilder.SetIsStatic(true)
	}
	if function.IsAbstract() {
		functionBuilder = functionBuilder.SetIsAbstract(true)
	}

	// Add comprehensive function metadata
	metadata := map[string]interface{}{
		"parameter_count":   len(function.Parameters()),
		"has_return_type":   function.ReturnType() != "",
		"is_async":          function.IsAsync(),
		"is_static":         function.IsStatic(),
		"is_abstract":       function.IsAbstract(),
		"is_constructor":    function.IsConstructor(),
		"is_method":         function.IsMethod(),
		"function_type":     string(function.GetFunctionType()),
		"has_decorators":    function.HasDecorators(),
		"decorator_count":   len(function.Decorators()),
		"parent_class":      function.GetParentClassName(),
		"parameters":        function.Parameters(),
		"return_type":       function.ReturnType(),
		"decorators":        function.Decorators(),
	}

	functionBuilder = functionBuilder.SetMetadata(metadata)

	_, err := functionBuilder.Save(fp.ctx)
	return err
}

func (fp *fileProcessor) extractInheritanceRelationships(tree core.ParseTree, resolvers core.ObjectOrientedLanguageResolvers, fileRecord *ent.File) error {
	// Use existing CAF inheritance resolution
	inheritanceGraph, err := resolvers.ResolveInheritance(tree)
	if err != nil {
		return fmt.Errorf("failed to resolve inheritance: %w", err)
	}

	// Get all classes in the inheritance graph
	allClasses := inheritanceGraph.GetAllClasses()

	for _, className := range allClasses {
		// Get direct parent relationships
		parentRelationships := inheritanceGraph.GetDirectParents(className)

		for _, relationship := range parentRelationships {
			err := fp.persistInheritanceRelationship(relationship, fileRecord, className)
			if err != nil {
				if fp.scanner.config.Verbose {
					fmt.Printf("Warning: failed to persist inheritance relationship: %v\n", err)
				}
			}
		}
	}

	return nil
}

func (fp *fileProcessor) persistInheritanceRelationship(relationship *ast.InheritanceRelationship, fileRecord *ent.File, childClassName string) error {
	// Create inheritance relationship record with full CAF data
	inhBuilder := fp.db.InheritanceRelationship.Create().
		SetRelationshipType(inheritancerelationship.RelationshipType(relationship.RelationshipType)).
		SetLineNumber(int(relationship.LineNumber)).
		SetIsDirectInheritance(relationship.IsDirectInheritance).
		SetInheritanceDepth(relationship.InheritanceDepth).
		SetFileID(fileRecord.ID)

	// Set module name if available
	if relationship.ModuleName != "" {
		inhBuilder = inhBuilder.SetModuleName(relationship.ModuleName)
	}

	// TODO: Implement cross-file symbol linking in Phase 2
	// Note: ENT schema doesn't support metadata for InheritanceRelationship
	// Store relationship information using the built-in fields

	_, err := inhBuilder.Save(fp.ctx)
	if err != nil {
		return fmt.Errorf("failed to save inheritance relationship %s -> %s: %w", 
			relationship.ChildClassName, relationship.ParentClassName, err)
	}

	return nil
}
