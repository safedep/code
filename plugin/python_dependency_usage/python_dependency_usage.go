package python_dependency_usage

import (
	"context"
	"fmt"
	"strings"

	"github.com/safedep/code/core"
	sitter "github.com/smacker/go-tree-sitter"
)

type DependencyUsagePlugin struct{}

// Verify contract
var _ core.TreePlugin = (*DependencyUsagePlugin)(nil)

func (p *DependencyUsagePlugin) Name() string {
	return "DependencyUsagePlugin"
}

func (p *DependencyUsagePlugin) AnalyzeTree(ctx context.Context, tree core.ParseTree) error {
	lang, err := tree.Language()
	if err != nil {
		return fmt.Errorf("failed to get language: %w", err)
	}

	if lang.Meta().Name != "python" {
		return nil
	}

	file, err := tree.File()
	if err != nil {
		return fmt.Errorf("failed to get file: %w", err)
	}

	imports, err := lang.Resolvers().ResolveImports(tree)
	if err != nil {
		return fmt.Errorf("failed to resolve imports: %w", err)
	}

	fmt.Println()
	fmt.Printf("All imports in %s --------------------------------- \n", file.Name())
	for _, imp := range imports {
		fmt.Println(imp)
	}
	fmt.Println("------------------------------------------------------------------------------------")
	fmt.Println()

	moduleUsageStatus := make(map[string]ImportModule)
	moduleIdentifierKeys := make(map[string]string)
	for _, imp := range imports {
		// Extract the base name of the module
		baseName := getBaseModuleName(imp.ModuleName())

		// If the module isn't in the map, initialize it
		if _, exists := moduleUsageStatus[baseName]; !exists {
			moduleUsageStatus[baseName] = ImportModule{
				Used:        false,
				Identifiers: []IdentifierItem{},
			}
		}

		if imp.IsWildcardImport() {
			// @TODO - This is false positive case for wildcard imports
			// If it is a wildcard import, mark the module as used by default
			module := moduleUsageStatus[baseName]
			module.Used = true
			moduleUsageStatus[baseName] = module
			continue
		}

		// Add identifier to the module
		identifierKey := getFirstNonEmptyString(imp.ModuleAlias(), imp.ModuleItem(), imp.ModuleName())

		itemName := imp.ModuleItem()

		// remove basename from any submodular imports
		if strings.HasPrefix(itemName, baseName+".") {
			itemName = itemName[len(baseName)+1:]
		}

		module := moduleUsageStatus[baseName]
		module.Identifiers = append(module.Identifiers, IdentifierItem{
			Identifier: identifierKey,
			Alias:      imp.ModuleAlias(),
			ItemName:   itemName,
		})
		moduleUsageStatus[baseName] = module
		moduleIdentifierKeys[identifierKey] = baseName
	}
	usageDiagnostics := UsageDiagnostics{
		moduleUsageStatus:    moduleUsageStatus,
		moduleIdentifierKeys: moduleIdentifierKeys,
	}

	fmt.Println("Imported Module map -")
	for module, details := range moduleUsageStatus {
		fmt.Printf("Module:\t%s\n", module)
		for _, item := range details.Identifiers {
			fmt.Printf("\tIdentifier: %s, Alias: %s, Item: %s\n", item.Identifier, item.Alias, item.ItemName)
		}
	}
	fmt.Println()

	fmt.Println("Usage Identifier Key -> Module mapping -")
	for key, module := range moduleIdentifierKeys {
		fmt.Printf("%s => %s\n", key, module)
	}
	fmt.Println()

	data, err := tree.Data()
	if err != nil {
		return fmt.Errorf("failed to get tree data: %w", err)
	}

	// fmt.Println("Code:", string(*data))

	// Analyze the AST
	analyzeAST(tree.Tree().RootNode(), *data, &usageDiagnostics)

	return nil
}

func analyzeAST(node *sitter.Node, code []byte, usageDiagnostics *UsageDiagnostics) {
	cursor := sitter.NewTreeCursor(node)
	defer cursor.Close()

	// fmt.Println("Traverse --------------------------------------------------------------------------")
	traverse(cursor, func(n *sitter.Node) {
		// if n.Type() == "module" {
		// 	fmt.Printf("Visit %s \n", cursor.CurrentNode().Type())
		// } else {
		// 	fmt.Printf("Visit %s -> `%s`\n", cursor.CurrentNode().Type(), cursor.CurrentNode().Content(code))
		// }

		nodeType := n.Type()
		content := n.Content(code)

		if nodeType == "identifier" {
			identifier := string(content)
			usageDiagnostics.setUsed(identifier)
		}

		// // @TODO - Is it needed ?
		// // If the node is a function call or attribute, check if the base identifier matches
		// if nodeType == "call" || nodeType == "attribute" {
		// 	identifier := string(content)
		// 	// Check for base identifier (e.g., sys.exit, 3plib.doSomething)
		// 	identifierParts := strings.Split(identifier, ".")
		// 	if len(identifierParts) > 1 {
		// 		baseIdentifier := identifierParts[0]

		// 		if used := usageDiagnostics.setUsed(baseIdentifier); used {
		// 			fmt.Printf("Function call or attribute access on base identifier '%s' is used.\n", baseIdentifier)
		// 		}
		// 	}
		// }
	})

	fmt.Println("Module usage status -")
	sortedKeys := getSortedKeys(usageDiagnostics.moduleUsageStatus, func(a, b string) bool { return a < b })
	for _, module := range sortedKeys {
		details := usageDiagnostics.moduleUsageStatus[module]
		fmt.Printf("Module: %s => %t\n", module, details.Used)
	}
	fmt.Println()
}

func traverse(cursor *sitter.TreeCursor, visit func(node *sitter.Node)) {
	for {
		// Call the visit function for the current node
		visit(cursor.CurrentNode())

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
				return // Exit traversal if back at the root
			}
		}
	}
}
