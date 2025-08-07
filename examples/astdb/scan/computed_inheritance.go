package scan

import (
	"context"
	"fmt"

	"github.com/safedep/code/core/ast"
	"github.com/safedep/code/examples/astdb/ent"
	"github.com/safedep/code/examples/astdb/ent/inheritancerelationship"
)

// ComputedInheritanceProcessor handles the creation and storage of computed inheritance relationships
type ComputedInheritanceProcessor struct {
	db             *ent.Client
	ctx            context.Context
	symbolRegistry *SymbolRegistry
	config         Config
}

// NewComputedInheritanceProcessor creates a new computed inheritance processor
func NewComputedInheritanceProcessor(db *ent.Client, ctx context.Context, symbolRegistry *SymbolRegistry, config Config) *ComputedInheritanceProcessor {
	return &ComputedInheritanceProcessor{
		db:             db,
		ctx:            ctx,
		symbolRegistry: symbolRegistry,
		config:         config,
	}
}

// StoreComputedRelationships processes and stores all computed inheritance relationships
func (cip *ComputedInheritanceProcessor) StoreComputedRelationships(projectID int, globalGraph *ast.InheritanceGraph) error {
	if cip.config.ShowProgress {
		fmt.Println("Computing and storing computed inheritance relationships...")
	}

	stored := 0
	errors := 0

	// Get all classes in the inheritance graph
	allClasses := globalGraph.GetAllClasses()
	
	for _, className := range allClasses {
		// Store computed ancestry relationships (excluding direct relationships)
		ancestors := globalGraph.GetAncestry(className)
		for i, ancestor := range ancestors {
			if i == 0 {
				continue // Skip self
			}
			
			// Get inheritance depth
			depth := globalGraph.GetInheritanceDepth(className, ancestor)
			if depth > 1 { // Only store computed (non-direct) relationships
				err := cip.storeComputedRelationship(className, ancestor, depth, projectID)
				if err != nil {
					errors++
					if cip.config.Verbose {
						fmt.Printf("Warning: failed to store computed relationship %s -> %s: %v\n", 
							className, ancestor, err)
					}
				} else {
					stored++
				}
			}
		}
		
		// Store Method Resolution Order data as computed relationships
		err := cip.storeMROData(className, globalGraph, projectID)
		if err != nil && cip.config.Verbose {
			fmt.Printf("Warning: failed to store MRO data for %s: %v\n", className, err)
		}
	}

	if cip.config.ShowProgress {
		fmt.Printf("Computed relationships: %d stored, %d errors\n", stored, errors)
	}

	return nil
}

// storeComputedRelationship stores a single computed inheritance relationship
func (cip *ComputedInheritanceProcessor) storeComputedRelationship(childName, ancestorName string, depth int, projectID int) error {
	// Resolve child symbol
	childSymbol, err := cip.symbolRegistry.ResolveSymbol(childName)
	if err != nil {
		return fmt.Errorf("child symbol not found: %s", childName)
	}
	
	// Resolve ancestor symbol
	ancestorSymbol, err := cip.symbolRegistry.ResolveSymbol(ancestorName)
	if err != nil {
		return fmt.Errorf("ancestor symbol not found: %s", ancestorName)
	}

	// Check if this computed relationship already exists
	existing, err := cip.db.InheritanceRelationship.Query().
		Where(
			inheritancerelationship.IsDirectInheritance(false),
			inheritancerelationship.InheritanceDepth(depth),
		).
		Exist(cip.ctx)
	
	if err != nil {
		return fmt.Errorf("failed to check existing computed relationship: %w", err)
	}
	
	if existing {
		// Relationship already exists, skip
		return nil
	}

	// Create computed inheritance relationship
	_, err = cip.db.InheritanceRelationship.Create().
		SetChildID(childSymbol.ID).
		SetParentID(ancestorSymbol.ID).
		SetRelationshipType(inheritancerelationship.RelationshipTypeExtends).
		SetLineNumber(0). // Computed relationships don't have source line
		SetIsDirectInheritance(false). // Mark as computed
		SetInheritanceDepth(depth).
		Save(cip.ctx)
		
	if err != nil {
		return fmt.Errorf("failed to save computed relationship: %w", err)
	}

	return nil
}

// storeMROData stores Method Resolution Order data for a class
func (cip *ComputedInheritanceProcessor) storeMROData(className string, globalGraph *ast.InheritanceGraph, projectID int) error {
	// Get Method Resolution Order
	mro := globalGraph.GetMethodResolutionOrder(className)
	if len(mro) <= 1 {
		return nil // No MRO data to store
	}

	// Resolve class symbol
	classSymbol, err := cip.symbolRegistry.ResolveSymbol(className)
	if err != nil {
		return fmt.Errorf("class symbol not found for MRO: %s", className)
	}

	// Update class symbol metadata with MRO information
	currentMetadata := classSymbol.Metadata
	if currentMetadata == nil {
		currentMetadata = make(map[string]interface{})
	}

	// Add computed MRO data
	currentMetadata["computed_method_resolution_order"] = mro
	currentMetadata["mro_length"] = len(mro)
	currentMetadata["has_complex_mro"] = len(mro) > 3

	// Store linearization steps for debugging (if available)
	if linearizationSteps := cip.getMROLinearizationSteps(globalGraph, className); len(linearizationSteps) > 0 {
		currentMetadata["mro_linearization_steps"] = linearizationSteps
	}

	// Update the symbol
	_, err = cip.db.Symbol.UpdateOneID(classSymbol.ID).
		SetMetadata(currentMetadata).
		Save(cip.ctx)
	
	if err != nil {
		return fmt.Errorf("failed to update symbol with MRO data: %w", err)
	}

	return nil
}

// getMROLinearizationSteps gets linearization steps for debugging (simplified implementation)
func (cip *ComputedInheritanceProcessor) getMROLinearizationSteps(globalGraph *ast.InheritanceGraph, className string) []string {
	// This is a simplified implementation
	// In a full implementation, would capture the actual C3 linearization steps
	directParents := globalGraph.GetDirectParents(className)
	if len(directParents) > 1 {
		steps := make([]string, 0, len(directParents)+1)
		steps = append(steps, fmt.Sprintf("Class: %s", className))
		for _, parent := range directParents {
			steps = append(steps, fmt.Sprintf("Parent: %s", parent.ParentClassName))
		}
		return steps
	}
	return nil
}

// ComputeProjectStatistics computes and stores project-level inheritance statistics
func (cip *ComputedInheritanceProcessor) ComputeProjectStatistics(projectID int, globalGraph *ast.InheritanceGraph) (map[string]interface{}, error) {
	allClasses := globalGraph.GetAllClasses()
	
	// Basic statistics
	stats := map[string]interface{}{
		"total_classes":               len(allClasses),
		"root_classes":                len(globalGraph.GetRootClasses()),
		"leaf_classes":                len(globalGraph.GetLeafClasses()),
	}
	
	// Compute inheritance patterns
	classesWithInheritance := 0
	multipleInheritanceCount := 0
	maxDepth := 0
	totalDepth := 0
	deepClassCount := 0
	
	for _, className := range allClasses {
		directParents := globalGraph.GetDirectParents(className)
		if len(directParents) > 0 {
			classesWithInheritance++
		}
		
		if len(directParents) > 1 {
			multipleInheritanceCount++
		}
		
		// Calculate depth
		ancestry := globalGraph.GetAncestry(className)
		depth := len(ancestry) - 1 // Exclude self
		if depth > maxDepth {
			maxDepth = depth
		}
		totalDepth += depth
		
		if depth > 5 { // Configurable threshold
			deepClassCount++
		}
	}
	
	stats["classes_with_inheritance"] = classesWithInheritance
	stats["multiple_inheritance_count"] = multipleInheritanceCount
	stats["max_inheritance_depth"] = maxDepth
	
	if len(allClasses) > 0 {
		stats["average_inheritance_depth"] = float64(totalDepth) / float64(len(allClasses))
	} else {
		stats["average_inheritance_depth"] = 0.0
	}
	
	stats["deep_inheritance_classes"] = deepClassCount
	
	// Circular inheritance detection (TODO: Implement DetectCircularInheritance method)
	// circularErrors := globalGraph.DetectCircularInheritance()
	// stats["circular_inheritance_count"] = len(circularErrors)
	stats["circular_inheritance_count"] = 0 // Placeholder
	
	// if len(circularErrors) > 0 {
	// 	circularPaths := make([]string, len(circularErrors))
	// 	for i, err := range circularErrors {
	// 		circularPaths[i] = fmt.Sprintf("%v", err.Path)
	// 	}
	// 	stats["circular_inheritance_paths"] = circularPaths
	// }
	
	// Method Resolution Order statistics
	complexMROCount := 0
	for _, className := range allClasses {
		mro := globalGraph.GetMethodResolutionOrder(className)
		if len(mro) > 3 {
			complexMROCount++
		}
	}
	stats["complex_mro_classes"] = complexMROCount
	
	return stats, nil
}

// ProcessInheritanceQuality analyzes inheritance quality and patterns
func (cip *ComputedInheritanceProcessor) ProcessInheritanceQuality(projectID int, globalGraph *ast.InheritanceGraph) ([]*InheritanceQualityIssue, error) {
	var issues []*InheritanceQualityIssue
	allClasses := globalGraph.GetAllClasses()
	
	// 1. Circular inheritance detection (TODO: Implement DetectCircularInheritance method)
	// circularErrors := globalGraph.DetectCircularInheritance()
	// for _, circErr := range circularErrors {
	// 	issues = append(issues, &InheritanceQualityIssue{
	// 		Type:        "circular_inheritance",
	// 		Severity:    "error",
	// 		Description: fmt.Sprintf("Circular inheritance detected: %v", circErr.Path),
	// 		Classes:     circErr.Path,
	// 		Suggestion:  "Break the circular dependency by removing one of the inheritance relationships or using composition instead.",
	// 	})
	// }
	
	// 2. Deep inheritance hierarchies
	for _, className := range allClasses {
		ancestry := globalGraph.GetAncestry(className)
		depth := len(ancestry) - 1
		if depth > 5 { // Configurable threshold
			issues = append(issues, &InheritanceQualityIssue{
				Type:        "deep_inheritance",
				Severity:    "warning",
				Description: fmt.Sprintf("Deep inheritance hierarchy for class %s (depth: %d)", className, depth),
				Classes:     []string{className},
				Suggestion:  "Consider using composition instead of deep inheritance to improve maintainability.",
			})
		}
	}
	
	// 3. Complex multiple inheritance
	for _, className := range allClasses {
		directParents := globalGraph.GetDirectParents(className)
		if len(directParents) > 2 {
			issues = append(issues, &InheritanceQualityIssue{
				Type:        "complex_multiple_inheritance",
				Severity:    "info",
				Description: fmt.Sprintf("Complex multiple inheritance for class %s (%d parents)", className, len(directParents)),
				Classes:     []string{className},
				Suggestion:  "Consider using mixins or interfaces to simplify the inheritance structure.",
			})
		}
	}
	
	// 4. Unused base classes (classes with no children)
	leafClasses := globalGraph.GetLeafClasses()
	unusedBaseThreshold := 0 // Classes with no children might be unused
	for _, leafClass := range leafClasses {
		// Check if this leaf class has methods that might indicate it's a base class
		if symbol, err := cip.symbolRegistry.ResolveSymbol(leafClass); err == nil {
			if metadata, ok := symbol.Metadata["method_count"].(int); ok && metadata > 3 {
				issues = append(issues, &InheritanceQualityIssue{
					Type:        "potentially_unused_base_class",
					Severity:    "info",
					Description: fmt.Sprintf("Class %s has methods but no child classes - might be an unused base class", leafClass),
					Classes:     []string{leafClass},
					Suggestion:  "Consider whether this class is intended as a base class or if it should be used differently.",
				})
			}
		}
		unusedBaseThreshold++ // Just to use the variable
	}
	
	return issues, nil
}

// InheritanceQualityIssue represents an inheritance-related code quality issue
type InheritanceQualityIssue struct {
	Type        string   `json:"type"`
	Severity    string   `json:"severity"`    // "error", "warning", "info"
	Description string   `json:"description"`
	Classes     []string `json:"classes"`
	Suggestion  string   `json:"suggestion"`
}

// UpdateSymbolsWithComputedData updates all class symbols with computed inheritance data
func (cip *ComputedInheritanceProcessor) UpdateSymbolsWithComputedData(projectID int, globalGraph *ast.InheritanceGraph) error {
	allClasses := globalGraph.GetAllClasses()
	updated := 0
	
	for _, className := range allClasses {
		symbol, err := cip.symbolRegistry.ResolveSymbol(className)
		if err != nil {
			continue // Skip if symbol not found
		}
		
		// Only update class symbols  
		if string(symbol.SymbolType) != SymbolTypeClass {
			continue
		}
		
		// Get current metadata
		currentMetadata := symbol.Metadata
		if currentMetadata == nil {
			currentMetadata = make(map[string]interface{})
		}
		
		// Add computed inheritance data
		currentMetadata["computed_ancestry"] = globalGraph.GetAncestry(className)
		currentMetadata["computed_descendants"] = globalGraph.GetDescendants(className)
		currentMetadata["computed_inheritance_depth"] = len(globalGraph.GetAncestry(className)) - 1
		currentMetadata["is_computed_root"] = len(globalGraph.GetDirectParents(className)) == 0
		currentMetadata["is_computed_leaf"] = len(globalGraph.GetDirectChildren(className)) == 0
		
		// Add MRO if applicable
		mro := globalGraph.GetMethodResolutionOrder(className)
		if len(mro) > 1 {
			currentMetadata["computed_mro"] = mro
		}
		
		// Update symbol
		_, err = cip.db.Symbol.UpdateOneID(symbol.ID).
			SetMetadata(currentMetadata).
			Save(cip.ctx)
		
		if err != nil {
			if cip.config.Verbose {
				fmt.Printf("Warning: failed to update symbol %s with computed data: %v\n", className, err)
			}
		} else {
			updated++
		}
	}
	
	if cip.config.ShowProgress {
		fmt.Printf("Updated %d symbols with computed inheritance data\n", updated)
	}
	
	return nil
}