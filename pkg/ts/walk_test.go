package ts_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/safedep/code/core"
	"github.com/safedep/code/lang"
	"github.com/safedep/code/parser"
	"github.com/safedep/code/pkg/ts"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parseJS(t testing.TB, source string) core.ParseTree {
	l, err := lang.NewJavascriptLanguage()
	require.NoError(t, err)
	tree, err := parser.ParseBytes(context.Background(), l, []byte(source), parser.WithMaxBytes(-1))
	require.NoError(t, err)
	return tree
}

func types(t *testing.T, root *sitter.Node, opts ts.WalkOptions, skip string) []string {
	var out []string
	err := ts.Walk(context.Background(), root, opts, func(n *sitter.Node) bool {
		out = append(out, n.Type())
		return n.Type() != skip
	})
	require.NoError(t, err)
	return out
}

func TestWalk(t *testing.T) {
	root := parseJS(t, "f(a, b)").Tree().RootNode()

	assert.Equal(t,
		[]string{"program", "expression_statement", "call_expression", "identifier", "arguments", "identifier", "identifier"},
		types(t, root, ts.WalkOptions{}, ""))
	assert.Equal(t,
		[]string{"program", "expression_statement", "call_expression", "identifier", "arguments"},
		types(t, root, ts.WalkOptions{}, "arguments"))
	assert.Contains(t, types(t, root, ts.WalkOptions{Anonymous: true}, ""), ",")
}

func TestWalkSubtreeStaysInside(t *testing.T) {
	root := parseJS(t, "f(a); g(b)").Tree().RootNode()
	first := root.NamedChild(0)
	assert.Equal(t,
		[]string{"expression_statement", "call_expression", "identifier", "arguments", "identifier"},
		types(t, first, ts.WalkOptions{}, ""))
}

func TestWalkNodeLimit(t *testing.T) {
	root := parseJS(t, "f(a, b)").Tree().RootNode()
	n := 0
	err := ts.Walk(context.Background(), root, ts.WalkOptions{MaxNodes: 3}, func(*sitter.Node) bool {
		n++
		return true
	})
	assert.ErrorIs(t, err, ts.ErrNodeLimit)
	assert.Equal(t, 3, n)
}

func TestWalkStopsOnDoneContext(t *testing.T) {
	root := parseJS(t, strings.Repeat("a;", 10_000)).Tree().RootNode()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := ts.Walk(ctx, root, ts.WalkOptions{}, func(*sitter.Node) bool { return true })
	assert.ErrorIs(t, err, context.Canceled)
}

// TestWalkHostileShapes checks that a walk stays linear on a wide node and
// on a deep tree. A loop over NamedChild(i) takes minutes on the wide node,
// and a recursive walk can exhaust the stack on the deep tree.
func TestWalkHostileShapes(t *testing.T) {
	if testing.Short() {
		t.Skip("parses large inputs")
	}
	shapes := map[string]string{
		"wide":   "f(" + strings.Repeat("a,", 100_000) + ")",
		"deep":   strings.Repeat("(", 20_000) + "a" + strings.Repeat(")", 20_000),
		"chain":  "x" + strings.Repeat("()", 100_000),
		"nested": strings.Repeat("[", 20_000) + strings.Repeat("]", 20_000),
	}
	for name, source := range shapes {
		t.Run(name, func(t *testing.T) {
			root := parseJS(t, source).Tree().RootNode()
			start := time.Now()
			n := 0
			require.NoError(t, ts.Walk(context.Background(), root, ts.WalkOptions{MaxNodes: -1}, func(*sitter.Node) bool {
				n++
				return true
			}))
			assert.Positive(t, n)
			assert.Less(t, time.Since(start), 20*time.Second)
		})
	}
}

func TestNamedChildrenOfAWideNode(t *testing.T) {
	if testing.Short() {
		t.Skip("parses a large input")
	}
	root := parseJS(t, "f("+strings.Repeat("a,", 100_000)+"a)").Tree().RootNode()
	args := root.NamedChild(0).NamedChild(0).ChildByFieldName("arguments")
	start := time.Now()
	assert.Len(t, ts.NamedChildren(args), 100_001)
	assert.Less(t, time.Since(start), 10*time.Second)
}

func TestWalkCountsAnonymousNodes(t *testing.T) {
	root := parseJS(t, "["+strings.Repeat(",", 1000)+"]").Tree().RootNode()
	err := ts.Walk(context.Background(), root, ts.WalkOptions{MaxNodes: 100}, func(*sitter.Node) bool { return true })
	assert.ErrorIs(t, err, ts.ErrNodeLimit)
}

func TestNamedChildren(t *testing.T) {
	root := parseJS(t, "f(a, /* c */ b)").Tree().RootNode()
	args := root.NamedChild(0).NamedChild(0).ChildByFieldName("arguments")
	var got []string
	for _, c := range ts.NamedChildren(args) {
		got = append(got, c.Type())
	}
	assert.Equal(t, []string{"identifier", "comment", "identifier"}, got)
	assert.Nil(t, ts.NamedChildren(nil))
}

func TestDiagnose(t *testing.T) {
	cases := []struct {
		name     string
		source   string
		hasError bool
		errors   int
		missing  int
		kinds    []string
		absent   []string
	}{
		{name: "clean", source: "f(a)", kinds: []string{"call_expression", "identifier"}, absent: []string{"ERROR", "jsx_element"}},
		{name: "jsx", source: "<svg><g/></svg>", kinds: []string{"jsx_element", "jsx_self_closing_element"}},
		{name: "missing brace", source: "function f() {", hasError: true, missing: 1},
		{name: "error text", source: "a b c d @@@", hasError: true, errors: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := parseJS(t, tc.source).Tree().RootNode()
			d, err := ts.Diagnose(context.Background(), root, 0)
			require.NoError(t, err)
			assert.Equal(t, tc.hasError, d.HasError)
			assert.Equal(t, tc.missing, d.MissingCount)
			assert.Len(t, d.Missing, tc.missing)
			if tc.errors > 0 {
				assert.GreaterOrEqual(t, d.ErrorCount, tc.errors)
				require.NotEmpty(t, d.Errors)
				assert.Equal(t, uint32(1), d.Errors[0].StartLine)
			} else {
				assert.Zero(t, d.ErrorCount)
			}
			for _, k := range tc.kinds {
				assert.True(t, d.Kinds[k], k)
			}
			for _, k := range tc.absent {
				assert.False(t, d.Kinds[k], k)
			}
			assert.Positive(t, d.Nodes)
		})
	}
}

func TestDiagnoseKeepsFewPositions(t *testing.T) {
	root := parseJS(t, strings.Repeat("let a = {\n", 150)).Tree().RootNode()
	d, err := ts.Diagnose(context.Background(), root, 0)
	require.NoError(t, err)
	assert.Greater(t, d.ErrorCount, 100)
	assert.Len(t, d.Errors, 100)
}

func FuzzDiagnose(f *testing.F) {
	for _, s := range []string{"", "f(a)", "function f() {", "<svg/>", "@@@", "x\x00y"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, source string) {
		root := parseJS(t, source).Tree().RootNode()
		d, err := ts.Diagnose(context.Background(), root, 0)
		require.NoError(t, err)
		assert.Equal(t, d.HasError, d.ErrorCount+d.MissingCount > 0)
	})
}
