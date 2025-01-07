package depsusage

import (
	"context"
	"fmt"
	"strings"

	"github.com/safedep/code/core"
	"github.com/safedep/code/pkg/helpers"
	"github.com/safedep/dry/log"
	sitter "github.com/smacker/go-tree-sitter"
)

type DependencyUsageCallback func(*UsageEvidence) error

type dependencyUsagePlugin struct {
	// Callback function which is called with the usage evidence
	usageCallback DependencyUsageCallback
}

// Verify contract
var _ core.TreePlugin = (*dependencyUsagePlugin)(nil)

func NewDependencyUsagePlugin(usageCallback DependencyUsageCallback) *dependencyUsagePlugin {
	return &dependencyUsagePlugin{
		usageCallback: usageCallback,
	}
}

func (p *dependencyUsagePlugin) Name() string {
	return "DependencyUsagePlugin"
}

var supportedLanguages = []core.LanguageCode{core.LanguageCodePython}

func (p *dependencyUsagePlugin) SupportedLanguages() []core.LanguageCode {
	return supportedLanguages
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
		baseModuleName := GetBaseModuleName(imp.ModuleName())

		itemName := imp.ModuleItem()

		// In case of wildcard imports, ModuleItem isn't explicitly specified hence derived from modulename
		if imp.IsWildcardImport() {
			itemName = imp.ModuleName()
		}

		// Remove the base module name (if present) from item name
		if strings.HasPrefix(itemName, baseModuleName+".") {
			itemName = itemName[len(baseModuleName)+1:]
		}

		if imp.IsWildcardImport() {
			// @TODO - This is false positive case for wildcard imports
			// If it is a wildcard import, mark the module as used by default
			evidence := newUsageEvidence(baseModuleName, wildcardIdentifier, wildcardIdentifier, itemName, file.Name(), uint(imp.GetModuleNameNode().StartPoint().Row)+1, true)
			if err := p.usageCallback(evidence); err != nil {
				return err
			}
			continue
		}
		identifierKey := helpers.GetFirstNonEmptyString(imp.ModuleAlias(), imp.ModuleItem(), imp.ModuleName())
		moduleIdentifiers[identifierKey] = newIdentifierItem(baseModuleName, identifierKey, imp.ModuleAlias(), itemName)
	}

	treeData, err := tree.Data()
	if err != nil {
		return fmt.Errorf("failed to get tree data: %w", err)
	}

	cursor := sitter.NewTreeCursor(tree.Tree().RootNode())
	defer cursor.Close()

	traverse(cursor, func(n *sitter.Node) error {
		nodeType := n.Type()
		content := n.Content(*treeData)
		identifierKey := string(content)
		identifier, exists := moduleIdentifiers[identifierKey]

		if nodeType == "identifier" && exists {
			evidence := newUsageEvidence(identifier.Module, identifier.Identifier, identifier.Alias, identifier.ItemName, file.Name(), uint(n.StartPoint().Row)+1, false)
			if err := p.usageCallback(evidence); err != nil {
				return err
			}
		}

		return nil
	})
	return nil
}

func traverse(cursor *sitter.TreeCursor, visit func(node *sitter.Node) error) error {
	for {
		// Call the visit function for the current node
		err := visit(cursor.CurrentNode())
		if err != nil {
			return err
		}

		// No need to traverse inside if the node is of an ignored type
		if _, ignored := ignoredTypes[cursor.CurrentNode().Type()]; !ignored {
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
