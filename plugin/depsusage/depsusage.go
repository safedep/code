package depsusage

import (
	"context"
	"fmt"
	"slices"

	"github.com/safedep/code/core"
	"github.com/safedep/code/pkg/helpers"
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

	// log.Debugf("depsusage - Analyzing tree for language: %s, file: %s\n",
	// 	lang.Meta().Code, file.Name())

	imports, err := lang.Resolvers().ResolveImports(tree)
	if err != nil {
		return fmt.Errorf("failed to resolve imports: %w", err)
	}

	moduleIdentifiers := make(map[string]*identifierItem)
	for _, imp := range imports {
		packageHint := resolvePackageHint(imp.ModuleName(), lang)

		if imp.IsWildcardImport() {
			// @TODO - This is false positive case for wildcard imports
			// If it is a wildcard import, mark the module as used by default
			evidence := newUsageEvidence(packageHint, imp.ModuleName(), imp.ModuleItem(), imp.ModuleAlias(), true, "", file.Name(), uint(imp.GetModuleNameNode().StartPoint().Row)+1, "")
			if err := p.usageCallback(ctx, evidence); err != nil {
				return fmt.Errorf("failed to call usage callback for wildcard import: %w", err)
			}
		} else {
			identifierKey := helpers.GetFirstNonEmptyString(imp.ModuleAlias(), imp.ModuleItem(), imp.ModuleName())
			moduleIdentifiers[identifierKey] = newIdentifierItem(imp.ModuleName(), imp.ModuleItem(), imp.ModuleAlias(), identifierKey, packageHint)
		}
	}

	treeData, err := tree.Data()
	if err != nil {
		return fmt.Errorf("failed to get tree data: %w", err)
	}

	cursor := sitter.NewTreeCursor(tree.Tree().RootNode())
	defer cursor.Close()

	err = traverse(cursor, func(n *sitter.Node) error {
		nodeType := n.Type()
		identifierKey := n.Content(*treeData)
		identifiedItem, exists := moduleIdentifiers[identifierKey]

		if nodeType == "identifier" && exists {
			// pr := n.Parent()
			// pr2 := pr.Parent()
			// pr3 := pr2.Parent()
			// pr4 := pr3.Parent()

			// fmt.Println("Found evidence for: identifier", identifierKey)
			// fmt.Println("Parent", pr.Type(), pr.Content(*treeData))
			// fmt.Println("Parent-l2", pr2.Type(), pr2.Content(*treeData))
			// fmt.Println("Parent-l3", pr3.Type(), pr3.Content(*treeData))
			// if pr4 != nil {
			// 	fmt.Println("Parent-l4", pr4.Type(), pr4.Content(*treeData))
			// 	pr5 := pr4.Parent()
			// 	if pr5 != nil {
			// 		fmt.Println("Parent-l5", pr5.Type(), pr5.Content(*treeData))
			// 	}
			// }

			evidenceSnippet := getEvidenceSnippet(n, treeData)
			evidence := newUsageEvidence(identifiedItem.PackageHint, identifiedItem.Module, identifiedItem.Item, identifiedItem.Alias, false, identifierKey, file.Name(), uint(n.StartPoint().Row)+1, evidenceSnippet)
			if err := p.usageCallback(ctx, evidence); err != nil {
				return fmt.Errorf("failed to call usage callback: %w", err)
			}

		}
		return nil
	})
	if err != nil {
		return err
	}
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

var evidenceStatementTypes = []string{"expression_statement", "assignment", "call", "return_statement", "import_statement", "import_from_statement", "class_definition", "function_definition", "block", "module"}

func getEvidenceSnippet(node *sitter.Node, treeData *[]byte) string {
	for node != nil {
		if slices.Contains(evidenceStatementTypes, node.Type()) {
			return node.Content(*treeData)
		}
		node = node.Parent()
	}
	return ""
}
