package ts

import (
	"context"
	"errors"
	"fmt"

	sitter "github.com/smacker/go-tree-sitter"
)

// DefaultMaxNodes is the number of nodes that Walk reaches when the caller
// sets no limit. go-tree-sitter keeps a Go value for each node that a walk
// reaches until the tree is gone, so the limit also bounds the memory. A
// walk of a million nodes takes a few seconds.
const DefaultMaxNodes = 1_000_000

// ErrNodeLimit is the error of a walk that stopped at its node limit.
var ErrNodeLimit = errors.New("walk reached the node limit")

// ctxCheckInterval is the number of nodes between two checks of the context.
const ctxCheckInterval = 4096

// WalkOptions bound a walk.
type WalkOptions struct {
	// MaxNodes is the largest number of nodes that the walk reaches, named
	// or anonymous. The cursor reaches each anonymous node, as a comma, also
	// when the walk does not visit it. Zero selects DefaultMaxNodes. A
	// negative value removes the limit.
	MaxNodes int

	// Anonymous makes the walk visit anonymous nodes too, such as the
	// punctuation and the keywords.
	Anonymous bool
}

// Visit is called for each node of a walk. It returns false to skip the
// children of the node.
type Visit func(*sitter.Node) bool

// Walk visits root and its descendants in source order, a parent before
// its children. It uses a tree cursor, so its time is linear in the number
// of nodes, and it uses no recursion, so a deep tree cannot exhaust the
// stack. A loop over NamedChild(i) is quadratic in the number of children,
// because tree-sitter finds child i from the first child.
//
// Walk returns ErrNodeLimit at the node limit, and the error of ctx when
// ctx is done. The nodes before the stop were visited.
func Walk(ctx context.Context, root *sitter.Node, opts WalkOptions, visit Visit) error {
	if root == nil {
		return nil
	}
	limit := opts.MaxNodes
	if limit == 0 {
		limit = DefaultMaxNodes
	}

	c := sitter.NewTreeCursor(root)
	defer c.Close()

	reached := 0
	for {
		if limit > 0 && reached >= limit {
			return fmt.Errorf("%w of %d", ErrNodeLimit, limit)
		}
		reached++
		if reached%ctxCheckInterval == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		n := c.CurrentNode()
		descend := true
		if opts.Anonymous || n.IsNamed() {
			descend = visit(n)
		}
		if descend && c.GoToFirstChild() {
			continue
		}
		for !c.GoToNextSibling() {
			if !c.GoToParent() {
				return nil
			}
		}
	}
}

// NamedChildren returns the named children of n in order, in time linear in
// the number of children.
func NamedChildren(n *sitter.Node) []*sitter.Node {
	if n == nil || n.NamedChildCount() == 0 {
		return nil
	}
	out := make([]*sitter.Node, 0, n.NamedChildCount())
	c := sitter.NewTreeCursor(n)
	defer c.Close()
	for ok := c.GoToFirstChild(); ok; ok = c.GoToNextSibling() {
		if child := c.CurrentNode(); child.IsNamed() {
			out = append(out, child)
		}
	}
	return out
}
