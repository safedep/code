package ts

import (
	"fmt"

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
