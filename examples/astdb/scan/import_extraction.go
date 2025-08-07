package scan

import (
	"fmt"
	"strings"

	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
	"github.com/safedep/code/examples/astdb/ent"
	"github.com/safedep/code/examples/astdb/ent/importstatement"
)

func (fp *fileProcessor) extractAndPersistImports(tree core.ParseTree, fileRecord *ent.File) error {
	// Get language and resolvers
	language, err := tree.Language()
	if err != nil {
		return fmt.Errorf("failed to get language: %w", err)
	}
	
	resolvers := language.Resolvers()
	if resolvers == nil {
		return nil // No resolvers available for this language
	}

	// Use existing CAF import resolution
	imports, err := resolvers.ResolveImports(tree)
	if err != nil {
		return fmt.Errorf("failed to resolve imports: %w", err)
	}

	// Persist each import statement with enhanced symbol linking
	for _, importNode := range imports {
		err := fp.persistImportWithSymbolLinking(importNode, fileRecord)
		if err != nil {
			if fp.scanner.config.Verbose {
				fmt.Printf("Warning: failed to persist import %s: %v\n", importNode.ModuleName(), err)
			}
		}
	}

	return nil
}

func (fp *fileProcessor) persistImportWithSymbolLinking(importNode *ast.ImportNode, fileRecord *ent.File) error {
	// Extract line number from import node if available
	lineNumber := 1 // Default
	// TODO: Extract actual line number from tree-sitter node
	
	// Create import statement record
	importBuilder := fp.db.ImportStatement.Create().
		SetModuleName(importNode.ModuleName()).
		SetLineNumber(lineNumber).
		SetFileID(fileRecord.ID)

	// Set import alias if available
	if importNode.ModuleAlias() != "" {
		importBuilder = importBuilder.SetImportAlias(importNode.ModuleAlias())
	}

	// Determine import type based on CAF import node
	importType := fp.getImportType(importNode)
	importBuilder = importBuilder.SetImportType(importstatement.ImportType(importType))

	// Dynamic import flag - not available in current ImportNode implementation
	// TODO: Implement dynamic import detection

	// Get imported names using enhanced extraction
	importedNames := fp.getImportedNamesEnhanced(importNode)
	if len(importedNames) > 0 {
		importBuilder = importBuilder.SetImportedNames(importedNames)
	}

	// Save the import statement
	importRecord, err := importBuilder.Save(fp.ctx)
	if err != nil {
		return fmt.Errorf("failed to save import statement: %w", err)
	}

	// Enhanced: Attempt to link to imported symbols
	err = fp.linkImportToSymbols(importRecord, importNode, fileRecord)
	if err != nil {
		// Log but don't fail - symbol linking is best-effort
		if fp.scanner.config.Verbose {
			fmt.Printf("Warning: failed to link import to symbols: %v\n", err)
		}
	}

	return nil
}

func (fp *fileProcessor) getImportType(importNode *ast.ImportNode) string {
	// Determine import type based on import characteristics
	// This is a simplified classification - a full implementation would
	// analyze the specific import syntax for each language
	
	if importNode.ModuleAlias() != "" {
		return ImportTypeNamed // Has an alias, likely a named import
	}
	
	// Check if it's a wildcard import
	if importNode.IsWildcardImport() {
		return ImportTypeWildcard
	}
	
	// Check if it's a namespace import
	if fp.isNamespaceImport(importNode) {
		return ImportTypeNamespace
	}
	
	return ImportTypeDefault // Default fallback
}

func (fp *fileProcessor) isWildcardImport(importNode *ast.ImportNode) bool {
	// This would need to be implemented with language-specific logic
	// For example, in Python: "from module import *"
	// In Java: "import package.*"
	// For now, return false as placeholder
	return false
}

func (fp *fileProcessor) isNamespaceImport(importNode *ast.ImportNode) bool {
	// This would need to be implemented with language-specific logic
	// For example, in Python: "import module"
	// In JavaScript: "import * as namespace from 'module'"
	// For now, return false as placeholder
	return false
}

func (fp *fileProcessor) getImportedNamesEnhanced(importNode *ast.ImportNode) []string {
	// Extract names from available ImportNode methods
	
	// Check if there's a module item (specific imported item)
	if importNode.ModuleItem() != "" {
		return []string{importNode.ModuleItem()}
	}

	// Fallback: try to extract names from module name and alias
	if importNode.ModuleAlias() != "" {
		// If there's an alias, the imported name is likely the module itself
		return []string{importNode.ModuleAlias()}
	}

	// For wildcard imports, indicate wildcard
	if importNode.IsWildcardImport() {
		return []string{"*"}
	}

	// For simple module imports, the imported name is the module
	moduleParts := fp.splitModuleName(importNode.ModuleName())
	if len(moduleParts) > 0 {
		return []string{moduleParts[len(moduleParts)-1]} // Last part of module name
	}

	return []string{}
}

func (fp *fileProcessor) getImportedNames(importNode *ast.ImportNode) []string {
	// Deprecated: use getImportedNamesEnhanced instead
	return fp.getImportedNamesEnhanced(importNode)
}

// linkImportToSymbols attempts to link import statements to actual symbols
func (fp *fileProcessor) linkImportToSymbols(importRecord *ent.ImportStatement, importNode *ast.ImportNode, fileRecord *ent.File) error {
	if fp.symbolRegistry == nil {
		return nil // No symbol registry available
	}

	// Try to resolve imported symbols
	importedNames := fp.getImportedNamesEnhanced(importNode)
	
	for _, importedName := range importedNames {
		if importedName == "*" {
			continue // Skip wildcard imports
		}
		
		// Build qualified name for imported symbol
		qualifiedName := fp.buildImportedSymbolName(importedName, importNode.ModuleName(), fileRecord)
		
		// Try multiple resolution strategies
		symbol := fp.tryResolveImportedSymbol(qualifiedName, importedName, importNode.ModuleName())
		
		if symbol != nil {
			// Link import to symbol
			_, err := fp.db.ImportStatement.UpdateOneID(importRecord.ID).
				SetImportedSymbolID(symbol.ID).
				Save(fp.ctx)
			if err != nil {
				return fmt.Errorf("failed to link import to symbol: %w", err)
			}
			break // Successfully linked, stop trying other patterns
		}
	}

	return nil
}

// buildImportedSymbolName builds a qualified name for an imported symbol
func (fp *fileProcessor) buildImportedSymbolName(importedName, moduleName string, fileRecord *ent.File) string {
	if moduleName == "" {
		return importedName
	}

	// Language-specific qualified name building
	language := fp.detectLanguageFromFileRecord(fileRecord)
	switch language {
	case "python":
		return fmt.Sprintf("%s.%s", moduleName, importedName)
	case "java":
		return fmt.Sprintf("%s.%s", moduleName, importedName)
	case "javascript", "typescript":
		// JavaScript modules might not follow the same pattern
		return importedName
	case "go":
		// Go imports are package-based
		return fmt.Sprintf("%s.%s", fp.extractGoPackageName(moduleName), importedName)
	default:
		return fmt.Sprintf("%s.%s", moduleName, importedName)
	}
}

// tryResolveImportedSymbol attempts multiple strategies to resolve an imported symbol
func (fp *fileProcessor) tryResolveImportedSymbol(qualifiedName, importedName, moduleName string) *ent.Symbol {
	// Strategy 1: Exact qualified name match
	if symbol, err := fp.symbolRegistry.ResolveSymbol(qualifiedName); err == nil {
		return symbol
	}

	// Strategy 2: Simple name match (for local symbols)
	if symbol, err := fp.symbolRegistry.ResolveSymbol(importedName); err == nil {
		return symbol
	}

	// Strategy 3: Module name + imported name variations
	variations := fp.generateImportNameVariations(importedName, moduleName)
	for _, variation := range variations {
		if symbol, err := fp.symbolRegistry.ResolveSymbol(variation); err == nil {
			return symbol
		}
	}

	return nil
}

// generateImportNameVariations generates variations of imported symbol names for resolution
func (fp *fileProcessor) generateImportNameVariations(importedName, moduleName string) []string {
	variations := make([]string, 0, 5)
	
	// Add variations with different module formats
	if moduleName != "" {
		moduleParts := fp.splitModuleName(moduleName)
		
		// Try with just the last part of module name
		if len(moduleParts) > 0 {
			lastPart := moduleParts[len(moduleParts)-1]
			variations = append(variations, fmt.Sprintf("%s.%s", lastPart, importedName))
		}
		
		// Try with each level of module hierarchy
		for i := len(moduleParts) - 1; i >= 0; i-- {
			modulePrefix := strings.Join(moduleParts[i:], ".")
			variations = append(variations, fmt.Sprintf("%s.%s", modulePrefix, importedName))
		}
	}
	
	return variations
}

// splitModuleName splits a module name into parts
func (fp *fileProcessor) splitModuleName(moduleName string) []string {
	// Split on common separators
	parts := strings.FieldsFunc(moduleName, func(r rune) bool {
		return r == '.' || r == '/' || r == '\\'
	})
	
	// Filter out empty parts
	filteredParts := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			filteredParts = append(filteredParts, strings.TrimSpace(part))
		}
	}
	
	return filteredParts
}

// extractGoPackageName extracts package name from Go import path
func (fp *fileProcessor) extractGoPackageName(importPath string) string {
	// Go import paths are typically URLs or relative paths
	// Extract the last component as the package name
	parts := fp.splitModuleName(importPath)
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return importPath
}