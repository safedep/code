package depsusage

import (
	"fmt"
)

// identifierItem represents an item from module for usage evidence
type identifierItem struct {
	Module      string
	Item        string
	Alias       string
	Identifier  string
	PackageHint string
}

func newIdentifierItem(module string, item string, alias string, identifier string, packageHint string) *identifierItem {
	return &identifierItem{
		Module:      module,
		Item:        item,
		Alias:       alias,
		Identifier:  identifier,
		PackageHint: packageHint,
	}
}

func (i *identifierItem) String() string {
	return fmt.Sprintf("IdentifierItem: Module: %s, Identifier: %s, Alias: %s, ItemName: %s", i.Module, i.Identifier, i.Alias, i.Item)
}

// UsageEvidence represents the evidence of usage of a module item in a file
type UsageEvidence struct {
	PackageHint string // PackageHint: A hint of what could be the package containing this module

	// ModuleName: The module name taken directly from the ImportNode
	ModuleName string

	// The imported item name taken directly from the ImportNode
	ModuleItem string

	// The import alias name taken directly from the ImportNode
	ModuleAlias string

	// Whether the usage is a wildcard usage
	IsWildCardUsage bool

	// The identifier which led to this usage evidence
	Identifier string

	// File path where the usage was found
	FilePath string

	// Line number where the usage was found
	Line uint

	// Evidence snippet
	EvidenceSnippet string
}

func newUsageEvidence(packageHint string, module string, itemName string, alias string, isWildCardUsage bool, identifier string, filePath string, line uint, evidenceSnippet string) *UsageEvidence {
	return &UsageEvidence{
		PackageHint:     packageHint,
		ModuleName:      module,
		ModuleItem:      itemName,
		ModuleAlias:     alias,
		IsWildCardUsage: isWildCardUsage,
		Identifier:      identifier,
		FilePath:        filePath,
		Line:            line,
		EvidenceSnippet: evidenceSnippet,
	}
}

func (e *UsageEvidence) String() string {
	if e.IsWildCardUsage {
		return fmt.Sprintf("UsageEvidence (WildCardUsage) - PackageHint: %s, Module: %s, ModuleItem: %s, Alias: %s, Identifier: %s, FilePath: %s, Line: %d, EvidenceSnippet: %s", e.PackageHint, e.ModuleName, e.ModuleItem, e.ModuleAlias, e.Identifier, e.FilePath, e.Line, e.EvidenceSnippet)
	}
	return fmt.Sprintf("UsageEvidence - PackageHint: %s, Module: %s, ModuleItem: %s, Alias: %s, Identifier: %s, FilePath: %s, Line: %d, EvidenceSnippet: %s", e.PackageHint, e.ModuleName, e.ModuleItem, e.ModuleAlias, e.Identifier, e.FilePath, e.Line, e.EvidenceSnippet)
}
