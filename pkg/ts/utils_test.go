package ts_test

import (
	"context"
	"sync"
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/lang"
	"github.com/safedep/code/pkg/ts"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const identifierQuery = `(identifier) @name`

func countIdentifiers(t *testing.T, language core.Language, tree core.ParseTree) int {
	data, err := tree.Data()
	require.NoError(t, err)

	count := 0
	err = ts.ExecuteQueries(ts.NewQueriesRequest(language, []ts.QueryItem{
		ts.NewQueryItem(identifierQuery, func(*sitter.QueryMatch) error {
			count++
			return nil
		}),
	}), data, tree)
	require.NoError(t, err)
	return count
}

func TestExecuteQueriesSharesCompiledQueries(t *testing.T) {
	language, err := lang.NewTypescriptLanguage()
	require.NoError(t, err)

	// In x.y(z, w), y is a property_identifier, so the query matches x, z and w.
	sources := map[string]int{"const a = b(c)": 3, "x.y(z, w)": 3, "f(g)": 2}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		for source, want := range sources {
			wg.Add(1)
			go func(source string, want int) {
				defer wg.Done()
				tree := parseString(t, language, source)
				assert.Equal(t, want, countIdentifiers(t, language, tree), source)
			}(source, want)
		}
	}
	wg.Wait()
}

func TestExecuteQueriesReportsInvalidQuery(t *testing.T) {
	language, err := lang.NewTypescriptLanguage()
	require.NoError(t, err)

	tree := parseString(t, language, "a")
	data, err := tree.Data()
	require.NoError(t, err)

	err = ts.ExecuteQueries(ts.NewQueriesRequest(language, []ts.QueryItem{
		ts.NewQueryItem(`(no_such_node) @x`, func(*sitter.QueryMatch) error { return nil }),
	}), data, tree)
	assert.Error(t, err)
}

func parseString(t *testing.T, language core.Language, source string) core.ParseTree {
	parser := sitter.NewParser()
	parser.SetLanguage(language.Language())
	data := []byte(source)
	tree, err := parser.ParseCtx(context.Background(), nil, data)
	require.NoError(t, err)
	return stringTree{tree: tree, data: data, language: language}
}

type stringTree struct {
	tree     *sitter.Tree
	data     []byte
	language core.Language
}

func (s stringTree) Tree() *sitter.Tree               { return s.tree }
func (s stringTree) Data() (*[]byte, error)           { return &s.data, nil }
func (s stringTree) File() (core.File, error)         { return nil, nil }
func (s stringTree) Language() (core.Language, error) { return s.language, nil }

// otherGrammar has the code of one language and the grammar of another, as
// a second version of a grammar would.
type otherGrammar struct {
	base    core.Language
	grammar *sitter.Language
}

func (o otherGrammar) Meta() core.LanguageMeta           { return o.base.Meta() }
func (o otherGrammar) Language() *sitter.Language        { return o.grammar }
func (o otherGrammar) Resolvers() core.LanguageResolvers { return o.base.Resolvers() }

func TestExecuteQueriesKeepsAQueryForEachGrammar(t *testing.T) {
	python, err := lang.NewPythonLanguage()
	require.NoError(t, err)
	typescript, err := lang.NewTypescriptLanguage()
	require.NoError(t, err)
	strings := func(language core.Language, source string) int {
		tree := parseString(t, language, source)
		data, err := tree.Data()
		require.NoError(t, err)
		count := 0
		require.NoError(t, ts.ExecuteQueries(ts.NewQueriesRequest(language, []ts.QueryItem{
			ts.NewQueryItem(`(string) @s`, func(*sitter.QueryMatch) error {
				count++
				return nil
			}),
		}), data, tree))
		return count
	}

	assert.Equal(t, 1, strings(python, "a = 'x'"))
	mixed := otherGrammar{base: python, grammar: typescript.Language()}
	assert.Equal(t, 2, strings(mixed, "const a = 'x' + 'y'"),
		"a query compiled for the Python grammar does not run on a TypeScript tree")
}
