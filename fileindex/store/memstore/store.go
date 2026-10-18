// Package memstore is a store.Pruner in memory, for tests and one-shot
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
	"github.com/safedep/code/fileindex/store"
)

var _ store.Pruner = (*Store)(nil)

// entry keeps the bytes of an index or an artifact, as a file store does,
// so a caller that changes a returned value does not change the store.
type entry struct {
	data   []byte
	readAt int64
}

type Store struct {
	mu        sync.Mutex
	entries   map[fileindex.Key]entry
	artifacts map[store.ArtifactKey]entry
}

func New() *Store {
	return &Store{entries: map[fileindex.Key]entry{}, artifacts: map[store.ArtifactKey]entry{}}
}

// Get decodes the stored index of each key and marks the hit as read.
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

// Put encodes each index before it takes the lock.
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

// GetArtifacts returns a copy of each stored artifact and marks the hit as
// read.
func (s *Store) GetArtifacts(ctx context.Context, keys []store.ArtifactKey) (map[store.ArtifactKey][]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	now := time.Now().UnixMilli()
	found := make(map[store.ArtifactKey][]byte)

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, key := range keys {
		e, ok := s.artifacts[key]
		if !ok {
			continue
		}
		e.readAt = now
		s.artifacts[key] = e
		found[key] = append([]byte(nil), e.data...)
	}

	return found, nil
}

// PutArtifacts keeps a copy of each artifact.
func (s *Store) PutArtifacts(ctx context.Context, artifacts []store.Artifact) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	now := time.Now().UnixMilli()

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, a := range artifacts {
		s.artifacts[a.Key] = entry{data: append([]byte(nil), a.Data...), readAt: now}
	}

	return nil
}

// Prune removes the indexes and the artifacts read before the time.
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
	for key, e := range s.artifacts {
		if e.readAt < cut {
			delete(s.artifacts, key)
			removed++
		}
	}

	return removed, nil
}
