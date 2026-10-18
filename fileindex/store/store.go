// Package store defines where a build keeps file indexes and plugin
// artifacts between runs. It imports no SQLite package, so a caller can
// pick the store.
package store

import (
	"context"
	"time"

	"github.com/safedep/code/fileindex"
)

// ArtifactKey names the output of one plugin for one file: the key of the
// file and the name and the version of the plugin. A new version of a
// plugin makes every older artifact miss.
type ArtifactKey struct {
	fileindex.Key
	Plugin  string
	Version string
}

// Artifact is the output of one plugin for one file, as the plugin encodes
// it.
type Artifact struct {
	Key  ArtifactKey
	Data []byte
}

// Store keeps file indexes by fileindex.Key and plugin artifacts by
// ArtifactKey.
type Store interface {
	// Get returns the stored index of each key that the store has. A key
	// that the store does not have is not in the map.
	Get(ctx context.Context, keys []fileindex.Key) (map[fileindex.Key]*fileindex.Index, error)

	// Put stores each index under its Key. It replaces an index of the
	// same key.
	Put(ctx context.Context, indexes []*fileindex.Index) error

	// GetArtifacts returns the stored artifact of each key that the store
	// has. A key that the store does not have is not in the map.
	GetArtifacts(ctx context.Context, keys []ArtifactKey) (map[ArtifactKey][]byte, error)

	// PutArtifacts stores each artifact under its key. It replaces an
	// artifact of the same key.
	PutArtifacts(ctx context.Context, artifacts []Artifact) error
}

// Pruner is a store that removes what no run has read for a time. It keeps
// the time of the last read of each index and artifact at millisecond
// precision. Its Get and GetArtifacts mark each hit as read at the time of
// the call, and a Put counts as a read.
type Pruner interface {
	Store

	// Prune removes the indexes and the artifacts whose last read is
	// before readBefore, and returns how many it removed.
	Prune(ctx context.Context, readBefore time.Time) (int, error)
}
