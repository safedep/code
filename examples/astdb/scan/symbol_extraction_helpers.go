package scan

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/safedep/code/core/ast"
	"github.com/safedep/code/examples/astdb/ent"
)

// Helper methods for enhanced symbol extraction

// buildQualifiedClassName constructs a fully qualified class name
func (fp *fileProcessor) buildQualifiedClassName(class *ast.ClassDeclarationNode, fileRecord *ent.File) string {
	baseName := class.ClassName()
	if baseName == "" {
		return "<anonymous_class>"
	}

	// Language-specific qualification
	language := fp.detectLanguageFromFileRecord(fileRecord)
	switch language {
	case "python":
		return fp.buildPythonQualifiedName(baseName, fileRecord)
	case "java":
		return fp.buildJavaQualifiedName(baseName, fileRecord)
	case "javascript", "typescript":
		return fp.buildJavaScriptQualifiedName(baseName, fileRecord)
	case "go":
		return fp.buildGoQualifiedName(baseName, fileRecord)
	default:
		return baseName
	}
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
	baseName := function.FunctionName()
	if baseName == "" {
		return "<anonymous_function>"
	}

	// Include parent class if it's a method
	parentClassName := function.GetParentClassName()
	if parentClassName != "" {
		// Build qualified parent class name first
		language := fp.detectLanguageFromFileRecord(fileRecord)
		var qualifiedParent string
		switch language {
		case "python":
			qualifiedParent = fp.buildPythonQualifiedName(parentClassName, fileRecord)
		case "java":
			qualifiedParent = fp.buildJavaQualifiedName(parentClassName, fileRecord)
		case "javascript", "typescript":
			qualifiedParent = fp.buildJavaScriptQualifiedName(parentClassName, fileRecord)
		case "go":
			qualifiedParent = fp.buildGoQualifiedName(parentClassName, fileRecord)
		default:
			qualifiedParent = parentClassName
		}
		return fmt.Sprintf("%s.%s", qualifiedParent, baseName)
	}

	// For standalone functions, add module-level qualification
	language := fp.detectLanguageFromFileRecord(fileRecord)
	switch language {
	case "python":
		return fp.buildPythonQualifiedName(baseName, fileRecord)
	case "java":
		return fp.buildJavaQualifiedName(baseName, fileRecord)
	case "javascript", "typescript":
		return fp.buildJavaScriptQualifiedName(baseName, fileRecord)
	case "go":
		return fp.buildGoQualifiedName(baseName, fileRecord)
	default:
		return baseName
	}
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

// detectLanguageFromFileRecord detects language from file record
func (fp *fileProcessor) detectLanguageFromFileRecord(fileRecord *ent.File) string {
	return string(fileRecord.Language)
}

// buildPythonQualifiedName creates Python-style qualified names
func (fp *fileProcessor) buildPythonQualifiedName(symbolName string, fileRecord *ent.File) string {
	moduleFromPath := fp.convertPathToPythonModule(fileRecord.RelativePath)
	if moduleFromPath != "" {
		return fmt.Sprintf("%s.%s", moduleFromPath, symbolName)
	}
	return symbolName
}

// buildJavaQualifiedName creates Java-style qualified names
func (fp *fileProcessor) buildJavaQualifiedName(symbolName string, fileRecord *ent.File) string {
	packageName := fp.extractJavaPackageName(fileRecord.AbsolutePath)
	if packageName != "" {
		return fmt.Sprintf("%s.%s", packageName, symbolName)
	}
	return symbolName
}

// buildJavaScriptQualifiedName creates JavaScript/TypeScript-style qualified names
func (fp *fileProcessor) buildJavaScriptQualifiedName(symbolName string, fileRecord *ent.File) string {
	fileBaseName := strings.TrimSuffix(filepath.Base(fileRecord.RelativePath), filepath.Ext(fileRecord.RelativePath))
	if fileBaseName != "" && fileBaseName != symbolName {
		return fmt.Sprintf("%s.%s", fileBaseName, symbolName)
	}
	return symbolName
}

// buildGoQualifiedName creates Go-style qualified names
func (fp *fileProcessor) buildGoQualifiedName(symbolName string, fileRecord *ent.File) string {
	packageName := filepath.Base(filepath.Dir(fileRecord.RelativePath))
	if packageName != "" && packageName != "." {
		return fmt.Sprintf("%s.%s", packageName, symbolName)
	}
	return symbolName
}

// convertPathToPythonModule converts file path to Python module notation
func (fp *fileProcessor) convertPathToPythonModule(filePath string) string {
	// Remove file extension
	pathWithoutExt := strings.TrimSuffix(filePath, filepath.Ext(filePath))
	
	// Convert path separators to dots
	modulePath := strings.ReplaceAll(pathWithoutExt, string(filepath.Separator), ".")
	
	// Remove common prefixes like src, app, etc.
	parts := strings.Split(modulePath, ".")
	filteredParts := make([]string, 0, len(parts))
	
	for _, part := range parts {
		// Skip common directory names
		if part != "" && part != "src" && part != "app" && part != "lib" && part != "__pycache__" {
			filteredParts = append(filteredParts, part)
		}
	}
	
	if len(filteredParts) > 0 {
		return strings.Join(filteredParts, ".")
	}
	
	return ""
}

// extractJavaPackageName extracts package name from Java file
func (fp *fileProcessor) extractJavaPackageName(filePath string) string {
	// This is a simplified implementation
	// In a full implementation, would parse the Java file for package declaration
	dirPath := filepath.Dir(filePath)
	packageName := filepath.Base(dirPath)
	return packageName
}

// buildQualifiedName builds a qualified name for symbols using enhanced language-specific logic
func (fp *fileProcessor) buildQualifiedName(symbolName string, fileRecord *ent.File) string {
	if fp.symbolRegistry != nil {
		// Use the symbol registry's enhanced qualified name building
		return fp.symbolRegistry.BuildQualifiedName(symbolName, "", fileRecord.AbsolutePath)
	}

	// Fallback to simple name if registry not available
	return symbolName
}