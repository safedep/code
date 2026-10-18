package ts_test

import (
	"context"
	"testing"

	"github.com/safedep/code/lang"
	"github.com/safedep/code/parser"
	"github.com/safedep/code/pkg/ts"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parseCSharp(t testing.TB, source string) *sitter.Node {
	t.Helper()
	l, err := lang.NewCSharpLanguage()
	require.NoError(t, err)
	tree, err := parser.ParseBytes(context.Background(), l, []byte(source), parser.WithMaxBytes(-1))
	require.NoError(t, err)
	t.Cleanup(tree.Close)
	return tree.Tree().RootNode()
}

func TestNodeHelpers(t *testing.T) {
	root := parseCSharp(t, "class A { public static void M() {} void N() {} }")
	class := ts.ChildOfType(root, "class_declaration")
	require.NotNil(t, class)
	body := class.ChildByFieldName("body")
	require.NotNil(t, body)
	methods := ts.NamedChildren(body)
	require.Len(t, methods, 2)

	assert.True(t, ts.HasModifier(methods[0], "static"))
	assert.False(t, ts.HasModifier(methods[1], "static"))
	assert.True(t, ts.HasChild(class, "class"), "the class keyword is a child")
	assert.True(t, ts.HasKeyword(class, "class"))
	assert.False(t, ts.HasKeyword(class, "identifier"), "a named child is no keyword")
	assert.Nil(t, ts.ChildOfType(nil, "x"))
	assert.Nil(t, ts.ChildOfType(class, "no_such_type"))
	assert.True(t, ts.SameNode(class, ts.ChildOfType(root, "class_declaration")))
	assert.False(t, ts.SameNode(methods[0], methods[1]))
}
