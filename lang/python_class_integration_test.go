package lang

import (
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
)

func TestPythonLanguageImplementsObjectOrientedResolvers(t *testing.T) {
	lang, err := NewPythonLanguage()
	if err != nil {
		t.Fatalf("Failed to create Python language: %v", err)
	}
	resolvers := lang.Resolvers()

	// Test that Python resolvers implement ObjectOrientedLanguageResolvers
	_, ok := resolvers.(core.ObjectOrientedLanguageResolvers)
	if !ok {
		t.Error("Python resolvers should implement ObjectOrientedLanguageResolvers interface")
	}

	// Test that the language is marked as object-oriented
	meta := lang.Meta()
	if !meta.ObjectOriented {
		t.Error("Python language should be marked as object-oriented")
	}
}

func TestPythonResolversInterfaceCompliance(t *testing.T) {
	lang, err := NewPythonLanguage()
	if err != nil {
		t.Fatalf("Failed to create Python language: %v", err)
	}
	resolvers := lang.Resolvers()

	// Test basic LanguageResolvers interface
	var _ core.LanguageResolvers = resolvers

	// Test ObjectOrientedLanguageResolvers interface
	ooResolvers, ok := resolvers.(core.ObjectOrientedLanguageResolvers)
	if !ok {
		t.Fatal("Python resolvers should implement ObjectOrientedLanguageResolvers")
	}

	// Test that methods exist and have the expected signatures
	// We can't call them with nil without causing panics, but we can test
	// that the interface is properly implemented

	// This will compile only if the interface is properly implemented
	var _ func(core.ParseTree) ([]*ast.ClassDeclarationNode, error) = ooResolvers.ResolveClasses
	var _ func(core.ParseTree) (*ast.InheritanceGraph, error) = ooResolvers.ResolveInheritance
}

func TestHelperMethods(t *testing.T) {
	resolvers := &pythonResolvers{}

	// Test extractBaseClassNodes with nil input
	result := resolvers.extractBaseClassNodes(nil)
	if len(result) != 0 {
		t.Error("extractBaseClassNodes should return empty slice for nil input")
	}

	// Test findParentClassName with nil input
	className := resolvers.findParentClassName(nil, []byte("test"))
	if className != "" {
		t.Error("findParentClassName should return empty string for nil input")
	}

	// Test findFollowingClassName with nil input
	className = resolvers.findFollowingClassName(nil, []byte("test"))
	if className != "" {
		t.Error("findFollowingClassName should return empty string for nil input")
	}
}
