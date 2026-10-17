package depsusage

import (
	"context"
	"fmt"

	"github.com/safedep/code/core"
	"github.com/safedep/code/pkg/helpers"
	"github.com/safedep/code/pkg/ts"
	"github.com/safedep/dry/log"
	sitter "github.com/smacker/go-tree-sitter"
)

type DependencyUsageCallback core.PluginCallback[*UsageEvidence]

type dependencyUsagePlugin struct {
	// Callback function which is called with the usage evidence
	usageCallback DependencyUsageCallback
}

// Verify contract
var _ core.TreePlugin = (*dependencyUsagePlugin)(nil)

// depsusage plugin collects the usage evidence for the imported dependencies.
// It uses tree-sitter to parse the imported dependency-identifier relations in the
// source code and verify the usage of dependencies based on identifier usage.
func NewDependencyUsagePlugin(usageCallback DependencyUsageCallback) *dependencyUsagePlugin {
	return &dependencyUsagePlugin{
		usageCallback: usageCallback,
	}
}

func (p *dependencyUsagePlugin) Name() string {
	return "DependencyUsagePlugin"
}

var supportedLanguages = []core.LanguageCode{
	core.LanguageCodePython,
	core.LanguageCodeGo,
	core.LanguageCodeJavascript,
	core.LanguageCodeJava,
	core.LanguageCodeTypescript,
	core.LanguageCodeCSharp,
	core.LanguageCodeRust,
	core.LanguageCodePHP,
	core.LanguageCodeRuby,
	core.LanguageCodeKotlin,
}

func (p *dependencyUsagePlugin) SupportedLanguages() []core.LanguageCode {
	return supportedLanguages
}

var usageEvidentNodeTypes = map[string]bool{
	"identifier":      true,
	"type_identifier": true,
}

// languageUsageEvidentNodeTypes replaces usageEvidentNodeTypes for a grammar
// with other names for identifier nodes.
var languageUsageEvidentNodeTypes = map[core.LanguageCode]map[string]bool{
	core.LanguageCodePHP:    {"name": true},
	core.LanguageCodeKotlin: {"simple_identifier": true, "type_identifier": true},
}

func usageEvidentNodeTypesOf(code core.LanguageCode) map[string]bool {
	if nodeTypes, ok := languageUsageEvidentNodeTypes[code]; ok {
		return nodeTypes
	}
	return usageEvidentNodeTypes
}

func (p *dependencyUsagePlugin) AnalyzeTree(ctx context.Context, tree core.ParseTree) error {
	lang, err := tree.Language()
	if err != nil {
		return fmt.Errorf("failed to get language: %w", err)
	}

	file, err := tree.File()
	if err != nil {
		return fmt.Errorf("failed to get file: %w", err)
	}

	log.Debugf("depsusage - Analyzing tree for language: %s, file: %s\n",
		lang.Meta().Code, file.Name())

	imports, err := lang.Resolvers().ResolveImports(ctx, tree)
	if err != nil {
		return fmt.Errorf("failed to resolve imports: %w", err)
	}

	moduleIdentifiers := make(map[string]*identifierItem)
	for _, imp := range imports {
		moduleName, moduleItem, moduleAlias := imp.ModuleName(), imp.ModuleItem(), imp.ModuleAlias()

		packageHint, err := resolvePackageHint(moduleName, lang)
		if err != nil {
			log.Debugf("failed to resolve package hint: %s", err)
		}

		if imp.IsWildcardImport() {
			// @TODO - This is false positive case for wildcard imports
			// If it is a wildcard import, mark the module as used by default
			evidence := newUsageEvidence(packageHint, moduleName, moduleItem, moduleAlias, true, "", file.Name(), uint(imp.GetModuleNameNode().StartPoint().Row)+1)
			if err := p.usageCallback(ctx, evidence); err != nil {
				return fmt.Errorf("failed to call usage callback for wildcard import: %w", err)
			}
		} else {
			identifierKey := helpers.GetFirstNonEmptyString(moduleAlias, moduleItem, moduleName)
			moduleIdentifiers[identifierKey] = newIdentifierItem(moduleName, moduleItem, moduleAlias, identifierKey, packageHint)
		}
	}

	treeData, err := tree.Data()
	if err != nil {
		return fmt.Errorf("failed to get tree data: %w", err)
	}

	treeLanguage, err := tree.Language()
	if err != nil {
		return fmt.Errorf("failed to get tree language: %w", err)
	}

	evidentNodeTypes := usageEvidentNodeTypesOf(lang.Meta().Code)

	// The walk collects the evidence and the callbacks run after it, because
	// a visit cannot stop the walk with the error of a callback.
	var evidences []*UsageEvidence
	err = ts.Walk(ctx, tree.Tree().RootNode(), ts.WalkOptions{}, func(n *sitter.Node) bool {
		if evidentNodeTypes[n.Type()] {
			identifierKey := n.Content(*treeData)
			if identifiedItem, ok := moduleIdentifiers[identifierKey]; ok {
				evidences = append(evidences, newUsageEvidence(identifiedItem.PackageHint, identifiedItem.Module, identifiedItem.Item, identifiedItem.Alias, false, identifierKey, file.Name(), uint(n.StartPoint().Row)+1))
			}
		}
		return !isIgnoredNode(n, &treeLanguage, treeData)
	})
	if err != nil {
		return fmt.Errorf("failed to walk the tree: %w", err)
	}

	for _, evidence := range evidences {
		if err := p.usageCallback(ctx, evidence); err != nil {
			return fmt.Errorf("failed to call usage callback: %w", err)
		}
	}
	return nil
}
