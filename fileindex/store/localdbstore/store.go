// Package localdbstore is a store.Pruner in the SQLite file of a dry/localdb
// manager. It owns the code_graph module and its tables. The caller owns the
// manager and the file, so the store never opens, closes or removes the file.
package localdbstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/fileindex/store"
	"github.com/safedep/code/langcode"
	"github.com/safedep/dry/localdb"
)

const moduleName = "code_graph"

// maxBatchKeys bounds the keys of one query. Each key binds at most seven
// variables, so a batch stays far below the variable limit of SQLite.
const maxBatchKeys = 500

// migrations is append-only, as localdb requires. A change of the record
// shape bumps fileindex.Version and not the schema, so old rows miss and
// Prune removes them.
var migrations = []string{
	`CREATE TABLE code_graph_file_indexes (
		content_hash    TEXT NOT NULL,
		path            TEXT NOT NULL,
		index_version   TEXT NOT NULL,
		grammar_version TEXT NOT NULL,
		language        TEXT NOT NULL,
		data            BLOB NOT NULL,
		written_at      INTEGER NOT NULL,
		read_at         INTEGER NOT NULL,
		PRIMARY KEY (content_hash, path, index_version, grammar_version, language)
	)`,
	`CREATE INDEX code_graph_file_indexes_read ON code_graph_file_indexes (read_at)`,
	`CREATE TABLE code_graph_artifacts (
		content_hash    TEXT NOT NULL,
		path            TEXT NOT NULL,
		index_version   TEXT NOT NULL,
		grammar_version TEXT NOT NULL,
		language        TEXT NOT NULL,
		plugin          TEXT NOT NULL,
		plugin_version  TEXT NOT NULL,
		data            BLOB NOT NULL,
		written_at      INTEGER NOT NULL,
		read_at         INTEGER NOT NULL,
		PRIMARY KEY (content_hash, path, index_version, grammar_version, language, plugin, plugin_version)
	)`,
	`CREATE INDEX code_graph_artifacts_read ON code_graph_artifacts (read_at)`,
}

const (
	indexColumns    = `(content_hash, path, index_version, grammar_version, language)`
	artifactColumns = `(content_hash, path, index_version, grammar_version, language, plugin, plugin_version)`
)

const (
	getQuery = `UPDATE code_graph_file_indexes SET read_at = ?
		WHERE ` + indexColumns + ` IN (VALUES %s)
		RETURNING content_hash, path, index_version, grammar_version, language, data`

	putQuery = `INSERT INTO code_graph_file_indexes
		(content_hash, path, index_version, grammar_version, language, data, written_at, read_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT ` + indexColumns + ` DO UPDATE SET
			data = excluded.data,
			written_at = excluded.written_at,
			read_at = excluded.read_at`

	getArtifactsQuery = `UPDATE code_graph_artifacts SET read_at = ?
		WHERE ` + artifactColumns + ` IN (VALUES %s)
		RETURNING content_hash, path, index_version, grammar_version, language, plugin, plugin_version, data`

	putArtifactQuery = `INSERT INTO code_graph_artifacts
		(content_hash, path, index_version, grammar_version, language, plugin, plugin_version, data, written_at, read_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT ` + artifactColumns + ` DO UPDATE SET
			data = excluded.data,
			written_at = excluded.written_at,
			read_at = excluded.read_at`

	pruneQuery         = `DELETE FROM code_graph_file_indexes WHERE read_at < ?`
	pruneArtifactQuery = `DELETE FROM code_graph_artifacts WHERE read_at < ?`
)

var _ store.Pruner = (*Store)(nil)

type Store struct {
	db *sql.DB
}

// New registers the code_graph module on mgr and returns its store. A
// second call on one manager gets the module that the first call made.
func New(ctx context.Context, mgr localdb.Manager) (*Store, error) {
	s, err := mgr.Store(ctx, localdb.Descriptor{Name: moduleName, Migrations: migrations})
	if err != nil {
		return nil, fmt.Errorf("localdbstore: register module %s: %w", moduleName, err)
	}

	return &Store{db: s.DB()}, nil
}

// Get marks the hits of each batch as read and reads them in one statement.
func (s *Store) Get(ctx context.Context, keys []fileindex.Key) (map[fileindex.Key]*fileindex.Index, error) {
	found := make(map[fileindex.Key]*fileindex.Index, len(keys))
	err := s.getBatches(ctx, len(keys), 5, func(i int, args []any) []any {
		k := keys[i]
		return append(args, k.ContentHash, k.Path, k.IndexVersion, k.GrammarVersion, string(k.Language))
	}, getQuery, func(rows *sql.Rows) error {
		var k fileindex.Key
		var language string
		var data []byte
		if err := rows.Scan(&k.ContentHash, &k.Path, &k.IndexVersion, &k.GrammarVersion, &language, &data); err != nil {
			return fmt.Errorf("localdbstore: scan index: %w", err)
		}
		k.Language = langcode.Code(language)

		var ix fileindex.Index
		if err := json.Unmarshal(data, &ix); err != nil {
			return fmt.Errorf("localdbstore: decode index of %s: %w", k.Path, err)
		}
		found[k] = &ix
		return nil
	})
	if err != nil {
		return nil, err
	}
	return found, nil
}

// GetArtifacts marks the hits of each batch as read and reads them in one
// statement.
func (s *Store) GetArtifacts(ctx context.Context, keys []store.ArtifactKey) (map[store.ArtifactKey][]byte, error) {
	found := make(map[store.ArtifactKey][]byte, len(keys))
	err := s.getBatches(ctx, len(keys), 7, func(i int, args []any) []any {
		k := keys[i]
		return append(args, k.ContentHash, k.Path, k.IndexVersion, k.GrammarVersion, string(k.Language), k.Plugin, k.Version)
	}, getArtifactsQuery, func(rows *sql.Rows) error {
		var k store.ArtifactKey
		var language string
		var data []byte
		if err := rows.Scan(&k.ContentHash, &k.Path, &k.IndexVersion, &k.GrammarVersion, &language, &k.Plugin, &k.Version, &data); err != nil {
			return fmt.Errorf("localdbstore: scan artifact: %w", err)
		}
		k.Language = langcode.Code(language)
		found[k] = data
		return nil
	})
	if err != nil {
		return nil, err
	}
	return found, nil
}

// getBatches runs the query, with the placeholders of each batch of at
// most maxBatchKeys keys, and gives each row to scan. bind appends the
// width values of key i to the arguments.
func (s *Store) getBatches(ctx context.Context, keys, width int, bind func(i int, args []any) []any, query string, scan func(*sql.Rows) error) error {
	now := time.Now().UnixMilli()
	for start := 0; start < keys; start += maxBatchKeys {
		end := min(start+maxBatchKeys, keys)
		args := make([]any, 0, 1+width*(end-start))
		args = append(args, now)
		for i := start; i < end; i++ {
			args = bind(i, args)
		}
		if err := s.query(ctx, fmt.Sprintf(query, placeholders(end-start, width)), args, scan); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) query(ctx context.Context, query string, args []any, scan func(*sql.Rows) error) (err error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("localdbstore: get: %w", err)
	}
	defer func() {
		err = errors.Join(err, rows.Close())
	}()

	for rows.Next() {
		if err := scan(rows); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("localdbstore: read rows: %w", err)
	}
	return nil
}

// Put encodes every index before the transaction starts, so the write
// transaction stays short.
func (s *Store) Put(ctx context.Context, indexes []*fileindex.Index) error {
	rows := make([][]any, 0, len(indexes))
	for _, ix := range indexes {
		if ix == nil {
			return errors.New("localdbstore: nil index")
		}

		data, err := json.Marshal(ix)
		if err != nil {
			return fmt.Errorf("localdbstore: encode index of %s: %w", ix.Path, err)
		}
		k := ix.Key()
		rows = append(rows, []any{k.ContentHash, k.Path, k.IndexVersion, k.GrammarVersion, string(k.Language), data})
	}
	return s.put(ctx, putQuery, rows)
}

// PutArtifacts writes the artifacts in one transaction.
func (s *Store) PutArtifacts(ctx context.Context, artifacts []store.Artifact) error {
	rows := make([][]any, 0, len(artifacts))
	for _, a := range artifacts {
		k := a.Key
		rows = append(rows, []any{k.ContentHash, k.Path, k.IndexVersion, k.GrammarVersion, string(k.Language), k.Plugin, k.Version, a.Data})
	}
	return s.put(ctx, putArtifactQuery, rows)
}

// put runs the insert for each row in one transaction, with the time of
// the write and of the read after the values of the row. The commit or
// the rollback closes the statement.
func (s *Store) put(ctx context.Context, query string, rows [][]any) (err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("localdbstore: begin put: %w", err)
	}
	defer func() {
		if err == nil {
			return
		}
		if rbErr := tx.Rollback(); !errors.Is(rbErr, sql.ErrTxDone) {
			err = errors.Join(err, rbErr)
		}
	}()

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("localdbstore: prepare put: %w", err)
	}

	now := time.Now().UnixMilli()
	for _, r := range rows {
		if _, err := stmt.ExecContext(ctx, append(r, now, now)...); err != nil {
			return fmt.Errorf("localdbstore: put %v: %w", r[1], err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("localdbstore: commit put: %w", err)
	}

	return nil
}

// Prune deletes the rows of both tables read before the time, in one
// transaction, so a reader sees the indexes and the artifacts of a file
// go together.
func (s *Store) Prune(ctx context.Context, readBefore time.Time) (removed int, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("localdbstore: begin prune: %w", err)
	}
	defer func() {
		if err == nil {
			return
		}
		if rbErr := tx.Rollback(); !errors.Is(rbErr, sql.ErrTxDone) {
			err = errors.Join(err, rbErr)
		}
	}()

	for _, query := range []string{pruneQuery, pruneArtifactQuery} {
		res, err := tx.ExecContext(ctx, query, readBefore.UnixMilli())
		if err != nil {
			return 0, fmt.Errorf("localdbstore: prune: %w", err)
		}

		n, err := res.RowsAffected()
		if err != nil {
			return 0, fmt.Errorf("localdbstore: count pruned rows: %w", err)
		}
		removed += int(n)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("localdbstore: commit prune: %w", err)
	}
	return removed, nil
}

// placeholders returns n groups of width placeholders, as (?, ?), (?, ?).
func placeholders(n, width int) string {
	group := "(" + strings.TrimSuffix(strings.Repeat("?, ", width), ", ") + "), "
	return strings.TrimSuffix(strings.Repeat(group, n), ", ")
}
