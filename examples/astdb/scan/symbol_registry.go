package scan

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/safedep/code/core/ast"
	"github.com/safedep/code/examples/astdb/ent"
	"github.com/safedep/code/examples/astdb/ent/file"
	"github.com/safedep/code/examples/astdb/ent/project"
	"github.com/safedep/code/examples/astdb/ent/symbol"
)

// SymbolRegistry manages cross-file symbol resolution for inheritance relationships
type SymbolRegistry struct {
	db        *ent.Client
	ctx       context.Context
	projectID int

	// Thread-safe maps for symbol tracking
	mu             sync.RWMutex
	symbolsByQName map[string]*ent.Symbol
	symbolsByID    map[int]*ent.Symbol
	pendingLinks   []PendingInheritanceLink

	// Caches for performance
	qualifiedNameCache map[string]string
	moduleNameCache    map[string]string
}

// PendingInheritanceLink represents an inheritance relationship waiting for symbol resolution
type PendingInheritanceLink struct {
	ChildSymbolID       int
	ChildQualifiedName  string
	ParentQualifiedName string
	Relationship        *ast.InheritanceRelationship
	FileRecord          *ent.File
	InheritanceRecordID int
}

// NewSymbolRegistry creates a new symbol registry for cross-file symbol resolution
func NewSymbolRegistry(db *ent.Client, ctx context.Context, projectID int) *SymbolRegistry {
	return &SymbolRegistry{
		db:                 db,
		ctx:                ctx,
		projectID:          projectID,
		symbolsByQName:     make(map[string]*ent.Symbol),
		symbolsByID:        make(map[int]*ent.Symbol),
		pendingLinks:       make([]PendingInheritanceLink, 0),
		qualifiedNameCache: make(map[string]string),
		moduleNameCache:    make(map[string]string),
	}
}

// RegisterSymbol adds a symbol to the registry for cross-file resolution
func (sr *SymbolRegistry) RegisterSymbol(symbol *ent.Symbol) error {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	if symbol == nil {
		return fmt.Errorf("cannot register nil symbol")
	}

	sr.symbolsByQName[symbol.QualifiedName] = symbol
	sr.symbolsByID[symbol.ID] = symbol

	return nil
}

// ResolveSymbol attempts to find a symbol by its qualified name
func (sr *SymbolRegistry) ResolveSymbol(qualifiedName string) (*ent.Symbol, error) {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	if symbol, exists := sr.symbolsByQName[qualifiedName]; exists {
		return symbol, nil
	}

	return nil, fmt.Errorf("symbol not found: %s", qualifiedName)
}

// ResolveSymbolByID retrieves a symbol by its database ID
func (sr *SymbolRegistry) ResolveSymbolByID(id int) (*ent.Symbol, error) {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	if symbol, exists := sr.symbolsByID[id]; exists {
		return symbol, nil
	}

	return nil, fmt.Errorf("symbol not found by ID: %d", id)
}

// AddPendingInheritance adds an inheritance link that needs symbol resolution
func (sr *SymbolRegistry) AddPendingInheritance(link PendingInheritanceLink) error {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	sr.pendingLinks = append(sr.pendingLinks, link)
	return nil
}

// ResolvePendingInheritance processes all pending inheritance links
func (sr *SymbolRegistry) ResolvePendingInheritance() error {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	resolved := 0
	unresolved := 0

	for _, link := range sr.pendingLinks {
		err := sr.resolveSingleInheritanceLink(link)
		if err == nil {
			resolved++
		} else {
			unresolved++
			// Log but don't fail - some symbols may be from external dependencies
		}
	}

	// Clear pending links after processing
	sr.pendingLinks = make([]PendingInheritanceLink, 0)

	if resolved > 0 || unresolved > 0 {
		fmt.Printf("Resolved inheritance links: %d successful, %d unresolved\n", resolved, unresolved)
	}

	return nil
}

// resolveSingleInheritanceLink attempts to resolve one inheritance relationship
func (sr *SymbolRegistry) resolveSingleInheritanceLink(link PendingInheritanceLink) error {
	// Try to find child symbol
	childSymbol, exists := sr.symbolsByID[link.ChildSymbolID]
	if !exists {
		return fmt.Errorf("child symbol not found: ID %d", link.ChildSymbolID)
	}

	// Try to find parent symbol with multiple qualified name patterns
	var parentSymbol *ent.Symbol
	var err error

	// Try exact qualified name first
	parentSymbol, err = sr.tryResolveParentSymbol(link.ParentQualifiedName)
	if err != nil {
		// Try with child's module prefix
		childModuleName := sr.extractModuleFromQualifiedName(childSymbol.QualifiedName)
		if childModuleName != "" {
			parentWithModule := fmt.Sprintf("%s.%s", childModuleName, link.ParentQualifiedName)
			parentSymbol, err = sr.tryResolveParentSymbol(parentWithModule)
		}

		if err != nil {
			// Try simple class name matching
			parentSymbol, err = sr.tryResolveParentBySimpleName(link.ParentQualifiedName)
		}
	}

	if err != nil {
		return fmt.Errorf("parent symbol not found: %s", link.ParentQualifiedName)
	}

	// Update inheritance relationship with symbol links
	_, updateErr := sr.db.InheritanceRelationship.UpdateOneID(link.InheritanceRecordID).
		SetChildID(childSymbol.ID).
		SetParentID(parentSymbol.ID).
		Save(sr.ctx)

	if updateErr != nil {
		return fmt.Errorf("failed to update inheritance relationship: %w", updateErr)
	}

	return nil
}

// tryResolveParentSymbol attempts to find parent symbol by qualified name
func (sr *SymbolRegistry) tryResolveParentSymbol(qualifiedName string) (*ent.Symbol, error) {
	if symbol, exists := sr.symbolsByQName[qualifiedName]; exists {
		return symbol, nil
	}

	return nil, fmt.Errorf("parent symbol not found: %s", qualifiedName)
}

// tryResolveParentBySimpleName attempts fuzzy matching by simple class name
func (sr *SymbolRegistry) tryResolveParentBySimpleName(className string) (*ent.Symbol, error) {
	for qualifiedName, symbol := range sr.symbolsByQName {
		if string(symbol.SymbolType) == SymbolTypeClass {
			// Extract simple name from qualified name
			parts := strings.Split(qualifiedName, ".")
			simpleName := parts[len(parts)-1]
			if simpleName == className {
				return symbol, nil
			}
		}
	}

	return nil, fmt.Errorf("parent symbol not found by simple name: %s", className)
}

// BuildQualifiedName constructs a language-appropriate qualified name
func (sr *SymbolRegistry) BuildQualifiedName(symbolName, moduleName, filePath string) string {
	if symbolName == "" {
		return "<anonymous>"
	}

	// Check cache first
	cacheKey := fmt.Sprintf("%s|%s|%s", symbolName, moduleName, filePath)
	sr.mu.RLock()
	if cached, exists := sr.qualifiedNameCache[cacheKey]; exists {
		sr.mu.RUnlock()
		return cached
	}
	sr.mu.RUnlock()

	// Build qualified name based on language
	var qualifiedName string

	// Detect language from file extension
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".py":
		qualifiedName = sr.buildPythonQualifiedName(symbolName, moduleName, filePath)
	case ".java":
		qualifiedName = sr.buildJavaQualifiedName(symbolName, moduleName, filePath)
	case ".js", ".ts", ".tsx", ".jsx":
		qualifiedName = sr.buildJavaScriptQualifiedName(symbolName, moduleName, filePath)
	case ".go":
		qualifiedName = sr.buildGoQualifiedName(symbolName, moduleName, filePath)
	default:
		// Default to simple name if language not recognized
		if moduleName != "" {
			qualifiedName = fmt.Sprintf("%s.%s", moduleName, symbolName)
		} else {
			qualifiedName = symbolName
		}
	}

	// Cache the result
	sr.mu.Lock()
	sr.qualifiedNameCache[cacheKey] = qualifiedName
	sr.mu.Unlock()

	return qualifiedName
}

// buildPythonQualifiedName creates Python-style qualified names
func (sr *SymbolRegistry) buildPythonQualifiedName(symbolName, moduleName, filePath string) string {
	if moduleName != "" {
		return fmt.Sprintf("%s.%s", moduleName, symbolName)
	}

	// Convert file path to Python module notation
	// e.g., src/package/module.py -> package.module.ClassName
	moduleFromPath := sr.convertPathToPythonModule(filePath)
	if moduleFromPath != "" {
		return fmt.Sprintf("%s.%s", moduleFromPath, symbolName)
	}

	return symbolName
}

// buildJavaQualifiedName creates Java-style qualified names
func (sr *SymbolRegistry) buildJavaQualifiedName(symbolName, moduleName, filePath string) string {
	if moduleName != "" {
		return fmt.Sprintf("%s.%s", moduleName, symbolName)
	}

	// Extract package name from file content or path
	packageName := sr.extractJavaPackageName(filePath)
	if packageName != "" {
		return fmt.Sprintf("%s.%s", packageName, symbolName)
	}

	return symbolName
}

// buildJavaScriptQualifiedName creates JavaScript/TypeScript-style qualified names
func (sr *SymbolRegistry) buildJavaScriptQualifiedName(symbolName, moduleName, filePath string) string {
	if moduleName != "" {
		return fmt.Sprintf("%s.%s", moduleName, symbolName)
	}

	// Use file-based qualification for JavaScript
	fileBaseName := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	if fileBaseName != "" && fileBaseName != symbolName {
		return fmt.Sprintf("%s.%s", fileBaseName, symbolName)
	}

	return symbolName
}

// buildGoQualifiedName creates Go-style qualified names
func (sr *SymbolRegistry) buildGoQualifiedName(symbolName, moduleName, filePath string) string {
	if moduleName != "" {
		return fmt.Sprintf("%s.%s", moduleName, symbolName)
	}

	// Use package name from directory
	packageName := filepath.Base(filepath.Dir(filePath))
	if packageName != "" && packageName != "." {
		return fmt.Sprintf("%s.%s", packageName, symbolName)
	}

	return symbolName
}

// convertPathToPythonModule converts file path to Python module notation
func (sr *SymbolRegistry) convertPathToPythonModule(filePath string) string {
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
func (sr *SymbolRegistry) extractJavaPackageName(filePath string) string {
	// Check cache first
	sr.mu.RLock()
	if cached, exists := sr.moduleNameCache[filePath]; exists {
		sr.mu.RUnlock()
		return cached
	}
	sr.mu.RUnlock()

	// This is a simplified implementation
	// In a full implementation, would parse the Java file for package declaration
	dirPath := filepath.Dir(filePath)
	packageName := filepath.Base(dirPath)

	// Cache the result
	sr.mu.Lock()
	sr.moduleNameCache[filePath] = packageName
	sr.mu.Unlock()

	return packageName
}

// extractModuleFromQualifiedName extracts module name from a qualified name
func (sr *SymbolRegistry) extractModuleFromQualifiedName(qualifiedName string) string {
	parts := strings.Split(qualifiedName, ".")
	if len(parts) > 1 {
		return strings.Join(parts[:len(parts)-1], ".")
	}

	return ""
}

// LoadExistingSymbols loads all symbols from the database for the current project
func (sr *SymbolRegistry) LoadExistingSymbols() error {
	symbols, err := sr.db.Symbol.Query().
		Where(symbol.HasFileWith(file.HasProjectWith(project.ID(sr.projectID)))).
		All(sr.ctx)

	if err != nil {
		return fmt.Errorf("failed to load existing symbols: %w", err)
	}

	sr.mu.Lock()
	defer sr.mu.Unlock()

	for _, sym := range symbols {
		sr.symbolsByQName[sym.QualifiedName] = sym
		sr.symbolsByID[sym.ID] = sym
	}

	fmt.Printf("Loaded %d existing symbols into registry\n", len(symbols))
	return nil
}

type symbolRegistryStats struct {
	TotalSymbols       int `json:"total_symbols"`
	PendingLinks       int `json:"pending_links"`
	CacheEntries       int `json:"cache_entries"`
	ModuleCacheEntries int `json:"module_cache_entries"`
}

// GetStatistics returns registry statistics
func (sr *SymbolRegistry) GetStatistics() symbolRegistryStats {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	return symbolRegistryStats{
		TotalSymbols:       len(sr.symbolsByQName),
		PendingLinks:       len(sr.pendingLinks),
		CacheEntries:       len(sr.qualifiedNameCache),
		ModuleCacheEntries: len(sr.moduleNameCache),
	}
}
