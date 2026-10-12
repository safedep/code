package ts

import (
	"context"

	"github.com/safedep/code/core/ast"
	sitter "github.com/smacker/go-tree-sitter"
)

// maxProblemPositions is the number of positions that Diagnostics keeps for
// each problem kind. The counts hold all of them.
const maxProblemPositions = 100

// Diagnostics describe how well a tree parsed and what it holds.
type Diagnostics struct {
	// HasError reports an ERROR or a MISSING node anywhere in the tree.
	HasError bool

	// Errors are the positions of the ERROR nodes: text that the grammar
	// could not place. ErrorCount counts all of them.
	Errors     []ast.NodePosition
	ErrorCount int

	// Missing are the positions of the MISSING nodes: tokens that the
	// parser added to recover, such as a closing brace. MissingCount
	// counts all of them.
	Missing      []ast.NodePosition
	MissingCount int

	// Kinds holds the type of each named node, such as call_expression or
	// jsx_element.
	Kinds map[string]bool

	// Nodes is the number of nodes that the walk visited.
	Nodes int
}

// Diagnose walks the tree from root and reports its parse problems and its
// node kinds. maxNodes is the node limit of the walk, as WalkOptions.MaxNodes
// sets it. On ErrNodeLimit or a done ctx it returns the diagnostics of the
// nodes before the stop with the error.
func Diagnose(ctx context.Context, root *sitter.Node, maxNodes int) (Diagnostics, error) {
	d := Diagnostics{Kinds: map[string]bool{}}
	if root == nil {
		return d, nil
	}
	d.HasError = root.HasError()
	// A MISSING node can be anonymous, as a missing semicolon.
	err := Walk(ctx, root, WalkOptions{MaxNodes: maxNodes, Anonymous: true}, func(n *sitter.Node) bool {
		d.Nodes++
		switch {
		case n.IsMissing():
			d.MissingCount++
			if len(d.Missing) < maxProblemPositions {
				d.Missing = append(d.Missing, ast.GetNodePosition(n))
			}
		case n.IsError():
			d.ErrorCount++
			if len(d.Errors) < maxProblemPositions {
				d.Errors = append(d.Errors, ast.GetNodePosition(n))
			}
		}
		if n.IsNamed() {
			d.Kinds[n.Type()] = true
		}
		return true
	})
	return d, err
}
