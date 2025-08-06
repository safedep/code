package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/safedep/code/examples/astdb/ent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEntSqliteStorage_BasicCreation(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	config := DefaultEntSqliteConfig(dbPath)
	storage, err := NewEntSqliteStorage(config)
	require.NoError(t, err)
	defer storage.Close()

	// Verify storage implements SqlStorage interface
	assert.Implements(t, (*SqlStorage)(nil), storage)

	client, err := storage.Client()
	require.NoError(t, err)
	assert.NotNil(t, client)

	// Verify database file was created
	_, err = os.Stat(dbPath)
	assert.NoError(t, err)
}

func TestNewEntSqliteStorage_WithCustomConfig(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_custom.db")

	config := EntSqliteClientConfig{
		Path:                dbPath,
		ReadOnly:            false,
		SkipSchemaCreation:  false,
		EnableForeignKeys:   true,
		MaxOpenConns:        10,
		MaxIdleConns:        5,
		ConnMaxLifetime:     2 * time.Minute,
		EnableAutoMigration: true,
		DropColumns:         false,
		DropIndexes:         false,
	}

	storage, err := NewEntSqliteStorage(config)
	require.NoError(t, err)
	defer storage.Close()

	client, err := storage.Client()
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestEntSqliteStorage_SchemaCreation(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "schema_test.db")

	config := DefaultEntSqliteConfig(dbPath)
	storage, err := NewEntSqliteStorage(config)
	require.NoError(t, err)
	defer storage.Close()

	client, err := storage.Client()
	require.NoError(t, err)

	// Test that we can create a project (schema should be initialized)
	ctx := context.Background()
	project, err := client.Project.Create().
		SetName("test-project").
		SetRootPath("/test/path").
		Save(ctx)
	require.NoError(t, err)
	assert.Equal(t, "test-project", project.Name)
	assert.Equal(t, "/test/path", project.RootPath)
}

func TestSqlStorage_InterfaceMethods(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "sql_storage_test.db")

	config := DefaultEntSqliteConfig(dbPath)
	storage, err := NewEntSqliteStorage(config)
	require.NoError(t, err)
	defer storage.Close()

	ctx := context.Background()

	// Test DatabaseStats method
	stats := storage.DatabaseStats()
	assert.GreaterOrEqual(t, stats.MaxOpenConnections, 0)

	// Test Vacuum method
	err = storage.Vacuum(ctx)
	assert.NoError(t, err)

	// Test Analyze method
	err = storage.Analyze(ctx)
	assert.NoError(t, err)

	// Test BeginTx method
	tx, err := storage.BeginTx(ctx, nil)
	require.NoError(t, err)

	// Create a test entity within the transaction
	project, err := tx.Project.Create().
		SetName("tx-test").
		SetRootPath("/tx/test").
		Save(ctx)
	require.NoError(t, err)
	assert.Equal(t, "tx-test", project.Name)

	// Commit the transaction
	err = tx.Commit()
	assert.NoError(t, err)

	// Test WithTx method
	err = storage.WithTx(ctx, func(tx *ent.Tx) error {
		_, err := tx.Project.Create().
			SetName("withtx-test").
			SetRootPath("/withtx/test").
			Save(ctx)
		return err
	})
	assert.NoError(t, err)

	// Verify both projects exist
	client, err := storage.Client()
	require.NoError(t, err)

	projects, err := client.Project.Query().All(ctx)
	require.NoError(t, err)
	assert.Len(t, projects, 2)
}

func TestDefaultEntSqliteConfig(t *testing.T) {
	path := "/test/path/db.sqlite"
	config := DefaultEntSqliteConfig(path)

	assert.Equal(t, path, config.Path)
	assert.False(t, config.ReadOnly)
	assert.False(t, config.SkipSchemaCreation)
	assert.True(t, config.EnableForeignKeys)
	assert.Equal(t, 25, config.MaxOpenConns)
	assert.Equal(t, 5, config.MaxIdleConns)
	assert.Equal(t, 5*time.Minute, config.ConnMaxLifetime)
	assert.True(t, config.EnableAutoMigration)
	assert.False(t, config.DropColumns)
	assert.False(t, config.DropIndexes)
}
