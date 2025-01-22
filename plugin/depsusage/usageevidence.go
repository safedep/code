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
	PackageHint string

	Module          string
	ModuleItem      string
	Alias           string
	IsWildCardUsage bool

	Identifier string
	FilePath   string
	Line       uint
}

const wildcardIdentifier = "*"

func newUsageEvidence(packageHint string, module string, itemName string, alias string, isWildCardUsage bool, identifier string, filePath string, line uint) *UsageEvidence {
	return &UsageEvidence{
		PackageHint:     packageHint,
		Module:          module,
		ModuleItem:      itemName,
		Alias:           alias,
		IsWildCardUsage: isWildCardUsage,
		Identifier:      identifier,
		FilePath:        filePath,
		Line:            line,
	}
}

func (e *UsageEvidence) String() string {
	if e.IsWildCardUsage {
		return fmt.Sprintf("UsageEvidence (WildCardUsage) - PackageHint: %s, Module: %s, ModuleItem: %s, Alias: %s, Identifier: %s, FilePath: %s, Line: %d", e.PackageHint, e.Module, e.ModuleItem, e.Alias, e.Identifier, e.FilePath, e.Line)
	}
	return fmt.Sprintf("UsageEvidence - PackageHint: %s, Module: %s, ModuleItem: %s, Alias: %s, Identifier: %s, FilePath: %s, Line: %d", e.PackageHint, e.Module, e.ModuleItem, e.Alias, e.Identifier, e.FilePath, e.Line)
}
