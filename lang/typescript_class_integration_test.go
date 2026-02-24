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

func TestTypescriptClassResolutionWithRealFixtures(t *testing.T) {
	t.Run("SimpleClasses", func(t *testing.T) {
		testTypescriptClassResolution(t, "fixtures/ts_simple_classes.ts", map[string][]string{
			"SimpleClass":      {},
			"ClassWithMethods": {},
			"ClassWithFields":  {},
			"SimpleInterface":  {},
			"Direction":        {},
			"DecoratedClass":   {},
			"StandaloneClass":  {},
		})
	})

	t.Run("InheritanceHierarchy", func(t *testing.T) {
		testTypescriptClassResolution(t, "fixtures/ts_class_hierarchy.ts", map[string][]string{
			"BaseService":            {},
			"StorageService":         {"BaseService"},
			"Cacheable":              {},
			"Loggable":               {},
			"AdvancedStorageService": {"StorageService", "Cacheable", "Loggable"},
			"CloudStorageService":    {"AdvancedStorageService"},
			"AbstractProcessor":      {},
			"DataProcessor":          {"AbstractProcessor"},
			"Level1":                 {"BaseService"},
			"Level2":                 {"Level1"},
			"Level3":                 {"Level2"},
			"Level4":                 {"Level3"},
			"GenericService":         {"BaseService"},
			"OuterClass":             {},
			"ExtendedInterface":      {"Cacheable"},
			"TestRunner":             {},
		})
	})
}

func TestTypescriptInheritanceGraphConstruction(t *testing.T) {
	parseTree := parseTypescriptFixtureFile(t, "fixtures/ts_class_hierarchy.ts")

	tsLang, err := lang.NewTypescriptLanguage()
	if err != nil {
		t.Fatalf("Failed to create TypeScript language: %v", err)
	}

	resolvers := tsLang.Resolvers().(core.ObjectOrientedLanguageResolvers)

	inheritanceGraph, err := resolvers.ResolveInheritance(parseTree)
	if err != nil {
		t.Fatalf("Failed to resolve inheritance: %v", err)
	}

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
		{"GenericService", "BaseService", true},
		{"ExtendedInterface", "Cacheable", true},
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

	allClasses := inheritanceGraph.GetAllClasses()
	if len(allClasses) < 5 {
		t.Errorf("Expected at least 5 classes in inheritance graph, got %d", len(allClasses))
	}
}

func TestTypescriptClassMethodExtraction(t *testing.T) {
	parseTree := parseTypescriptFixtureFile(t, "fixtures/ts_class_hierarchy.ts")

	tsLang, err := lang.NewTypescriptLanguage()
	if err != nil {
		t.Fatalf("Failed to create TypeScript language: %v", err)
	}

	resolvers := tsLang.Resolvers().(core.ObjectOrientedLanguageResolvers)

	classes, err := resolvers.ResolveClasses(parseTree)
	if err != nil {
		t.Fatalf("Failed to resolve classes: %v", err)
	}

	classMap := make(map[string]*ast.ClassDeclarationNode)
	for _, class := range classes {
		classMap[class.ClassName()] = class
	}

	// Test BaseService methods
	if baseService, exists := classMap["BaseService"]; exists {
		methods := baseService.GetMethodNodes()
		if len(methods) < 2 { // getConfig, isInitialized
			t.Errorf("BaseService should have at least 2 methods, got %d", len(methods))
		}

		if baseService.GetConstructorNode() == nil {
			t.Error("BaseService should have constructor")
		}

		if !baseService.IsAbstract() {
			t.Error("BaseService should be marked as abstract")
		}
	} else {
		t.Error("BaseService not found in resolved classes")
	}

	// Test AdvancedStorageService with multiple interface implementation
	if advancedService, exists := classMap["AdvancedStorageService"]; exists {
		baseClasses := advancedService.BaseClasses()
		if len(baseClasses) != 3 {
			t.Errorf("AdvancedStorageService should have 3 base classes, got %d: %v", len(baseClasses), baseClasses)
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

func TestTypescriptInterfaceResolution(t *testing.T) {
	parseTree := parseTypescriptFixtureFile(t, "fixtures/ts_simple_classes.ts")

	tsLang, err := lang.NewTypescriptLanguage()
	if err != nil {
		t.Fatalf("Failed to create TypeScript language: %v", err)
	}

	resolvers := tsLang.Resolvers().(core.ObjectOrientedLanguageResolvers)

	classes, err := resolvers.ResolveClasses(parseTree)
	if err != nil {
		t.Fatalf("Failed to resolve classes: %v", err)
	}

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

	if !simpleInterface.IsAbstract() {
		t.Error("SimpleInterface should be marked as abstract (interfaces are abstract)")
	}

	if simpleInterface.AccessModifier() != ast.AccessModifierPublic {
		t.Error("SimpleInterface should have public access modifier")
	}
}

func TestTypescriptAbstractClassResolution(t *testing.T) {
	parseTree := parseTypescriptFixtureFile(t, "fixtures/ts_class_hierarchy.ts")

	tsLang, err := lang.NewTypescriptLanguage()
	if err != nil {
		t.Fatalf("Failed to create TypeScript language: %v", err)
	}

	resolvers := tsLang.Resolvers().(core.ObjectOrientedLanguageResolvers)

	classes, err := resolvers.ResolveClasses(parseTree)
	if err != nil {
		t.Fatalf("Failed to resolve classes: %v", err)
	}

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

	if !abstractProcessor.IsAbstract() {
		t.Error("AbstractProcessor should be marked as abstract")
	}

	// Test that it has decorators
	decorators := abstractProcessor.GetDecoratorNodes()
	if len(decorators) == 0 {
		t.Error("AbstractProcessor should have decorators (@Deprecated)")
	}
}

func TestTypescriptEnumResolution(t *testing.T) {
	parseTree := parseTypescriptFixtureFile(t, "fixtures/ts_simple_classes.ts")

	tsLang, err := lang.NewTypescriptLanguage()
	if err != nil {
		t.Fatalf("Failed to create TypeScript language: %v", err)
	}

	resolvers := tsLang.Resolvers().(core.ObjectOrientedLanguageResolvers)

	classes, err := resolvers.ResolveClasses(parseTree)
	if err != nil {
		t.Fatalf("Failed to resolve classes: %v", err)
	}

	var direction *ast.ClassDeclarationNode
	for _, class := range classes {
		if class.ClassName() == "Direction" {
			direction = class
			break
		}
	}

	if direction == nil {
		t.Fatal("Direction enum not found in resolved classes")
	}

	// Enums are not abstract
	if direction.IsAbstract() {
		t.Error("Direction enum should not be marked as abstract")
	}
}

// Helper functions

func testTypescriptClassResolution(t *testing.T, fixturePath string, expectedClasses map[string][]string) {
	parseTree := parseTypescriptFixtureFile(t, fixturePath)

	tsLang, err := lang.NewTypescriptLanguage()
	if err != nil {
		t.Fatalf("Failed to create TypeScript language: %v", err)
	}

	resolvers := tsLang.Resolvers().(core.ObjectOrientedLanguageResolvers)

	classes, err := resolvers.ResolveClasses(parseTree)
	if err != nil {
		t.Fatalf("Failed to resolve classes: %v", err)
	}

	foundClasses := make(map[string][]string)
	for _, class := range classes {
		foundClasses[class.ClassName()] = class.BaseClasses()
	}

	for expectedName, expectedBases := range expectedClasses {
		if foundBases, exists := foundClasses[expectedName]; exists {
			if len(foundBases) != len(expectedBases) {
				t.Errorf("Class %s: expected %d base classes %v, got %d: %v",
					expectedName, len(expectedBases), expectedBases, len(foundBases), foundBases)
				continue
			}

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

	for foundName := range foundClasses {
		if _, expected := expectedClasses[foundName]; !expected {
			t.Logf("Note: Found unexpected class %s (may be from inner classes)", foundName)
		}
	}
}

// tsFixtureVisitor is a visitor for finding specific fixture files
type tsFixtureVisitor struct {
	targetFilename string
	foundTree      *core.ParseTree
}

func (v *tsFixtureVisitor) VisitTree(tree core.ParseTree) error {
	file, err := tree.File()
	if err != nil {
		return err
	}

	if filepath.Base(file.Name()) == v.targetFilename {
		*v.foundTree = tree
	}
	return nil
}

func parseTypescriptFixtureFile(t *testing.T, relativePath string) core.ParseTree {
	fixtureDir := filepath.Join(".", relativePath)

	fileSystem, err := fs.NewLocalFileSystem(fs.LocalFileSystemConfig{
		AppDirectories: []string{filepath.Dir(fixtureDir)},
	})
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	tsLang, err := lang.NewTypescriptLanguage()
	if err != nil {
		t.Fatalf("Failed to create TypeScript language: %v", err)
	}

	walker, err := fs.NewSourceWalker(fs.SourceWalkerConfig{}, []core.Language{tsLang})
	if err != nil {
		t.Fatalf("Failed to create walker: %v", err)
	}

	treeWalker, err := parser.NewWalkingParser(walker, []core.Language{tsLang})
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}

	var parseTree core.ParseTree
	visitor := &tsFixtureVisitor{
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
