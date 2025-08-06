package storage

import (
	"context"
	"database/sql"

	"github.com/safedep/code/examples/astdb/ent"
)

// Storage interface defines a generic contract for implementing
// a storage driver in the system. This usually means wrapping
// a DB client with our defined interface.
type Storage[T any] interface {
	// Returns a client to the underlying storage driver.
	// We will NOT hide the underlying storage driver because for an ORM
	// which already abstracts DB connection, its unnecessary work to abstract
	// an ORM and then implement function wrappers for all ORM operations.
	Client() (T, error)

	// Close any open connections, file descriptors and free
	// any resources used by the storage driver
	Close() error
}

// SqlStorage interface defines advanced SQL storage operations
// for SQLite and other SQL-based storage implementations.
// This interface extends the basic Storage interface with
// transaction management, performance, and maintenance operations.
type SqlStorage interface {
	Storage[*ent.Client]

	// Transaction Management

	// BeginTx starts a new transaction with the given options
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*ent.Tx, error)

	// WithTx executes a function within a transaction.
	// The transaction is automatically committed if the function returns nil,
	// or rolled back if it returns an error or panics.
	WithTx(ctx context.Context, fn func(*ent.Tx) error) error

	// Performance and Maintenance

	// DatabaseStats returns database connection statistics
	DatabaseStats() sql.DBStats

	// Vacuum runs the VACUUM command to reclaim unused space
	// and defragment the database file
	Vacuum(ctx context.Context) error

	// Analyze runs the ANALYZE command to update query planner
	// statistics for better query optimization
	Analyze(ctx context.Context) error
}
