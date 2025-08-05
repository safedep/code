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
type fixtureVisitor struct {
	targetFilename string
	foundTree      *core.ParseTree
}

func (v *fixtureVisitor) VisitTree(tree core.ParseTree) error {
	file, err := tree.File()
	if err != nil {
		return err
	}

	if filepath.Base(file.Name()) == v.targetFilename {
		*v.foundTree = tree
	}
	return nil
}

func TestPythonClassResolutionWithRealFixtures(t *testing.T) {
	// Test simple classes first
	t.Run("SimpleClasses", func(t *testing.T) {
		testPythonClassResolution(t, "fixtures/python_simple_classes.py", map[string][]string{
			"SimpleClass":      {}, // No inheritance
			"ClassWithMethods": {}, // No inheritance
			"ClassWithFields":  {}, // No inheritance
			"DecoratedClass":   {}, // No inheritance
			"StandaloneClass":  {}, // No inheritance
		})
	})

	// Test complex inheritance hierarchy
	t.Run("InheritanceHierarchy", func(t *testing.T) {
		testPythonClassResolution(t, "fixtures/python_class_hierarchy.py", map[string][]string{
			"BaseService":            {},                                          // No inheritance
			"StorageService":         {"BaseService"},                             // Single inheritance
			"Cacheable":              {},                                          // No inheritance
			"Loggable":               {},                                          // No inheritance
			"AdvancedStorageService": {"StorageService", "Cacheable", "Loggable"}, // Multiple inheritance
			"CloudStorageService":    {"AdvancedStorageService"},                  // Single inheritance
			"AbstractProcessor":      {"ABC"},                                     // From abc import
			"DataProcessor":          {"AbstractProcessor"},                       // Single inheritance
			"Level1":                 {"BaseService"},                             // Single inheritance
			"Level2":                 {"Level1"},                                  // Deep inheritance chain
			"Level3":                 {"Level2"},                                  // Deep inheritance chain
			"Level4":                 {"Level3"},                                  // Deep inheritance chain
			"ServiceWithDefaults":    {"BaseService"},                             // Single inheritance
		})
	})
}

func TestPythonInheritanceGraphConstruction(t *testing.T) {
	// Test inheritance graph construction with complex hierarchy
	parseTree := parseFixtureFile(t, "fixtures/python_class_hierarchy.py")

	pythonLang, err := lang.NewPythonLanguage()
	if err != nil {
		t.Fatalf("Failed to create Python language: %v", err)
	}

	resolvers := pythonLang.Resolvers().(core.ObjectOrientedLanguageResolvers)

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
		{"AdvancedStorageService", "Cacheable", true},
		{"AdvancedStorageService", "Loggable", true},
		{"CloudStorageService", "AdvancedStorageService", true},
		{"Level4", "Level3", true},
		{"Level4", "Level2", false}, // Direct relationship only
		{"DataProcessor", "AbstractProcessor", true},
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
	if len(allClasses) < 10 { // Should have at least 10 classes from fixture
		t.Errorf("Expected at least 10 classes in inheritance graph, got %d", len(allClasses))
	}
}

func TestPythonClassMethodExtraction(t *testing.T) {
	parseTree := parseFixtureFile(t, "fixtures/python_class_hierarchy.py")

	pythonLang, err := lang.NewPythonLanguage()
	if err != nil {
		t.Fatalf("Failed to create Python language: %v", err)
	}

	resolvers := pythonLang.Resolvers().(core.ObjectOrientedLanguageResolvers)

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
		if len(methods) < 3 { // __init__, get_config, status
			t.Errorf("BaseService should have at least 3 methods, got %d", len(methods))
		}

		// Should have constructor
		if baseService.GetConstructorNode() == nil {
			t.Error("BaseService should have constructor (__init__)")
		}
	} else {
		t.Error("BaseService not found in resolved classes")
	}

	// Test AdvancedStorageService with multiple inheritance
	if advancedService, exists := classMap["AdvancedStorageService"]; exists {
		baseClasses := advancedService.BaseClasses()
		if len(baseClasses) != 3 {
			t.Errorf("AdvancedStorageService should have 3 base classes, got %d", len(baseClasses))
		}

		expectedBases := map[string]bool{
			"StorageService": false,
			"Cacheable":      false,
			"Loggable":       false,
		}

		for _, base := range baseClasses {
			if _, exists := expectedBases[base]; exists {
				expectedBases[base] = true
			}
		}

		for base, found := range expectedBases {
			if !found {
				t.Errorf("AdvancedStorageService should inherit from %s", base)
			}
		}
	} else {
		t.Error("AdvancedStorageService not found in resolved classes")
	}
}

func TestPythonClassDecoratorsAndAbstractClasses(t *testing.T) {
	parseTree := parseFixtureFile(t, "fixtures/python_class_hierarchy.py")

	pythonLang, err := lang.NewPythonLanguage()
	if err != nil {
		t.Fatalf("Failed to create Python language: %v", err)
	}

	resolvers := pythonLang.Resolvers().(core.ObjectOrientedLanguageResolvers)

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

	// Test that it has decorators
	decorators := abstractProcessor.GetDecoratorNodes()
	if len(decorators) == 0 {
		t.Error("AbstractProcessor should have decorators")
	}
}

// Helper functions

func testPythonClassResolution(t *testing.T, fixturePath string, expectedClasses map[string][]string) {
	parseTree := parseFixtureFile(t, fixturePath)

	pythonLang, err := lang.NewPythonLanguage()
	if err != nil {
		t.Fatalf("Failed to create Python language: %v", err)
	}

	resolvers := pythonLang.Resolvers().(core.ObjectOrientedLanguageResolvers)

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
			t.Logf("Note: Found unexpected class %s (may be from imports or nested classes)", foundName)
		}
	}
}

func parseFixtureFile(t *testing.T, relativePath string) core.ParseTree {
	// Get absolute path to fixture
	fixtureDir := filepath.Join(".", relativePath)

	// Create filesystem for the fixture
	fileSystem, err := fs.NewLocalFileSystem(fs.LocalFileSystemConfig{
		AppDirectories: []string{filepath.Dir(fixtureDir)},
	})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	// Create Python language
	pythonLang, err := lang.NewPythonLanguage()
	if err != nil {
		t.Fatalf("Failed to create Python language: %v", err)
	}

	// Create walker and parser
	walker, err := fs.NewSourceWalker(fs.SourceWalkerConfig{}, []core.Language{pythonLang})
	if err != nil {
		t.Fatalf("Failed to create walker: %v", err)
	}

	treeWalker, err := parser.NewWalkingParser(walker, []core.Language{pythonLang})
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}

	// Find and parse the specific file
	var parseTree core.ParseTree
	visitor := &fixtureVisitor{
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
