package depsusage

import (
	"context"
	"fmt"

	"github.com/safedep/code/core"
	"github.com/safedep/code/pkg/helpers"
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
}

func (p *dependencyUsagePlugin) SupportedLanguages() []core.LanguageCode {
	return supportedLanguages
}

var usageEvidentNodeTypes = map[string]bool{
	"identifier":      true,
	"type_identifier": true,
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

	imports, err := lang.Resolvers().ResolveImports(tree)
	if err != nil {
		return fmt.Errorf("failed to resolve imports: %w", err)
	}

	moduleIdentifiers := make(map[string]*identifierItem)
	for _, imp := range imports {
		importContents, err := helpers.ResolveImportContents(imp, lang)
		if err != nil {
			return fmt.Errorf("failed to resolve import contents: %w", err)
		}

		packageHint, err := resolvePackageHint(importContents.ModuleName, lang)
		if err != nil {
			log.Debugf("failed to resolve package hint: %s", err)
		}

		if imp.IsWildcardImport() {
			// @TODO - This is false positive case for wildcard imports
			// If it is a wildcard import, mark the module as used by default
			evidence := newUsageEvidence(packageHint, importContents.ModuleName, importContents.ModuleItem, importContents.ModuleAlias, true, "", file.Name(), uint(imp.GetModuleNameNode().StartPoint().Row)+1)
			if err := p.usageCallback(ctx, evidence); err != nil {
				return fmt.Errorf("failed to call usage callback for wildcard import: %w", err)
			}
		} else {
			identifierKey := helpers.GetFirstNonEmptyString(importContents.ModuleAlias, importContents.ModuleItem, importContents.ModuleName)
			moduleIdentifiers[identifierKey] = newIdentifierItem(importContents.ModuleName, importContents.ModuleItem, importContents.ModuleAlias, identifierKey, packageHint)
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

	cursor := sitter.NewTreeCursor(tree.Tree().RootNode())
	defer cursor.Close()

	err = traverse(cursor, &treeLanguage, treeData, func(n *sitter.Node) error {
		nodeType := n.Type()

		if _, usageEvidentNode := usageEvidentNodeTypes[nodeType]; usageEvidentNode {
			identifierKey := n.Content(*treeData)
			identifiedItem, identifierKeyExists := moduleIdentifiers[identifierKey]
			if identifierKeyExists {
				evidence := newUsageEvidence(identifiedItem.PackageHint, identifiedItem.Module, identifiedItem.Item, identifiedItem.Alias, false, identifierKey, file.Name(), uint(n.StartPoint().Row)+1)
				if err := p.usageCallback(ctx, evidence); err != nil {
					return fmt.Errorf("failed to call usage callback: %w", err)
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func traverse(cursor *sitter.TreeCursor, treeLanguage *core.Language, treeData *[]byte, visit func(node *sitter.Node) error) error {
	for {
		// Call the visit function for the current node
		err := visit(cursor.CurrentNode())
		if err != nil {
			return err
		}

		if !isIgnoredNode(cursor.CurrentNode(), treeLanguage, treeData) {
			// Try going to the first child
			if cursor.GoToFirstChild() {
				continue
			}
		}

		// If no children, try the next sibling
		for !cursor.GoToNextSibling() {
			// If no siblings, go to the parent and repeat
			if !cursor.GoToParent() {
				return nil // Exit traversal if back at the root
			}
		}
	}
}
