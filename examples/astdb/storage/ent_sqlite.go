package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/safedep/code/examples/astdb/ent"
)

type EntSqliteClientConfig struct {
	// Path to the sqlite database file
	Path string

	// Read Only mode
	ReadOnly bool

	// Skip schema creation/migration
	SkipSchemaCreation bool

	// Enable foreign key constraints
	EnableForeignKeys bool

	// Connection pool settings
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration

	// Migration settings
	EnableAutoMigration bool
	DropColumns         bool
	DropIndexes         bool
}

// DefaultEntSqliteConfig returns a configuration with sensible defaults
func DefaultEntSqliteConfig(path string) EntSqliteClientConfig {
	return EntSqliteClientConfig{
		Path:                path,
		ReadOnly:            false,
		SkipSchemaCreation:  false,
		EnableForeignKeys:   true,
		MaxOpenConns:        25,
		MaxIdleConns:        5,
		ConnMaxLifetime:     5 * time.Minute,
		EnableAutoMigration: true,
		DropColumns:         false,
		DropIndexes:         false,
	}
}

type entSqliteClient struct {
	client *ent.Client
	db     *sql.DB
	config EntSqliteClientConfig
}

// Compile-time interface compliance check
var _ SqlStorage = (*entSqliteClient)(nil)

func NewEntSqliteStorage(config EntSqliteClientConfig) (SqlStorage, error) {
	mode := "rwc"
	if config.ReadOnly {
		mode = "ro"
	}

	// Ensure the path exists
	dir := filepath.Dir(config.Path)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create DB path %s: %w", dir, err)
	}

	// Build connection string with additional options
	connStr := fmt.Sprintf("file:%s?mode=%s&cache=shared", config.Path, mode)
	if config.EnableForeignKeys {
		connStr += "&_fk=1"
	}
	connStr += "&_journal=WAL&_sync=NORMAL&_timeout=5000"

	// Open raw database connection for configuration
	db, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite3 connection: %w", err)
	}

	// Configure connection pool
	if config.MaxOpenConns > 0 {
		db.SetMaxOpenConns(config.MaxOpenConns)
	}

	if config.MaxIdleConns > 0 {
		db.SetMaxIdleConns(config.MaxIdleConns)
	}

	if config.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(config.ConnMaxLifetime)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Create ENT client using the configured database
	client, err := ent.Open("sqlite3", connStr)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create ENT client: %w", err)
	}

	storage := &entSqliteClient{
		client: client,
		db:     db,
		config: config,
	}

	// Handle schema creation/migration
	if !config.SkipSchemaCreation {
		if err := storage.migrateSchema(context.Background()); err != nil {
			storage.Close()
			return nil, fmt.Errorf("failed to migrate schema: %w", err)
		}
	}

	return storage, nil
}

func (c *entSqliteClient) migrateSchema(ctx context.Context) error {
	// Simple schema creation for now
	// TODO: Add advanced migration options when needed
	return c.client.Schema.Create(ctx)
}

func (c *entSqliteClient) Client() (*ent.Client, error) {
	return c.client, nil
}

func (c *entSqliteClient) Close() error {
	if c.client != nil {
		if err := c.client.Close(); err != nil {
			return fmt.Errorf("failed to close ENT client: %w", err)
		}
	}
	if c.db != nil {
		if err := c.db.Close(); err != nil {
			return fmt.Errorf("failed to close database: %w", err)
		}
	}
	return nil
}

// BeginTx starts a new transaction
func (c *entSqliteClient) BeginTx(ctx context.Context, opts *sql.TxOptions) (*ent.Tx, error) {
	return c.client.BeginTx(ctx, opts)
}

// WithTx executes a function within a transaction
func (c *entSqliteClient) WithTx(ctx context.Context, fn func(*ent.Tx) error) error {
	tx, err := c.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}

		if err != nil {
			tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()

	err = fn(tx)
	return err
}

// DatabaseStats returns database statistics
func (c *entSqliteClient) DatabaseStats() sql.DBStats {
	return c.db.Stats()
}

// Vacuum runs VACUUM command to reclaim space
func (c *entSqliteClient) Vacuum(ctx context.Context) error {
	_, err := c.db.ExecContext(ctx, "VACUUM")
	return err
}

// Analyze runs ANALYZE command to update statistics
func (c *entSqliteClient) Analyze(ctx context.Context) error {
	_, err := c.db.ExecContext(ctx, "ANALYZE")
	return err
}
