// Package storetest is the test suite that every store.Store passes.
package storetest

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/fileindex/store"
	"github.com/safedep/code/langcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tick is longer than the millisecond precision of a read time, so two
// reads on either side of a sleep of tick have two read times.
const tick = 5 * time.Millisecond

// Run runs the suite. newStore returns an empty store for each test. A
// store that is a store.Pruner also passes the prune tests.
func Run(t *testing.T, newStore func(t *testing.T) store.Store) {
	tests := []struct {
		name string
		run  func(t *testing.T, s store.Store)
	}{
		{"RoundTrip", testRoundTrip},
		{"GetMissingKey", testGetMissingKey},
		{"GetEmpty", testGetEmpty},
		{"MissAfterKeyChange", testMissAfterKeyChange},
		{"PutReplaces", testPutReplaces},
		{"ReturnedIndexIsACopy", testReturnedIndexIsACopy},
		{"ManyKeys", testManyKeys},
		{"ArtifactRoundTrip", testArtifactRoundTrip},
		{"ArtifactMissAfterKeyChange", testArtifactMissAfterKeyChange},
		{"ArtifactReplacesAndCopies", testArtifactReplacesAndCopies},
		{"ManyArtifactKeys", testManyArtifactKeys},
		{"Concurrent", testConcurrent},
		{"CanceledContext", testCanceledContext},
	}
	pruneTests := []struct {
		name string
		run  func(t *testing.T, s store.Pruner)
	}{
		{"PruneRemovesOnlyRowsReadBefore", testPruneRemovesOnlyRowsReadBefore},
		{"PruneRemovesArtifacts", testPruneRemovesArtifacts},
		{"GetMarksRead", testGetMarksRead},
		{"GetArtifactsMarksRead", testGetArtifactsMarksRead},
		{"CanceledPrune", testCanceledPrune},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.run(t, newStore(t))
		})
	}
	for _, tc := range pruneTests {
		t.Run(tc.name, func(t *testing.T) {
			p, ok := newStore(t).(store.Pruner)
			if !ok {
				t.Skip("the store is no store.Pruner")
			}
			tc.run(t, p)
		})
	}
}

// ArtifactKey returns the artifact key of a plugin for the index of a path.
func ArtifactKey(path, plugin, version string) store.ArtifactKey {
	return store.ArtifactKey{Key: Index(path).Key(), Plugin: plugin, Version: version}
}

func putArtifacts(t *testing.T, s store.Store, artifacts ...store.Artifact) {
	t.Helper()
	require.NoError(t, s.PutArtifacts(context.Background(), artifacts))
}

func getArtifacts(t *testing.T, s store.Store, keys ...store.ArtifactKey) map[store.ArtifactKey][]byte {
	t.Helper()
	found, err := s.GetArtifacts(context.Background(), keys)
	require.NoError(t, err)
	return found
}

// Index returns a small index of every record kind for a path.
func Index(path string) *fileindex.Index {
	return &fileindex.Index{
		Version:     fileindex.Version,
		Path:        path,
		Language:    langcode.Python,
		ContentHash: fileindex.HashContent([]byte(path)),
		Imports:     []fileindex.Import{{Line: 1, Module: ".utils", Item: "load", Kind: fileindex.ImportStatic, Namespace: "utils//load"}},
		Exports:     []fileindex.Export{{Name: "run", Target: path + "//run", Line: 3}},
		Definitions: []fileindex.Definition{
			{Key: path, Kind: fileindex.DefModule, Name: "app", StartLine: 1, EndLine: 9, Exported: true},
			{Key: path + "//run", Kind: fileindex.DefFunction, Name: "run", StartLine: 3, EndLine: 5, Exported: true, Flags: []fileindex.Flag{{Kind: fileindex.FlagEval, Line: 4}}},
		},
		Classes:    []fileindex.Class{{Key: path + "//C", Bases: []string{"Base"}}},
		Calls:      []fileindex.Call{{Caller: path + "//run", Line: 4, Column: 5, Callee: "load", Targets: []string{"utils//load"}, Kind: fileindex.CallCall, Args: []fileindex.Arg{{Shape: fileindex.ArgName, Targets: []string{path + "//text"}}}}},
		Unresolved: []fileindex.Unresolved{{Caller: path + "//run", Line: 5, Column: 5, Text: "c.send", Receiver: "c", Method: "send", Reason: fileindex.UnresolvedUnknownReceiver}},
		Flags:      []fileindex.Flag{{Kind: fileindex.FlagReflection, Line: 6}},
		Truncated:  []fileindex.Truncation{{Kind: fileindex.TruncationDepth, Line: 7}},
	}
}

func put(t *testing.T, s store.Store, indexes ...*fileindex.Index) {
	t.Helper()
	require.NoError(t, s.Put(context.Background(), indexes))
}

func get(t *testing.T, s store.Store, keys ...fileindex.Key) map[fileindex.Key]*fileindex.Index {
	t.Helper()
	found, err := s.Get(context.Background(), keys)
	require.NoError(t, err)
	return found
}

func testRoundTrip(t *testing.T, s store.Store) {
	a, b := Index("src/a.py"), Index("src/b.py")
	put(t, s, a, b)

	found := get(t, s, a.Key(), b.Key())
	assert.Equal(t, map[fileindex.Key]*fileindex.Index{a.Key(): a, b.Key(): b}, found)
}

func testGetMissingKey(t *testing.T, s store.Store) {
	a := Index("src/a.py")
	put(t, s, a)

	found := get(t, s, Index("src/b.py").Key(), a.Key())
	assert.Equal(t, map[fileindex.Key]*fileindex.Index{a.Key(): a}, found)
}

func testGetEmpty(t *testing.T, s store.Store) {
	assert.Empty(t, get(t, s))
	require.NoError(t, s.Put(context.Background(), nil))
}

func testMissAfterKeyChange(t *testing.T, s store.Store) {
	a := Index("src/a.py")
	put(t, s, a)

	changes := map[string]func(k *fileindex.Key){
		"index version":   func(k *fileindex.Key) { k.IndexVersion = fileindex.Version + "-next" },
		"grammar version": func(k *fileindex.Key) { k.GrammarVersion += "-next" },
		"content hash":    func(k *fileindex.Key) { k.ContentHash = fileindex.HashContent([]byte("changed")) },
		"path":            func(k *fileindex.Key) { k.Path = "src/copy.py" },
		"language":        func(k *fileindex.Key) { k.Language = langcode.Javascript },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			key := a.Key()
			change(&key)
			assert.Empty(t, get(t, s, key))
		})
	}

	assert.Len(t, get(t, s, a.Key()), 1)
}

func testPutReplaces(t *testing.T, s store.Store) {
	a := Index("src/a.py")
	put(t, s, a)

	b := Index("src/a.py")
	b.Package = "app"
	b.Calls = nil
	put(t, s, b)

	assert.Equal(t, b, get(t, s, a.Key())[a.Key()])
}

func testReturnedIndexIsACopy(t *testing.T, s store.Store) {
	a := Index("src/a.py")
	put(t, s, a)
	a.Calls[0].Callee = "changed"

	got := get(t, s, a.Key())[a.Key()]
	require.NotNil(t, got)
	assert.Equal(t, "load", got.Calls[0].Callee)

	got.Path = "changed"
	assert.Equal(t, "src/a.py", get(t, s, a.Key())[a.Key()].Path)
}

// testManyKeys passes more keys than one query of a SQL store holds.
func testManyKeys(t *testing.T, s store.Store) {
	const n = 1201
	indexes := make([]*fileindex.Index, n)
	keys := make([]fileindex.Key, 0, n+1)
	for i := range indexes {
		indexes[i] = Index(fmt.Sprintf("src/m%04d.py", i))
		keys = append(keys, indexes[i].Key())
	}
	put(t, s, indexes...)

	keys = append(keys, Index("src/missing.py").Key())
	found := get(t, s, keys...)
	require.Len(t, found, n)
	for _, ix := range indexes {
		assert.Equal(t, ix, found[ix.Key()])
	}
}

func testArtifactRoundTrip(t *testing.T, s store.Store) {
	a, b := ArtifactKey("src/a.py", "calls", "1"), ArtifactKey("src/a.py", "flags", "1")
	putArtifacts(t, s, store.Artifact{Key: a, Data: []byte("calls of a")}, store.Artifact{Key: b, Data: []byte("flags of a")})

	found := getArtifacts(t, s, a, b, ArtifactKey("src/b.py", "calls", "1"))
	assert.Equal(t, map[store.ArtifactKey][]byte{a: []byte("calls of a"), b: []byte("flags of a")}, found)
	assert.Empty(t, getArtifacts(t, s))
	require.NoError(t, s.PutArtifacts(context.Background(), nil))
}

func testArtifactMissAfterKeyChange(t *testing.T, s store.Store) {
	a := ArtifactKey("src/a.py", "calls", "1")
	putArtifacts(t, s, store.Artifact{Key: a, Data: []byte("calls of a")})

	changes := map[string]func(k *store.ArtifactKey){
		"plugin":          func(k *store.ArtifactKey) { k.Plugin = "flags" },
		"plugin version":  func(k *store.ArtifactKey) { k.Version = "2" },
		"index version":   func(k *store.ArtifactKey) { k.IndexVersion = fileindex.Version + "-next" },
		"grammar version": func(k *store.ArtifactKey) { k.GrammarVersion += "-next" },
		"content hash":    func(k *store.ArtifactKey) { k.ContentHash = fileindex.HashContent([]byte("changed")) },
		"path":            func(k *store.ArtifactKey) { k.Path = "src/copy.py" },
		"language":        func(k *store.ArtifactKey) { k.Language = langcode.Javascript },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			key := a
			change(&key)
			assert.Empty(t, getArtifacts(t, s, key))
		})
	}
	assert.Len(t, getArtifacts(t, s, a), 1)
}

func testArtifactReplacesAndCopies(t *testing.T, s store.Store) {
	a := ArtifactKey("src/a.py", "calls", "1")
	data := []byte("first")
	putArtifacts(t, s, store.Artifact{Key: a, Data: data})
	data[0] = 'x'
	assert.Equal(t, []byte("first"), getArtifacts(t, s, a)[a], "the store keeps its own copy")

	putArtifacts(t, s, store.Artifact{Key: a, Data: []byte("second")})
	got := getArtifacts(t, s, a)[a]
	assert.Equal(t, []byte("second"), got)
	got[0] = 'x'
	assert.Equal(t, []byte("second"), getArtifacts(t, s, a)[a], "a caller that changes a returned artifact does not change the store")
}

// testManyArtifactKeys passes more keys than one query of a SQL store
// holds.
func testManyArtifactKeys(t *testing.T, s store.Store) {
	const n = 1201
	artifacts := make([]store.Artifact, n)
	keys := make([]store.ArtifactKey, 0, n+1)
	for i := range artifacts {
		key := ArtifactKey(fmt.Sprintf("src/m%04d.py", i), "calls", "1")
		artifacts[i] = store.Artifact{Key: key, Data: []byte(key.Path)}
		keys = append(keys, key)
	}
	putArtifacts(t, s, artifacts...)

	keys = append(keys, ArtifactKey("src/missing.py", "calls", "1"))
	found := getArtifacts(t, s, keys...)
	require.Len(t, found, n)
	for _, a := range artifacts {
		assert.Equal(t, a.Data, found[a.Key])
	}
}

func testPruneRemovesArtifacts(t *testing.T, s store.Pruner) {
	old, fresh := ArtifactKey("src/old.py", "calls", "1"), ArtifactKey("src/fresh.py", "calls", "1")
	oldIndex := Index("src/old.py")
	put(t, s, oldIndex)
	putArtifacts(t, s, store.Artifact{Key: old, Data: []byte("old")})
	cut := pause()
	putArtifacts(t, s, store.Artifact{Key: fresh, Data: []byte("fresh")})

	removed, err := s.Prune(context.Background(), cut)
	require.NoError(t, err)
	assert.Equal(t, 2, removed, "the index and the artifact of the old file")
	assert.Equal(t, map[store.ArtifactKey][]byte{fresh: []byte("fresh")}, getArtifacts(t, s, old, fresh))
	assert.Empty(t, get(t, s, oldIndex.Key()))
}

func testGetArtifactsMarksRead(t *testing.T, s store.Pruner) {
	read, unread := ArtifactKey("src/read.py", "calls", "1"), ArtifactKey("src/unread.py", "calls", "1")
	putArtifacts(t, s, store.Artifact{Key: read, Data: []byte("read")}, store.Artifact{Key: unread, Data: []byte("unread")})
	cut := pause()
	assert.Len(t, getArtifacts(t, s, read), 1)

	removed, err := s.Prune(context.Background(), cut)
	require.NoError(t, err)
	assert.Equal(t, 1, removed)
	assert.Equal(t, map[store.ArtifactKey][]byte{read: []byte("read")}, getArtifacts(t, s, read, unread))
}

func testCanceledPrune(t *testing.T, s store.Pruner) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := s.Prune(ctx, time.Now())
	require.ErrorIs(t, err, context.Canceled)
}

func testPruneRemovesOnlyRowsReadBefore(t *testing.T, s store.Pruner) {
	old, fresh := Index("src/old.py"), Index("src/fresh.py")
	put(t, s, old)
	cut := pause()
	put(t, s, fresh)

	removed, err := s.Prune(context.Background(), cut)
	require.NoError(t, err)
	assert.Equal(t, 1, removed)

	found := get(t, s, old.Key(), fresh.Key())
	assert.Equal(t, map[fileindex.Key]*fileindex.Index{fresh.Key(): fresh}, found)

	removed, err = s.Prune(context.Background(), cut)
	require.NoError(t, err)
	assert.Zero(t, removed)
}

func testGetMarksRead(t *testing.T, s store.Pruner) {
	read, unread := Index("src/read.py"), Index("src/unread.py")
	put(t, s, read, unread)
	cut := pause()
	assert.Len(t, get(t, s, read.Key()), 1)

	removed, err := s.Prune(context.Background(), cut)
	require.NoError(t, err)
	assert.Equal(t, 1, removed)

	found := get(t, s, read.Key(), unread.Key())
	assert.Equal(t, map[fileindex.Key]*fileindex.Index{read.Key(): read}, found)
}

func testConcurrent(t *testing.T, s store.Store) {
	const workers = 8
	var wg sync.WaitGroup
	errs := make(chan error, workers)

	for w := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ix := Index(fmt.Sprintf("src/w%d.py", w))
			if err := s.Put(context.Background(), []*fileindex.Index{ix}); err != nil {
				errs <- err
				return
			}
			found, err := s.Get(context.Background(), []fileindex.Key{ix.Key()})
			if err != nil {
				errs <- err
				return
			}
			if found[ix.Key()] == nil {
				errs <- fmt.Errorf("no index for %s after put", ix.Path)
			}
		}()
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		assert.NoError(t, err)
	}
}

func testCanceledContext(t *testing.T, s store.Store) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	a := Index("src/a.py")
	require.ErrorIs(t, s.Put(ctx, []*fileindex.Index{a}), context.Canceled)
	_, err := s.Get(ctx, []fileindex.Key{a.Key()})
	require.ErrorIs(t, err, context.Canceled)
	key := ArtifactKey("src/a.py", "calls", "1")
	require.ErrorIs(t, s.PutArtifacts(ctx, []store.Artifact{{Key: key, Data: []byte("x")}}), context.Canceled)
	_, err = s.GetArtifacts(ctx, []store.ArtifactKey{key})
	require.ErrorIs(t, err, context.Canceled)
}

// pause returns a time with a read time before it and after it.
func pause() time.Time {
	time.Sleep(tick)
	cut := time.Now()
	time.Sleep(tick)
	return cut
}
