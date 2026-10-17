// Package memstore is a store.Store in memory, for tests and one-shot
// callers.
package memstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/projectgraph/store"
)

var _ store.Store = (*Store)(nil)

// entry keeps the JSON of an index, as a file store does, so a caller that
// changes a returned index does not change the store.
type entry struct {
	data   []byte
	readAt int64
}

type Store struct {
	mu      sync.Mutex
	entries map[fileindex.Key]entry
}

func New() *Store {
	return &Store{entries: map[fileindex.Key]entry{}}
}

func (s *Store) Get(ctx context.Context, keys []fileindex.Key) (map[fileindex.Key]*fileindex.Index, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	now := time.Now().UnixMilli()
	found := make(map[fileindex.Key]*fileindex.Index)

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, key := range keys {
		e, ok := s.entries[key]
		if !ok {
			continue
		}

		var ix fileindex.Index
		if err := json.Unmarshal(e.data, &ix); err != nil {
			return nil, fmt.Errorf("memstore: decode index of %s: %w", key.Path, err)
		}

		e.readAt = now
		s.entries[key] = e
		found[key] = &ix
	}

	return found, nil
}

func (s *Store) Put(ctx context.Context, indexes []*fileindex.Index) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	now := time.Now().UnixMilli()
	encoded := make(map[fileindex.Key]entry, len(indexes))
	for _, ix := range indexes {
		if ix == nil {
			return errors.New("memstore: nil index")
		}

		data, err := json.Marshal(ix)
		if err != nil {
			return fmt.Errorf("memstore: encode index of %s: %w", ix.Path, err)
		}

		encoded[ix.Key()] = entry{data: data, readAt: now}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for key, e := range encoded {
		s.entries[key] = e
	}

	return nil
}

func (s *Store) Prune(ctx context.Context, readBefore time.Time) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	cut := readBefore.UnixMilli()

	s.mu.Lock()
	defer s.mu.Unlock()

	removed := 0
	for key, e := range s.entries {
		if e.readAt < cut {
			delete(s.entries, key)
			removed++
		}
	}

	return removed, nil
}
