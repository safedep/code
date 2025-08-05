package ast

import (
	"fmt"
	"slices"
	"strings"
)

// RelationshipType represents the type of inheritance relationship
type RelationshipType string

const (
	// RelationshipTypeExtends represents single inheritance (Go, Java single inheritance)
	RelationshipTypeExtends RelationshipType = "extends"

	// RelationshipTypeImplements represents interface implementation (Go, Java)
	RelationshipTypeImplements RelationshipType = "implements"

	// RelationshipTypeInherits represents general inheritance (Python multiple inheritance)
	RelationshipTypeInherits RelationshipType = "inherits"

	// RelationshipTypeMixin represents mixin inheritance patterns
	RelationshipTypeMixin RelationshipType = "mixin"
)

// InheritanceRelationship represents a single parent-child class relationship
type InheritanceRelationship struct {
	// Core relationship information
	ChildClassName   string
	ParentClassName  string
	RelationshipType RelationshipType

	// Source location information
	FileLocation string
	LineNumber   uint32

	// Relationship metadata
	IsDirectInheritance bool // true for immediate parent, false for computed ancestor
	InheritanceDepth    int  // 1 for direct parent, 2 for grandparent, etc.

	// Additional context
	ModuleName string // Module/package where relationship is declared
}

// NewInheritanceRelationship creates a new direct inheritance relationship
func NewInheritanceRelationship(child, parent string, relType RelationshipType, file string, line uint32) *InheritanceRelationship {
	return &InheritanceRelationship{
		ChildClassName:      child,
		ParentClassName:     parent,
		RelationshipType:    relType,
		FileLocation:        file,
		LineNumber:          line,
		IsDirectInheritance: true,
		InheritanceDepth:    1,
	}
}

// String returns a string representation of the inheritance relationship
func (ir *InheritanceRelationship) String() string {
	directStr := ""
	if !ir.IsDirectInheritance {
		directStr = fmt.Sprintf(" (indirect, depth: %d)", ir.InheritanceDepth)
	}

	location := ""
	if ir.FileLocation != "" {
		location = fmt.Sprintf(" at %s:%d", ir.FileLocation, ir.LineNumber)
	}

	return fmt.Sprintf("%s %s %s%s%s", ir.ChildClassName, ir.RelationshipType, ir.ParentClassName, directStr, location)
}

// InheritanceGraph manages the complete inheritance hierarchy for a codebase
type InheritanceGraph struct {
	// Map from child class name to its direct parent relationships
	directParents map[string][]*InheritanceRelationship

	// Map from parent class name to its direct child relationships
	directChildren map[string][]*InheritanceRelationship

	// Cache for expensive computations
	ancestryCache   map[string][]string
	descendantCache map[string][]string
	mroCache        map[string][]string // Method Resolution Order cache

	// Track all known classes
	allClasses map[string]bool
}

// NewInheritanceGraph creates a new empty inheritance graph
func NewInheritanceGraph() *InheritanceGraph {
	return &InheritanceGraph{
		directParents:   make(map[string][]*InheritanceRelationship),
		directChildren:  make(map[string][]*InheritanceRelationship),
		ancestryCache:   make(map[string][]string),
		descendantCache: make(map[string][]string),
		mroCache:        make(map[string][]string),
		allClasses:      make(map[string]bool),
	}
}

// AddRelationship adds a direct inheritance relationship to the graph
func (ig *InheritanceGraph) AddRelationship(child, parent string, relType RelationshipType, file string, line uint32) *InheritanceRelationship {
	relationship := NewInheritanceRelationship(child, parent, relType, file, line)

	// Add to direct parents map
	ig.directParents[child] = append(ig.directParents[child], relationship)

	// Add to direct children map
	ig.directChildren[parent] = append(ig.directChildren[parent], relationship)

	// Track both classes as known
	ig.allClasses[child] = true
	ig.allClasses[parent] = true

	// Clear caches as they're now invalid
	ig.clearCaches()

	return relationship
}

// GetDirectParents returns all direct parent relationships for a class
func (ig *InheritanceGraph) GetDirectParents(className string) []*InheritanceRelationship {
	return ig.directParents[className]
}

// GetDirectChildren returns all direct child relationships for a class
func (ig *InheritanceGraph) GetDirectChildren(className string) []*InheritanceRelationship {
	return ig.directChildren[className]
}

// GetDirectParentNames returns the names of direct parent classes
func (ig *InheritanceGraph) GetDirectParentNames(className string) []string {
	var parents []string
	for _, rel := range ig.directParents[className] {
		parents = append(parents, rel.ParentClassName)
	}
	return parents
}

// GetDirectChildNames returns the names of direct child classes
func (ig *InheritanceGraph) GetDirectChildNames(className string) []string {
	var children []string
	for _, rel := range ig.directChildren[className] {
		children = append(children, rel.ChildClassName)
	}
	return children
}

// GetAncestry returns all ancestor classes (parents, grandparents, etc.) in breadth-first order
func (ig *InheritanceGraph) GetAncestry(className string) []string {
	// Check cache first
	if cached, exists := ig.ancestryCache[className]; exists {
		return cached
	}

	var ancestry []string
	visited := make(map[string]bool)
	queue := []string{className}
	visited[className] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		// Add all direct parents to queue and ancestry
		for _, rel := range ig.directParents[current] {
			parent := rel.ParentClassName
			if !visited[parent] {
				visited[parent] = true
				queue = append(queue, parent)
				ancestry = append(ancestry, parent)
			}
		}
	}

	// Cache the result
	ig.ancestryCache[className] = ancestry
	return ancestry
}

// GetDescendants returns all descendant classes (children, grandchildren, etc.) in breadth-first order
func (ig *InheritanceGraph) GetDescendants(className string) []string {
	// Check cache first
	if cached, exists := ig.descendantCache[className]; exists {
		return cached
	}

	var descendants []string
	visited := make(map[string]bool)
	queue := []string{className}
	visited[className] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		// Add all direct children to queue and descendants
		for _, rel := range ig.directChildren[current] {
			child := rel.ChildClassName
			if !visited[child] {
				visited[child] = true
				queue = append(queue, child)
				descendants = append(descendants, child)
			}
		}
	}

	// Cache the result
	ig.descendantCache[className] = descendants
	return descendants
}

// IsAncestor checks if ancestor is an ancestor of descendant
func (ig *InheritanceGraph) IsAncestor(ancestor, descendant string) bool {
	ancestry := ig.GetAncestry(descendant)
	return slices.Contains(ancestry, ancestor)
}

// IsDescendant checks if descendant is a descendant of ancestor
func (ig *InheritanceGraph) IsDescendant(descendant, ancestor string) bool {
	return ig.IsAncestor(ancestor, descendant)
}

// GetInheritanceDepth returns the minimum depth from child to ancestor
// Returns -1 if no inheritance relationship exists
func (ig *InheritanceGraph) GetInheritanceDepth(child, ancestor string) int {
	if child == ancestor {
		return 0
	}

	visited := make(map[string]bool)
	queue := []struct {
		class string
		depth int
	}{{child, 0}}
	visited[child] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		// Check direct parents
		for _, rel := range ig.directParents[current.class] {
			parent := rel.ParentClassName
			newDepth := current.depth + 1

			if parent == ancestor {
				return newDepth
			}

			if !visited[parent] {
				visited[parent] = true
				queue = append(queue, struct {
					class string
					depth int
				}{parent, newDepth})
			}
		}
	}

	return -1 // No relationship found
}

// GetMethodResolutionOrder calculates the Method Resolution Order using C3 linearization
// This is primarily for Python-style multiple inheritance
func (ig *InheritanceGraph) GetMethodResolutionOrder(className string) []string {
	// Check cache first
	if cached, exists := ig.mroCache[className]; exists {
		return cached
	}

	mro := ig.calculateC3Linearization(className)
	ig.mroCache[className] = mro
	return mro
}

// calculateC3Linearization implements the C3 linearization algorithm
func (ig *InheritanceGraph) calculateC3Linearization(className string) []string {
	// Base case: class with no parents
	directParents := ig.GetDirectParentNames(className)
	if len(directParents) == 0 {
		return []string{className}
	}

	// Get linearizations of all parents
	var parentLinearizations [][]string
	for _, parent := range directParents {
		parentLinearizations = append(parentLinearizations, ig.calculateC3Linearization(parent))
	}

	// Merge step of C3 linearization
	result := []string{className}
	tails := append(parentLinearizations, directParents)

	for {
		// Find next valid candidate
		candidate := ""
		for _, tail := range tails {
			if len(tail) > 0 {
				head := tail[0]
				// Check if head appears in any other tail (not as first element)
				validCandidate := true
				for _, otherTail := range tails {
					if len(otherTail) > 1 && slices.Contains(otherTail[1:], head) {
						validCandidate = false
						break
					}
				}
				if validCandidate {
					candidate = head
					break
				}
			}
		}

		// If no valid candidate found, we have an inconsistent hierarchy
		if candidate == "" {
			break
		}

		// Add candidate to result and remove from all tails
		result = append(result, candidate)
		for i, tail := range tails {
			if len(tail) > 0 && tail[0] == candidate {
				tails[i] = tail[1:]
			}
		}

		// Remove empty tails
		var nonEmptyTails [][]string
		for _, tail := range tails {
			if len(tail) > 0 {
				nonEmptyTails = append(nonEmptyTails, tail)
			}
		}
		tails = nonEmptyTails

		// If all tails are empty, we're done
		if len(tails) == 0 {
			break
		}
	}

	return result
}

// DetectCircularInheritance detects circular inheritance relationships
func (ig *InheritanceGraph) DetectCircularInheritance() [][]string {
	var cycles [][]string
	visited := make(map[string]int) // 0: unvisited, 1: visiting, 2: visited

	var dfs func(string, []string) bool
	dfs = func(class string, path []string) bool {
		if visited[class] == 1 {
			// Found a cycle - extract the cycle from the path
			cycleStart := -1
			for i, c := range path {
				if c == class {
					cycleStart = i
					break
				}
			}
			if cycleStart >= 0 {
				cycle := make([]string, len(path)-cycleStart)
				copy(cycle, path[cycleStart:])
				cycles = append(cycles, cycle)
			}
			return true
		}

		if visited[class] == 2 {
			return false // Already fully processed
		}

		visited[class] = 1 // Mark as visiting
		path = append(path, class)

		// Visit all direct parents
		for _, rel := range ig.directParents[class] {
			if dfs(rel.ParentClassName, path) {
				// Continue searching for more cycles
			}
		}

		visited[class] = 2 // Mark as fully visited
		return false
	}

	// Check all classes for cycles
	for class := range ig.allClasses {
		if visited[class] == 0 {
			dfs(class, []string{})
		}
	}

	return cycles
}

// GetAllClasses returns all classes known to the inheritance graph
func (ig *InheritanceGraph) GetAllClasses() []string {
	var classes []string
	for class := range ig.allClasses {
		classes = append(classes, class)
	}
	return classes
}

// GetRootClasses returns classes that have no parents (root of inheritance hierarchies)
func (ig *InheritanceGraph) GetRootClasses() []string {
	var roots []string
	for class := range ig.allClasses {
		if len(ig.directParents[class]) == 0 {
			roots = append(roots, class)
		}
	}
	return roots
}

// GetLeafClasses returns classes that have no children (leaves of inheritance hierarchies)
func (ig *InheritanceGraph) GetLeafClasses() []string {
	var leaves []string
	for class := range ig.allClasses {
		if len(ig.directChildren[class]) == 0 {
			leaves = append(leaves, class)
		}
	}
	return leaves
}

// MergeFrom merges inheritance information from another graph
func (ig *InheritanceGraph) MergeFrom(other *InheritanceGraph) {
	// Merge all relationships
	for _, relationships := range other.directParents {
		for _, rel := range relationships {
			ig.AddRelationship(rel.ChildClassName, rel.ParentClassName, rel.RelationshipType, rel.FileLocation, rel.LineNumber)
		}
	}
}

// clearCaches clears all internal caches when the graph is modified
func (ig *InheritanceGraph) clearCaches() {
	ig.ancestryCache = make(map[string][]string)
	ig.descendantCache = make(map[string][]string)
	ig.mroCache = make(map[string][]string)
}

// GetStatistics returns statistics about the inheritance graph
func (ig *InheritanceGraph) GetStatistics() map[string]interface{} {
	totalClasses := len(ig.allClasses)
	totalRelationships := 0
	for _, rels := range ig.directParents {
		totalRelationships += len(rels)
	}

	rootClasses := ig.GetRootClasses()
	leafClasses := ig.GetLeafClasses()
	cycles := ig.DetectCircularInheritance()

	// Calculate maximum inheritance depth
	maxDepth := 0
	for class := range ig.allClasses {
		ancestry := ig.GetAncestry(class)
		if len(ancestry) > maxDepth {
			maxDepth = len(ancestry)
		}
	}

	return map[string]interface{}{
		"total_classes":         totalClasses,
		"total_relationships":   totalRelationships,
		"root_classes":          len(rootClasses),
		"leaf_classes":          len(leafClasses),
		"circular_inheritance":  len(cycles),
		"max_inheritance_depth": maxDepth,
	}
}

// String returns a string representation of the inheritance graph
func (ig *InheritanceGraph) String() string {
	var parts []string
	stats := ig.GetStatistics()

	parts = append(parts, fmt.Sprintf("InheritanceGraph{classes: %d, relationships: %d}",
		stats["total_classes"], stats["total_relationships"]))

	if cycles := ig.DetectCircularInheritance(); len(cycles) > 0 {
		parts = append(parts, fmt.Sprintf("WARNING: %d circular inheritance detected", len(cycles)))
	}

	return strings.Join(parts, ", ")
}

// BuildInheritanceGraphFromClasses creates an inheritance graph from ClassDeclarationNode instances
func BuildInheritanceGraphFromClasses(classes []*ClassDeclarationNode, defaultFile string) *InheritanceGraph {
	ig := NewInheritanceGraph()

	for _, class := range classes {
		className := class.ClassName()
		if className == "" {
			continue // Skip classes without names
		}

		// Add inheritance relationships for each base class
		baseClasses := class.BaseClasses()
		for _, baseClass := range baseClasses {
			// Use RelationshipTypeInherits as default for ClassDeclarationNode
			ig.AddRelationship(className, baseClass, RelationshipTypeInherits, defaultFile, 1)
		}

		// Ensure the class is tracked even if it has no inheritance
		if len(baseClasses) == 0 {
			ig.allClasses[className] = true
		}
	}

	return ig
}
