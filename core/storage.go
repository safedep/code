package core

import (
	"context"

	"github.com/safedep/code/ent"
)

type CodeAnalysisStorage interface {
	// Client returns the ent client which is agnostic to internal storage implementation.
	Client() (*ent.Client, error)

	// Initialize the storage.
	Init(ctx context.Context) error

	// Close the connection to underlying storage
	Close() error
}
