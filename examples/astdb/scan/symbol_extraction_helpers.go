package scan

import (
	"fmt"

	"github.com/safedep/code/core/ast"
	"github.com/safedep/code/examples/astdb/ent"
)

// Helper methods for enhanced symbol extraction

// buildQualifiedClassName constructs a fully qualified class name
func (fp *fileProcessor) buildQualifiedClassName(class *ast.ClassDeclarationNode, fileRecord *ent.File) string {
	// Basic implementation - in a full system this would consider:
	// - Package/module names
	// - Nested class hierarchies
	// - Language-specific namespace rules
	baseName := class.ClassName()
	if baseName == "" {
		return "<anonymous_class>"
	}

	// For now, use simple file-based qualification
	// TODO: Enhance with proper module/package detection
	return baseName
}

// determineClassScope determines the scope type for a class
func (fp *fileProcessor) determineClassScope(class *ast.ClassDeclarationNode) string {
	// Enhanced scope detection based on class characteristics
	// In most languages, classes are global scope unless nested
	// TODO: Detect nested classes and adjust scope accordingly
	return ScopeTypeGlobal
}

// buildQualifiedFunctionName constructs a fully qualified function name
func (fp *fileProcessor) buildQualifiedFunctionName(function *ast.FunctionDeclarationNode, fileRecord *ent.File) string {
	// Enhanced qualified name building
	baseName := function.FunctionName()
	if baseName == "" {
		return "<anonymous_function>"
	}

	// Include parent class if it's a method
	parentClassName := function.GetParentClassName()
	if parentClassName != "" {
		return fmt.Sprintf("%s.%s", parentClassName, baseName)
	}

	// For standalone functions, could include module/package name
	// TODO: Add module-level qualification
	return baseName
}

// determineFunctionScope determines the scope type for a function
func (fp *fileProcessor) determineFunctionScope(function *ast.FunctionDeclarationNode) string {
	// Enhanced scope determination based on function type
	if function.IsMethod() {
		return ScopeTypeClass
	}
	return ScopeTypeGlobal
}

// convertAccessModifier converts AST access modifier to database enum
func (fp *fileProcessor) convertAccessModifier(astAccessMod ast.AccessModifier) string {
	switch astAccessMod {
	case ast.AccessModifierPublic:
		return AccessModifierPublic
	case ast.AccessModifierPrivate:
		return AccessModifierPrivate
	case ast.AccessModifierProtected:
		return AccessModifierProtected
	case ast.AccessModifierPackage:
		return AccessModifierPackage
	default:
		return ""
	}
}