package scan

import (
	"fmt"

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

	// Persist each import statement
	for _, importNode := range imports {
		err := fp.persistImport(importNode, fileRecord)
		if err != nil {
			if fp.scanner.config.Verbose {
				fmt.Printf("Warning: failed to persist import %s: %v\n", importNode.ModuleName(), err)
			}
		}
	}

	return nil
}

func (fp *fileProcessor) persistImport(importNode *ast.ImportNode, fileRecord *ent.File) error {
	// For now, we'll use a simplified approach since we can't directly access node positions
	// In a full implementation, this would extract position from the import node
	
	// Create import statement record
	importBuilder := fp.db.ImportStatement.Create().
		SetModuleName(importNode.ModuleName()).
		SetLineNumber(1). // Placeholder - would need actual line extraction
		SetFileID(fileRecord.ID)

	// Set import alias if available
	if importNode.ModuleAlias() != "" {
		importBuilder = importBuilder.SetImportAlias(importNode.ModuleAlias())
	}

	// Determine import type based on CAF import node
	importType := fp.getImportType(importNode)
	importBuilder = importBuilder.SetImportType(importstatement.ImportType(importType))

	// Set wildcard import flag (ImportNode has IsWildcardImport method)
	if importNode.IsWildcardImport() {
		importBuilder = importBuilder.SetIsDynamic(false) // This is wildcard, not dynamic
	}

	// Get imported names if this is a named import
	importedNames := fp.getImportedNames(importNode)
	if len(importedNames) > 0 {
		importBuilder = importBuilder.SetImportedNames(importedNames)
	}

	// Save the import statement
	_, err := importBuilder.Save(fp.ctx)
	return err
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

func (fp *fileProcessor) getImportedNames(importNode *ast.ImportNode) []string {
	// Extract specific names being imported in named imports
	// This would need language-specific parsing logic
	// For example, in Python: "from module import func1, func2, Class1"
	// For now, return empty slice as placeholder
	
	// In a full implementation, this would:
	// 1. Parse the import statement syntax
	// 2. Extract individual imported identifiers
	// 3. Handle aliased imports properly
	
	return []string{} // Placeholder
}