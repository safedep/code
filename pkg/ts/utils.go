package ts

import (
	"fmt"
	"sync"

	"github.com/safedep/code/core"
	sitter "github.com/smacker/go-tree-sitter"
)

type sitterQueryExecutor struct {
	lang   *sitter.Language
	source []byte
}

type queryMatchWrapper struct {
	cursor *sitter.QueryCursor
	source []byte
}

func (m *queryMatchWrapper) Close() {
	m.cursor.Close()
}

func (m *queryMatchWrapper) ForEach(cb func(*sitter.QueryMatch) error) error {
	for {
		match, ok := m.cursor.NextMatch()
		if !ok {
			break
		}

		match = m.cursor.FilterPredicates(match, m.source)
		if len(match.Captures) == 0 {
			continue
		}

		if err := cb(match); err != nil {
			return fmt.Errorf("callback failed: %w", err)
		}
	}

	return nil
}

func NewQueryExecutor(lang *sitter.Language, source []byte) *sitterQueryExecutor {
	return &sitterQueryExecutor{
		lang:   lang,
		source: source,
	}
}

func (e *sitterQueryExecutor) Execute(node *sitter.Node, query string) (*queryMatchWrapper, error) {
	q, err := sitter.NewQuery([]byte(query), e.lang)
	if err != nil {
		return nil, fmt.Errorf("failed to create query: %w", err)
	}

	return execute(q, node, e.source), nil
}

func execute(q *sitter.Query, node *sitter.Node, source []byte) *queryMatchWrapper {
	cursor := sitter.NewQueryCursor()
	cursor.Exec(q, node)

	return &queryMatchWrapper{cursor: cursor, source: source}
}

// queryKey names a compiled query by its language code and by the size of
// its grammar. Two languages with one code and other grammars, such as two
// versions of a grammar, get their own query. GetLanguage of a grammar
// returns a new wrapper on each call, so the key cannot be its pointer.
type queryKey struct {
	language core.LanguageCode
	symbols  uint32
	query    string
}

// compiledQueries keeps each compiled query for the life of the process. To
// compile a query against a large grammar, such as TSX, costs more than to
// parse a file. A query does not change after it is compiled, so the cursors
// of many files can share it.
var compiledQueries sync.Map

func compiledQuery(language core.Language, query string) (*sitter.Query, error) {
	key := queryKey{language: language.Meta().Code, symbols: language.Language().SymbolCount(), query: query}
	if q, ok := compiledQueries.Load(key); ok {
		return q.(*sitter.Query), nil
	}

	q, err := sitter.NewQuery([]byte(query), language.Language())
	if err != nil {
		return nil, fmt.Errorf("failed to create query: %w", err)
	}

	actual, loaded := compiledQueries.LoadOrStore(key, q)
	if loaded {
		// Another file compiled the same query first.
		q.Close()
	}
	return actual.(*sitter.Query), nil
}

type QueryMatchProcessor func(*sitter.QueryMatch) error
type QueryItem struct {
	query string
	cb    QueryMatchProcessor
}

func NewQueryItem(query string, cb QueryMatchProcessor) QueryItem {
	return QueryItem{
		query: query,
		cb:    cb,
	}
}

type QueriesRequest struct {
	language   core.Language
	queryItems []QueryItem
}

func NewQueriesRequest(language core.Language, queryItems []QueryItem) QueriesRequest {
	return QueriesRequest{
		language:   language,
		queryItems: queryItems,
	}
}

func ExecuteQueries(queriesRequest QueriesRequest, data *[]byte, tree core.ParseTree) error {
	for _, queryItem := range queriesRequest.queryItems {
		q, err := compiledQuery(queriesRequest.language, queryItem.query)
		if err != nil {
			return err
		}

		matches := execute(q, tree.Tree().RootNode(), *data)
		err = matches.ForEach(queryItem.cb)
		matches.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
