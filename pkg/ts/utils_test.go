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

	sources := []string{"const a = b(c)", "x.y(z, w)"}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		for _, source := range sources {
			wg.Add(1)
			go func(source string) {
				defer wg.Done()
				tree := parseString(t, language, source)
				assert.Equal(t, 3, countIdentifiers(t, language, tree))
			}(source)
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
