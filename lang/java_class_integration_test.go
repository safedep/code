package lang_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
	"github.com/safedep/code/fs"
	"github.com/safedep/code/lang"
	"github.com/safedep/code/parser"
)

// Test visitor for finding specific fixture files
type javaFixtureVisitor struct {
	targetFilename string
	foundTree      *core.ParseTree
}

func (v *javaFixtureVisitor) VisitTree(tree core.ParseTree) error {
	file, err := tree.File()
	if err != nil {
		return err
	}

	if filepath.Base(file.Name()) == v.targetFilename {
		*v.foundTree = tree
	}
	return nil
}

func TestJavaClassResolutionWithRealFixtures(t *testing.T) {
	// Test simple classes first
	t.Run("SimpleClasses", func(t *testing.T) {
		testJavaClassResolution(t, "fixtures/java_simple_classes.java", map[string][]string{
			"SimpleClass":      {}, // No inheritance
			"ClassWithMethods": {}, // No inheritance
			"ClassWithFields":  {}, // No inheritance
			"AnnotatedClass":   {}, // No inheritance
			"SimpleInterface":  {}, // Interface (no inheritance)
			"StandaloneClass":  {}, // No inheritance
		})
	})

	// Test complex inheritance hierarchy
	t.Run("InheritanceHierarchy", func(t *testing.T) {
		testJavaClassResolution(t, "fixtures/java_class_hierarchy.java", map[string][]string{
			"BaseService":            {},                         // Abstract class (no inheritance)
			"StorageService":         {"BaseService"},            // Single inheritance
			"Cacheable":              {},                         // Interface (no inheritance)
			"Loggable":               {},                         // Interface (no inheritance)
			"AdvancedStorageService": {"StorageService"},         // Single class inheritance + interfaces
			"CloudStorageService":    {"AdvancedStorageService"}, // Single inheritance
			"AbstractProcessor":      {},                         // Abstract class (no inheritance)
			"DataProcessor":          {"AbstractProcessor"},      // Single inheritance
			"Level1":                 {"BaseService"},            // Single inheritance
			"Level2":                 {"Level1"},                 // Deep inheritance chain
			"Level3":                 {"Level2"},                 // Deep inheritance chain
			"Level4":                 {"Level3"},                 // Deep inheritance chain
			"GenericService":         {"BaseService"},            // Generic class inheritance
			"OuterClass":             {},                         // No inheritance
			"ExtendedInterface":      {},                         // Interface inheritance (handled as class)
			"TestRunner":             {},                         // No inheritance
		})
	})
}

func TestJavaInheritanceGraphConstruction(t *testing.T) {
	// Test inheritance graph construction with complex hierarchy
	parseTree := parseJavaFixtureFile(t, "fixtures/java_class_hierarchy.java")

	javaLang, err := lang.NewJavaLanguage()
	if err != nil {
		t.Fatalf("Failed to create Java language: %v", err)
	}

	resolvers := javaLang.Resolvers().(core.ObjectOrientedLanguageResolvers)

	inheritanceGraph, err := resolvers.ResolveInheritance(parseTree)
	if err != nil {
		t.Fatalf("Failed to resolve inheritance: %v", err)
	}

	// Test specific inheritance relationships
	testCases := []struct {
		child    string
		parent   string
		expected bool
	}{
		{"StorageService", "BaseService", true},
		{"AdvancedStorageService", "StorageService", true},
		{"CloudStorageService", "AdvancedStorageService", true},
		{"Level4", "Level3", true},
		{"Level4", "Level2", false}, // Direct relationship only
		{"DataProcessor", "AbstractProcessor", true},
		{"GenericService", "BaseService", true},
		{"BaseService", "StorageService", false}, // Wrong direction
	}

	for _, tc := range testCases {
		t.Run(tc.child+"_inherits_"+tc.parent, func(t *testing.T) {
			parentNames := inheritanceGraph.GetDirectParentNames(tc.child)
			found := false
			for _, parent := range parentNames {
				if parent == tc.parent {
					found = true
					break
				}
			}

			if found != tc.expected {
				t.Errorf("Expected %s inherits %s = %v, got %v", tc.child, tc.parent, tc.expected, found)
			}
		})
	}

	// Test ancestry (transitive inheritance)
	if !inheritanceGraph.IsAncestor("BaseService", "Level4") {
		t.Error("BaseService should be an ancestor of Level4 through inheritance chain")
	}

	if !inheritanceGraph.IsAncestor("BaseService", "CloudStorageService") {
		t.Error("BaseService should be an ancestor of CloudStorageService")
	}

	// Test that graph has expected number of classes
	allClasses := inheritanceGraph.GetAllClasses()
	if len(allClasses) < 5 { // Should have at least 5 classes with inheritance from fixture
		t.Errorf("Expected at least 5 classes in inheritance graph, got %d", len(allClasses))
	}
}

func TestJavaClassMethodExtraction(t *testing.T) {
	parseTree := parseJavaFixtureFile(t, "fixtures/java_class_hierarchy.java")

	javaLang, err := lang.NewJavaLanguage()
	if err != nil {
		t.Fatalf("Failed to create Java language: %v", err)
	}

	resolvers := javaLang.Resolvers().(core.ObjectOrientedLanguageResolvers)

	classes, err := resolvers.ResolveClasses(parseTree)
	if err != nil {
		t.Fatalf("Failed to resolve classes: %v", err)
	}

	// Find specific classes and test their methods
	classMap := make(map[string]*ast.ClassDeclarationNode)
	for _, class := range classes {
		classMap[class.ClassName()] = class
	}

	// Test BaseService methods
	if baseService, exists := classMap["BaseService"]; exists {
		methods := baseService.GetMethodNodes()
		if len(methods) < 2 { // Should have getConfig, getServiceType, isInitialized
			t.Errorf("BaseService should have at least 2 methods, got %d", len(methods))
		}

		// Should have constructor
		if baseService.GetConstructorNode() == nil {
			t.Error("BaseService should have constructor")
		}

		// Should be marked as abstract
		if !baseService.IsAbstract() {
			t.Error("BaseService should be marked as abstract")
		}
	} else {
		t.Error("BaseService not found in resolved classes")
	}

	// Test AdvancedStorageService with multiple interface implementation
	if advancedService, exists := classMap["AdvancedStorageService"]; exists {
		baseClasses := advancedService.BaseClasses()
		if len(baseClasses) != 1 { // Java single inheritance - only StorageService
			t.Errorf("AdvancedStorageService should have 1 base class, got %d", len(baseClasses))
		}

		if baseClasses[0] != "StorageService" {
			t.Errorf("AdvancedStorageService should inherit from StorageService, got %s", baseClasses[0])
		}
	} else {
		t.Error("AdvancedStorageService not found in resolved classes")
	}
}

func TestJavaClassAnnotationsAndAbstractClasses(t *testing.T) {
	parseTree := parseJavaFixtureFile(t, "fixtures/java_class_hierarchy.java")

	javaLang, err := lang.NewJavaLanguage()
	if err != nil {
		t.Fatalf("Failed to create Java language: %v", err)
	}

	resolvers := javaLang.Resolvers().(core.ObjectOrientedLanguageResolvers)

	classes, err := resolvers.ResolveClasses(parseTree)
	if err != nil {
		t.Fatalf("Failed to resolve classes: %v", err)
	}

	// Find AbstractProcessor class
	var abstractProcessor *ast.ClassDeclarationNode
	for _, class := range classes {
		if class.ClassName() == "AbstractProcessor" {
			abstractProcessor = class
			break
		}
	}

	if abstractProcessor == nil {
		t.Fatal("AbstractProcessor not found in resolved classes")
	}

	// Test that it's marked as abstract
	if !abstractProcessor.IsAbstract() {
		t.Error("AbstractProcessor should be marked as abstract")
	}

	// Test that it has annotations (decorators)
	decorators := abstractProcessor.GetDecoratorNodes()
	if len(decorators) == 0 {
		t.Error("AbstractProcessor should have annotations")
	}
}

func TestJavaInterfaceResolution(t *testing.T) {
	parseTree := parseJavaFixtureFile(t, "fixtures/java_simple_classes.java")

	javaLang, err := lang.NewJavaLanguage()
	if err != nil {
		t.Fatalf("Failed to create Java language: %v", err)
	}

	resolvers := javaLang.Resolvers().(core.ObjectOrientedLanguageResolvers)

	classes, err := resolvers.ResolveClasses(parseTree)
	if err != nil {
		t.Fatalf("Failed to resolve classes: %v", err)
	}

	// Find SimpleInterface
	var simpleInterface *ast.ClassDeclarationNode
	for _, class := range classes {
		if class.ClassName() == "SimpleInterface" {
			simpleInterface = class
			break
		}
	}

	if simpleInterface == nil {
		t.Fatal("SimpleInterface not found in resolved classes")
	}

	// Interfaces should be marked as abstract
	if !simpleInterface.IsAbstract() {
		t.Error("SimpleInterface should be marked as abstract (interfaces are abstract)")
	}

	// Should have public access modifier
	if simpleInterface.AccessModifier() != ast.AccessModifierPublic {
		t.Error("SimpleInterface should have public access modifier")
	}
}

// Helper functions

func testJavaClassResolution(t *testing.T, fixturePath string, expectedClasses map[string][]string) {
	parseTree := parseJavaFixtureFile(t, fixturePath)

	javaLang, err := lang.NewJavaLanguage()
	if err != nil {
		t.Fatalf("Failed to create Java language: %v", err)
	}

	resolvers := javaLang.Resolvers().(core.ObjectOrientedLanguageResolvers)

	classes, err := resolvers.ResolveClasses(parseTree)
	if err != nil {
		t.Fatalf("Failed to resolve classes: %v", err)
	}

	// Create map of found classes
	foundClasses := make(map[string][]string)
	for _, class := range classes {
		foundClasses[class.ClassName()] = class.BaseClasses()
	}

	// Verify all expected classes were found
	for expectedName, expectedBases := range expectedClasses {
		if foundBases, exists := foundClasses[expectedName]; exists {
			// Check base classes match
			if len(foundBases) != len(expectedBases) {
				t.Errorf("Class %s: expected %d base classes %v, got %d: %v",
					expectedName, len(expectedBases), expectedBases, len(foundBases), foundBases)
				continue
			}

			// Check each expected base class
			expectedBaseMap := make(map[string]bool)
			for _, base := range expectedBases {
				expectedBaseMap[base] = false
			}

			for _, foundBase := range foundBases {
				if _, expected := expectedBaseMap[foundBase]; expected {
					expectedBaseMap[foundBase] = true
				} else {
					t.Errorf("Class %s: unexpected base class %s", expectedName, foundBase)
				}
			}

			for base, found := range expectedBaseMap {
				if !found {
					t.Errorf("Class %s: missing expected base class %s", expectedName, base)
				}
			}
		} else {
			t.Errorf("Expected class %s not found in resolved classes", expectedName)
		}
	}

	// Check for unexpected classes (optional - helps catch over-extraction)
	for foundName := range foundClasses {
		if _, expected := expectedClasses[foundName]; !expected {
			t.Logf("Note: Found unexpected class %s (may be from imports or inner classes)", foundName)
		}
	}
}

func parseJavaFixtureFile(t *testing.T, relativePath string) core.ParseTree {
	// Get absolute path to fixture
	fixtureDir := filepath.Join(".", relativePath)

	// Create filesystem for the fixture
	fileSystem, err := fs.NewLocalFileSystem(fs.LocalFileSystemConfig{
		AppDirectories: []string{filepath.Dir(fixtureDir)},
	})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	// Create Java language
	javaLang, err := lang.NewJavaLanguage()
	if err != nil {
		t.Fatalf("Failed to create Java language: %v", err)
	}

	// Create walker and parser
	walker, err := fs.NewSourceWalker(fs.SourceWalkerConfig{}, []core.Language{javaLang})
	if err != nil {
		t.Fatalf("Failed to create walker: %v", err)
	}

	treeWalker, err := parser.NewWalkingParser(walker, []core.Language{javaLang})
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}

	// Find and parse the specific file
	var parseTree core.ParseTree
	visitor := &javaFixtureVisitor{
		targetFilename: filepath.Base(relativePath),
		foundTree:      &parseTree,
	}

	err = treeWalker.Walk(context.Background(), fileSystem, visitor)
	if err != nil {
		t.Fatalf("Failed to parse fixture file: %v", err)
	}

	if parseTree == nil {
		t.Fatalf("Fixture file %s not found or not parsed", relativePath)
	}

	return parseTree
}
