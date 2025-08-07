package scan

import (
	"context"
	"fmt"

	"github.com/safedep/code/core/ast"
	"github.com/safedep/code/examples/astdb/ent"
	"github.com/safedep/code/examples/astdb/ent/file"
	"github.com/safedep/code/examples/astdb/ent/inheritancerelationship"
	"github.com/safedep/code/examples/astdb/ent/project"
)

// ProjectInheritanceAnalyzer provides project-level inheritance analysis and quality detection
type ProjectInheritanceAnalyzer struct {
	db             *ent.Client
	ctx            context.Context
	globalGraph    *ast.InheritanceGraph
	symbolRegistry *SymbolRegistry
	config         Config
	
	// Cache for performance
	relationshipCache map[int]*ent.InheritanceRelationship
	symbolCache       map[int]*ent.Symbol
}

// NewProjectInheritanceAnalyzer creates a new project inheritance analyzer
func NewProjectInheritanceAnalyzer(db *ent.Client, ctx context.Context, symbolRegistry *SymbolRegistry, config Config) *ProjectInheritanceAnalyzer {
	return &ProjectInheritanceAnalyzer{
		db:                db,
		ctx:               ctx,
		globalGraph:       ast.NewInheritanceGraph(),
		symbolRegistry:    symbolRegistry,
		config:            config,
		relationshipCache: make(map[int]*ent.InheritanceRelationship),
		symbolCache:       make(map[int]*ent.Symbol),
	}
}

// BuildGlobalGraph constructs the complete project-level inheritance graph
func (pia *ProjectInheritanceAnalyzer) BuildGlobalGraph(projectID int) error {
	if pia.config.ShowProgress {
		fmt.Println("Building global inheritance graph...")
	}

	// Query all inheritance relationships for the project
	relationships, err := pia.db.InheritanceRelationship.Query().
		Where(inheritancerelationship.HasFileWith(file.HasProjectWith(project.ID(projectID)))).
		WithChild().
		WithParent().
		WithFile().
		All(pia.ctx)
	
	if err != nil {
		return fmt.Errorf("failed to query inheritance relationships: %w", err)
	}

	// Cache relationships for performance
	for _, rel := range relationships {
		pia.relationshipCache[rel.ID] = rel
	}

	// Build global inheritance graph from database relationships
	added := 0
	skipped := 0
	
	for _, rel := range relationships {
		if rel.Edges.Child != nil && rel.Edges.Parent != nil {
			// Add to global graph using the correct signature
			fileLocation := ""
			if rel.Edges.File != nil {
				fileLocation = rel.Edges.File.RelativePath
			}
			
			relationship := pia.globalGraph.AddRelationship(
				rel.Edges.Child.QualifiedName,
				rel.Edges.Parent.QualifiedName,
				ast.RelationshipType(rel.RelationshipType),
				fileLocation,
				uint32(rel.LineNumber),
			)
			
			// Update the relationship with additional metadata
			relationship.IsDirectInheritance = rel.IsDirectInheritance
			relationship.InheritanceDepth = rel.InheritanceDepth
			if rel.ModuleName != "" {
				relationship.ModuleName = rel.ModuleName
			}
			added++
			
			// Cache symbols
			pia.symbolCache[rel.Edges.Child.ID] = rel.Edges.Child
			pia.symbolCache[rel.Edges.Parent.ID] = rel.Edges.Parent
		} else {
			skipped++
		}
	}

	if pia.config.ShowProgress {
		fmt.Printf("Global inheritance graph: %d relationships added, %d skipped\n", added, skipped)
	}

	return nil
}

// DetectHierarchyIssues identifies inheritance-related quality issues
func (pia *ProjectInheritanceAnalyzer) DetectHierarchyIssues() ([]*InheritanceIssue, error) {
	var issues []*InheritanceIssue

	// 1. Circular inheritance detection (TODO: Implement DetectCircularInheritance method)
	// circularErrors := pia.globalGraph.DetectCircularInheritance()
	// for _, circErr := range circularErrors {
	// 	issues = append(issues, &InheritanceIssue{
	// 		Type:        "circular_inheritance",
	// 		Description: fmt.Sprintf("Circular inheritance detected: %v", circErr.Path),
	// 		Classes:     circErr.Path,
	// 		Severity:    "error",
	// 		Suggestion:  "Break the circular dependency by refactoring one of the inheritance relationships.",
	// 	})
	// }

	// 2. Deep inheritance hierarchies
	allClasses := pia.globalGraph.GetAllClasses()
	for _, className := range allClasses {
		ancestry := pia.globalGraph.GetAncestry(className)
		depth := len(ancestry) - 1 // Exclude self
		if depth > 5 { // Configurable threshold
			issues = append(issues, &InheritanceIssue{
				Type:        "deep_inheritance",
				Description: fmt.Sprintf("Deep inheritance hierarchy for class %s (depth: %d)", className, depth),
				Classes:     []string{className},
				Severity:    "warning",
				Suggestion:  "Consider using composition instead of deep inheritance for better maintainability.",
			})
		}
	}

	// 3. Complex multiple inheritance
	for _, className := range allClasses {
		directParents := pia.globalGraph.GetDirectParents(className)
		if len(directParents) > 2 {
			parentNames := make([]string, len(directParents))
			for i, parent := range directParents {
				parentNames[i] = parent.ParentClassName
			}
			issues = append(issues, &InheritanceIssue{
				Type:        "complex_multiple_inheritance",
				Description: fmt.Sprintf("Complex multiple inheritance for class %s (%d parents: %v)", className, len(directParents), parentNames),
				Classes:     append([]string{className}, parentNames...),
				Severity:    "info",
				Suggestion:  "Consider using interfaces or mixins to simplify the inheritance structure.",
			})
		}
	}

	// 4. Orphaned inheritance relationships (relationships with missing symbols)
	orphanedCount := pia.detectOrphanedRelationships()
	if orphanedCount > 0 {
		issues = append(issues, &InheritanceIssue{
			Type:        "orphaned_relationships",
			Description: fmt.Sprintf("Found %d inheritance relationships with missing symbol links", orphanedCount),
			Classes:     []string{}, // No specific classes
			Severity:    "warning",
			Suggestion:  "Review and clean up inheritance relationships that reference non-existent symbols.",
		})
	}

	// 5. Potential abstract class violations
	abstractIssues := pia.detectAbstractClassIssues()
	issues = append(issues, abstractIssues...)

	// 6. Method Resolution Order complexity
	mroIssues := pia.detectMROComplexity()
	issues = append(issues, mroIssues...)

	return issues, nil
}

// detectOrphanedRelationships finds relationships with missing symbol links
func (pia *ProjectInheritanceAnalyzer) detectOrphanedRelationships() int {
	orphanedCount := 0
	
	for _, rel := range pia.relationshipCache {
		if rel.Edges.Child == nil || rel.Edges.Parent == nil {
			orphanedCount++
		}
	}
	
	return orphanedCount
}

// detectAbstractClassIssues finds potential abstract class violations
func (pia *ProjectInheritanceAnalyzer) detectAbstractClassIssues() []*InheritanceIssue {
	var issues []*InheritanceIssue
	
	// Find classes marked as abstract that might have concrete implementations
	for _, symbol := range pia.symbolCache {
		if symbol.IsAbstract && string(symbol.SymbolType) == SymbolTypeClass {
			// Check if this abstract class is being instantiated (simplified check)
			descendants := pia.globalGraph.GetDescendants(symbol.QualifiedName)
			if len(descendants) == 1 { // Only itself, no child classes
				issues = append(issues, &InheritanceIssue{
					Type:        "abstract_class_no_children",
					Description: fmt.Sprintf("Abstract class %s has no child classes", symbol.QualifiedName),
					Classes:     []string{symbol.QualifiedName},
					Severity:    "info",
					Suggestion:  "Consider whether this class should be abstract or if child classes are missing.",
				})
			}
		}
	}
	
	return issues
}

// detectMROComplexity finds Method Resolution Order complexity issues
func (pia *ProjectInheritanceAnalyzer) detectMROComplexity() []*InheritanceIssue {
	var issues []*InheritanceIssue
	
	allClasses := pia.globalGraph.GetAllClasses()
	for _, className := range allClasses {
		mro := pia.globalGraph.GetMethodResolutionOrder(className)
		if len(mro) > 8 { // Configurable threshold
			issues = append(issues, &InheritanceIssue{
				Type:        "complex_mro",
				Description: fmt.Sprintf("Complex Method Resolution Order for class %s (length: %d)", className, len(mro)),
				Classes:     []string{className},
				Severity:    "warning",
				Suggestion:  "Consider simplifying the inheritance structure to reduce MRO complexity.",
			})
		}
	}
	
	return issues
}

// GenerateHierarchyStatistics computes comprehensive inheritance statistics
func (pia *ProjectInheritanceAnalyzer) GenerateHierarchyStatistics() (map[string]interface{}, error) {
	allClasses := pia.globalGraph.GetAllClasses()
	
	stats := map[string]interface{}{
		"total_classes":              len(allClasses),
		"root_classes":               len(pia.globalGraph.GetRootClasses()),
		"leaf_classes":               len(pia.globalGraph.GetLeafClasses()),
		"total_relationships":        len(pia.relationshipCache),
	}

	// Count various inheritance patterns
	classesWithInheritance := 0
	multipleInheritanceCount := 0
	abstractClassCount := 0
	maxDepth := 0
	totalDepth := 0
	maxMROLength := 0
	totalMROLength := 0
	mroClassCount := 0

	for _, className := range allClasses {
		// Inheritance patterns
		directParents := pia.globalGraph.GetDirectParents(className)
		if len(directParents) > 0 {
			classesWithInheritance++
		}
		if len(directParents) > 1 {
			multipleInheritanceCount++
		}

		// Depth analysis
		ancestry := pia.globalGraph.GetAncestry(className)
		depth := len(ancestry) - 1
		if depth > maxDepth {
			maxDepth = depth
		}
		totalDepth += depth

		// MRO analysis
		mro := pia.globalGraph.GetMethodResolutionOrder(className)
		if len(mro) > 1 {
			mroLength := len(mro)
			if mroLength > maxMROLength {
				maxMROLength = mroLength
			}
			totalMROLength += mroLength
			mroClassCount++
		}

		// Abstract class count (from symbol cache)
		if symbol, err := pia.symbolRegistry.ResolveSymbol(className); err == nil {
			if symbol.IsAbstract {
				abstractClassCount++
			}
		}
	}

	stats["classes_with_inheritance"] = classesWithInheritance
	stats["multiple_inheritance_count"] = multipleInheritanceCount
	stats["abstract_classes"] = abstractClassCount
	stats["max_inheritance_depth"] = maxDepth
	stats["max_mro_length"] = maxMROLength

	// Averages
	if len(allClasses) > 0 {
		stats["average_inheritance_depth"] = float64(totalDepth) / float64(len(allClasses))
	}
	if mroClassCount > 0 {
		stats["average_mro_length"] = float64(totalMROLength) / float64(mroClassCount)
	}

	// Quality metrics (TODO: Implement DetectCircularInheritance method)
	// circularErrors := pia.globalGraph.DetectCircularInheritance()
	// stats["circular_inheritance_errors"] = len(circularErrors)
	stats["circular_inheritance_errors"] = 0 // Placeholder
	
	deepClassesCount := 0
	complexMROCount := 0
	for _, className := range allClasses {
		ancestry := pia.globalGraph.GetAncestry(className)
		if len(ancestry) > 6 { // Deep threshold
			deepClassesCount++
		}
		
		mro := pia.globalGraph.GetMethodResolutionOrder(className)
		if len(mro) > 5 { // Complex MRO threshold
			complexMROCount++
		}
	}
	stats["deep_inheritance_classes"] = deepClassesCount
	stats["complex_mro_classes"] = complexMROCount
	
	// Language distribution
	languageStats := pia.calculateLanguageDistribution()
	stats["language_distribution"] = languageStats

	return stats, nil
}

// calculateLanguageDistribution calculates inheritance patterns by language
func (pia *ProjectInheritanceAnalyzer) calculateLanguageDistribution() map[string]interface{} {
	languageStats := make(map[string]map[string]int)
	
	// Get language information from file relationships
	for _, rel := range pia.relationshipCache {
		if rel.Edges.File != nil {
			language := string(rel.Edges.File.Language)
			if _, exists := languageStats[language]; !exists {
				languageStats[language] = map[string]int{
					"total_relationships": 0,
					"multiple_inheritance": 0,
					"deep_inheritance": 0,
				}
			}
			
			languageStats[language]["total_relationships"]++
			
			// Check for multiple inheritance
			if rel.Edges.Child != nil {
				childName := rel.Edges.Child.QualifiedName
				directParents := pia.globalGraph.GetDirectParents(childName)
				if len(directParents) > 1 {
					languageStats[language]["multiple_inheritance"]++
				}
				
				// Check for deep inheritance
				ancestry := pia.globalGraph.GetAncestry(childName)
				if len(ancestry) > 5 {
					languageStats[language]["deep_inheritance"]++
				}
			}
		}
	}
	
	result := make(map[string]interface{})
	for lang, stats := range languageStats {
		result[lang] = stats
	}
	return result
}

// GetGlobalGraph returns the global inheritance graph
func (pia *ProjectInheritanceAnalyzer) GetGlobalGraph() *ast.InheritanceGraph {
	return pia.globalGraph
}

// ExportGraphData exports inheritance graph data for external analysis
func (pia *ProjectInheritanceAnalyzer) ExportGraphData() *InheritanceGraphExport {
	allClasses := pia.globalGraph.GetAllClasses()
	
	export := &InheritanceGraphExport{
		Classes:       make([]string, len(allClasses)),
		Relationships: make([]*RelationshipExport, 0),
		Statistics:    make(map[string]interface{}),
	}
	
	// Export classes
	copy(export.Classes, allClasses)
	
	// Export relationships
	for _, className := range allClasses {
		directParents := pia.globalGraph.GetDirectParents(className)
		for _, parent := range directParents {
			export.Relationships = append(export.Relationships, &RelationshipExport{
				Child:  className,
				Parent: parent.ParentClassName,
				Type:   string(parent.RelationshipType),
				Depth:  parent.InheritanceDepth,
				Direct: parent.IsDirectInheritance,
			})
		}
	}
	
	// Export statistics
	stats, _ := pia.GenerateHierarchyStatistics()
	export.Statistics = stats
	
	return export
}

// InheritanceIssue represents an inheritance-related quality issue
type InheritanceIssue struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Classes     []string `json:"classes"`
	Severity    string   `json:"severity"` // "error", "warning", "info"
	Suggestion  string   `json:"suggestion"`
}

// InheritanceGraphExport represents exported inheritance graph data
type InheritanceGraphExport struct {
	Classes       []string                 `json:"classes"`
	Relationships []*RelationshipExport    `json:"relationships"`
	Statistics    map[string]interface{}   `json:"statistics"`
}

// RelationshipExport represents an exported inheritance relationship
type RelationshipExport struct {
	Child  string `json:"child"`
	Parent string `json:"parent"`
	Type   string `json:"type"`
	Depth  int    `json:"depth"`
	Direct bool   `json:"direct"`
}

// ValidateGlobalGraph performs validation checks on the global inheritance graph
func (pia *ProjectInheritanceAnalyzer) ValidateGlobalGraph() []string {
	var warnings []string
	
	// Check for inconsistencies
	allClasses := pia.globalGraph.GetAllClasses()
	for _, className := range allClasses {
		// Check if ancestry calculation is consistent
		ancestry := pia.globalGraph.GetAncestry(className)
		descendants := pia.globalGraph.GetDescendants(className)
		
		// Validate that the class appears in its own ancestry
		found := false
		for _, ancestor := range ancestry {
			if ancestor == className {
				found = true
				break
			}
		}
		if !found {
			warnings = append(warnings, fmt.Sprintf("Class %s not found in its own ancestry", className))
		}
		
		// Validate descendants don't include self
		for _, descendant := range descendants {
			if descendant == className {
				warnings = append(warnings, fmt.Sprintf("Class %s incorrectly included in its own descendants", className))
			}
		}
	}
	
	return warnings
}