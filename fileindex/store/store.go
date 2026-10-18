// Package store defines where a build keeps file indexes between runs. It
// imports no SQLite package, so a caller can pick the store.
package store

import (
	"context"
	"time"

	"github.com/safedep/code/fileindex"
)

// Store keeps file indexes by fileindex.Key. A store keeps the time of the
// last read of each index at millisecond precision.
type Store interface {
	// Get returns the stored index of each key that the store has. A key
	// that the store does not have is not in the map. Get marks each hit
	// as read at the time of the call.
	Get(ctx context.Context, keys []fileindex.Key) (map[fileindex.Key]*fileindex.Index, error)

	// Put stores each index under its Key and marks it as read now. It
	// replaces an index of the same key.
	Put(ctx context.Context, indexes []*fileindex.Index) error

	// Prune removes the indexes whose last read is before readBefore, and
	// returns how many it removed. A Put counts as a read.
	Prune(ctx context.Context, readBefore time.Time) (int, error)
}
