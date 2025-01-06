package depsusage

import (
	"fmt"
)

// identifierItem represents an item from module for usage evidence
type identifierItem struct {
	Module     string
	Identifier string
	Alias      string
	ItemName   string
}

func newIdentifierItem(module string, identifier string, alias string, itemName string) *identifierItem {
	return &identifierItem{
		Module:     module,
		Identifier: identifier,
		Alias:      alias,
		ItemName:   itemName,
	}
}

func (i *identifierItem) String() string {
	return fmt.Sprintf("IdentifierItem: Module: %s, Identifier: %s, Alias: %s, ItemName: %s", i.Module, i.Identifier, i.Alias, i.ItemName)
}

// UsageEvidence represents the evidence of usage of a module item in a file
type UsageEvidence struct {
	Module          string
	Identifier      string
	Alias           string
	ItemName        string
	FilePath        string
	Line            uint
	IsWildCardUsage bool
}

const wildcardIdentifier = "*"

func newUsageEvidence(module string, identifier string, alias string, itemName string, filePath string, line uint, isWildCardUsage bool) *UsageEvidence {
	return &UsageEvidence{
		Module:          module,
		Identifier:      identifier,
		Alias:           alias,
		ItemName:        itemName,
		FilePath:        filePath,
		Line:            line,
		IsWildCardUsage: isWildCardUsage,
	}
}

func (e *UsageEvidence) String() string {
	if e.IsWildCardUsage {
		return fmt.Sprintf("UsageEvidence (WildCardUsage) - Module: %s, Identifier: %s, Alias: %s, ItemName: %s, File: %s, Line: %d", e.Module, e.Identifier, e.Alias, e.ItemName, e.FilePath, e.Line)
	}
	return fmt.Sprintf("UsageEvidence - Module: %s, Identifier: %s, Alias: %s, ItemName: %s, File: %s, Line: %d", e.Module, e.Identifier, e.Alias, e.ItemName, e.FilePath, e.Line)
}
