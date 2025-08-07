package scan

import (
	"fmt"
	"slices"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
	"github.com/safedep/code/examples/astdb/ent"
	"github.com/safedep/code/examples/astdb/ent/astnode"
)

func (fp *fileProcessor) extractAndPersistASTNodes(tree core.ParseTree, fileRecord *ent.File) error {
	// Get the root node from the tree
	sitterTree := tree.Tree()
	if sitterTree == nil {
		return fmt.Errorf("no tree-sitter tree available")
	}

	root := sitterTree.RootNode()
	if root == nil {
		return fmt.Errorf("no root node available in parse tree")
	}

	// Extract AST nodes recursively starting from root
	_, err := fp.extractASTNode(root, nil, fileRecord, 0)
	return err
}

func (fp *fileProcessor) extractASTNode(node *sitter.Node, parentRecord *ent.ASTNode,
	fileRecord *ent.File, depth int) (*ent.ASTNode, error) {
	if node == nil {
		return nil, nil
	}

	// Get position information without content (following user's guidance)
	position := ast.GetNodePosition(node)
	nodeType := ast.GetNodeType(node)

	// Determine semantic node type
	semanticType := fp.getSemanticNodeType(nodeType)

	// Create AST node record
	nodeBuilder := fp.db.ASTNode.Create().
		SetNodeType(astnode.NodeType(semanticType)).
		SetStartLine(int(position.StartLine)).
		SetEndLine(int(position.EndLine)).
		SetStartColumn(int(position.StartColumn)).
		SetEndColumn(int(position.EndColumn)).
		SetTreeSitterType(nodeType).
		SetFileID(fileRecord.ID)

	// Set parent relationship if exists
	if parentRecord != nil {
		nodeBuilder = nodeBuilder.SetParentID(parentRecord.ID)
	}

	// Add metadata with byte offset information
	metadata := map[string]any{
		"start_byte": position.StartByte,
		"end_byte":   position.EndByte,
		"depth":      depth,
	}

	// Extract name if applicable for certain node types
	name := fp.extractNodeName(node, nodeType)
	if name != "" {
		nodeBuilder = nodeBuilder.SetName(name)
		nodeBuilder = nodeBuilder.SetQualifiedName(name) // Basic qualified name, can be enhanced later
		metadata["has_name"] = true
	}

	nodeBuilder = nodeBuilder.SetMetadata(metadata)

	// Save the node
	astNodeRecord, err := nodeBuilder.Save(fp.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to save AST node: %w", err)
	}

	// Process child nodes
	childCount := int(node.ChildCount())
	for i := range childCount {
		child := node.Child(i)
		_, err := fp.extractASTNode(child, astNodeRecord, fileRecord, depth+1)
		if err != nil {
			// Log error but continue processing other children
			if fp.scanner.config.Verbose {
				fmt.Printf("Warning: failed to process child node %d: %v\n", i, err)
			}
		}
	}

	return astNodeRecord, nil
}

func (fp *fileProcessor) getSemanticNodeType(treeType string) string {
	// Map Tree-Sitter node types to our semantic types using constants
	switch treeType {
	case TreeSitterModule, TreeSitterSourceFile, TreeSitterProgram:
		return NodeTypeModule
	case TreeSitterClassDefinition, TreeSitterClassDeclaration:
		return NodeTypeClass
	case TreeSitterFunctionDefinition, TreeSitterFunctionDeclaration, TreeSitterMethodDefinition:
		return NodeTypeFunction
	case TreeSitterVariableDeclaration, TreeSitterAssignment, TreeSitterAssignmentStatement:
		return NodeTypeVariable
	case TreeSitterImportStatement, TreeSitterImportDeclaration, TreeSitterFromImport, TreeSitterImportFromStatement:
		return NodeTypeImport
	case TreeSitterCallExpression, TreeSitterCall:
		return NodeTypeCall
	case TreeSitterIfStatement, TreeSitterIf:
		return NodeTypeIfStatement
	case TreeSitterForStatement, TreeSitterFor, TreeSitterForInStatement:
		return NodeTypeForLoop
	case TreeSitterWhileStatement, TreeSitterWhile:
		return NodeTypeWhileLoop
	case TreeSitterTryStatement, TreeSitterTry, TreeSitterExceptClause, TreeSitterCatchClause:
		return NodeTypeTryCatch
	case TreeSitterExpression, TreeSitterExpressionStatement:
		return NodeTypeExpression
	case TreeSitterLiteral, TreeSitterStringLiteral, TreeSitterNumber, TreeSitterInteger, TreeSitterFloat:
		return NodeTypeLiteral
	case TreeSitterIdentifier, TreeSitterName:
		return NodeTypeIdentifier
	default:
		return NodeTypeExpression // Default fallback
	}
}

func (fp *fileProcessor) extractNodeName(node *sitter.Node, nodeType string) string {
	// Extract meaningful names from specific node types
	// This is a simplified version - in a full implementation,
	// this would use language-specific parsing logic

	switch nodeType {
	case TreeSitterClassDefinition, TreeSitterClassDeclaration:
		// Try to find the class name child node
		return fp.findNameInChildren(node, []string{TreeSitterName, TreeSitterIdentifier})
	case TreeSitterFunctionDefinition, TreeSitterFunctionDeclaration, TreeSitterMethodDefinition:
		// Try to find the function name child node
		return fp.findNameInChildren(node, []string{TreeSitterName, TreeSitterIdentifier})
	case TreeSitterVariableDeclaration, TreeSitterAssignment:
		// Try to find the variable name
		return fp.findNameInChildren(node, []string{TreeSitterName, TreeSitterIdentifier, TreeSitterLeft})
	case TreeSitterIdentifier, TreeSitterName:
		// For identifier nodes, we could extract the content, but following
		// user guidance to avoid storing content, we'll skip this for now
		return ""
	default:
		return ""
	}
}

func (fp *fileProcessor) findNameInChildren(node *sitter.Node, targetTypes []string) string {
	// Simple helper to find name nodes in children
	// In a real implementation, this would be more sophisticated
	childCount := int(node.ChildCount())
	for i := range childCount {
		child := node.Child(i)
		childType := child.Type()

		if slices.Contains(targetTypes, childType) {
			// Would extract content here, but following user guidance to avoid it
			// Instead, we could store a placeholder or compute it later when needed
			return "" // Placeholder - content extraction avoided per user instructions
		}
	}

	return ""
}
