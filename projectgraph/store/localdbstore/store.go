// Package localdbstore is a store.Store in the SQLite file of a dry/localdb
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
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/projectgraph/store"
	"github.com/safedep/dry/localdb"
)

const moduleName = "code_graph"

// maxBatchKeys bounds the keys of one query. Each key binds five variables,
// so a batch stays far below the variable limit of SQLite.
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
}

const keyColumns = `(content_hash, path, index_version, grammar_version, language)`

const putQuery = `INSERT INTO code_graph_file_indexes
	(content_hash, path, index_version, grammar_version, language, data, written_at, read_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT ` + keyColumns + ` DO UPDATE SET
		data = excluded.data,
		written_at = excluded.written_at,
		read_at = excluded.read_at`

const pruneQuery = `DELETE FROM code_graph_file_indexes WHERE read_at < ?`

var _ store.Store = (*Store)(nil)

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
	now := time.Now().UnixMilli()

	for start := 0; start < len(keys); start += maxBatchKeys {
		batch := keys[start:min(start+maxBatchKeys, len(keys))]
		if err := s.getBatch(ctx, batch, now, found); err != nil {
			return nil, err
		}
	}

	return found, nil
}

func (s *Store) getBatch(ctx context.Context, keys []fileindex.Key, now int64, found map[fileindex.Key]*fileindex.Index) (err error) {
	query := `UPDATE code_graph_file_indexes SET read_at = ?
		WHERE ` + keyColumns + ` IN (VALUES ` + placeholders(len(keys)) + `)
		RETURNING content_hash, path, index_version, grammar_version, language, data`

	args := make([]any, 0, 1+5*len(keys))
	args = append(args, now)
	for _, k := range keys {
		args = append(args, k.ContentHash, k.Path, k.IndexVersion, k.GrammarVersion, string(k.Language))
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("localdbstore: get indexes: %w", err)
	}
	defer func() {
		err = errors.Join(err, rows.Close())
	}()

	for rows.Next() {
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
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("localdbstore: read indexes: %w", err)
	}

	return nil
}

// Put encodes every index before the transaction starts, so the write
// transaction stays short. The commit or the rollback closes the statement.
func (s *Store) Put(ctx context.Context, indexes []*fileindex.Index) (err error) {
	type row struct {
		key  fileindex.Key
		data []byte
	}

	rows := make([]row, 0, len(indexes))
	for _, ix := range indexes {
		if ix == nil {
			return errors.New("localdbstore: nil index")
		}

		data, err := json.Marshal(ix)
		if err != nil {
			return fmt.Errorf("localdbstore: encode index of %s: %w", ix.Path, err)
		}
		rows = append(rows, row{key: ix.Key(), data: data})
	}

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

	stmt, err := tx.PrepareContext(ctx, putQuery)
	if err != nil {
		return fmt.Errorf("localdbstore: prepare put: %w", err)
	}

	now := time.Now().UnixMilli()
	for _, r := range rows {
		k := r.key
		if _, err := stmt.ExecContext(ctx, k.ContentHash, k.Path, k.IndexVersion, k.GrammarVersion, string(k.Language), r.data, now, now); err != nil {
			return fmt.Errorf("localdbstore: put index of %s: %w", k.Path, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("localdbstore: commit put: %w", err)
	}

	return nil
}

func (s *Store) Prune(ctx context.Context, readBefore time.Time) (int, error) {
	res, err := s.db.ExecContext(ctx, pruneQuery, readBefore.UnixMilli())
	if err != nil {
		return 0, fmt.Errorf("localdbstore: prune: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("localdbstore: count pruned rows: %w", err)
	}

	return int(n), nil
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("(?, ?, ?, ?, ?), ", n), ", ")
}
