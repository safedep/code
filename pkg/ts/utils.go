package ts

import (
	"fmt"

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

	cursor := sitter.NewQueryCursor()
	cursor.Exec(q, node)

	return &queryMatchWrapper{cursor: cursor, source: e.source}, nil
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
	qx := NewQueryExecutor(queriesRequest.language.Language(), *data)

	for _, queryItem := range queriesRequest.queryItems {
		matches, err := qx.Execute(tree.Tree().RootNode(), queryItem.query)
		if err != nil {
			return err
		}

		err = matches.ForEach(queryItem.cb)
		if err != nil {
			return err
		}
	}
	return nil
}
