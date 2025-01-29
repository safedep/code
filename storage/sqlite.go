package storage

import (
	"context"
	"fmt"
	"os"
	"path"

	"github.com/safedep/code/core"
	"github.com/safedep/code/ent"
	"github.com/safedep/code/pkg/helpers"
	"github.com/safedep/dry/log"
)

const DbDirectory = "data"

type SqliteStorage struct {
	storageId string
	client    *ent.Client
}

var _ core.CodeAnalysisStorage = (*SqliteStorage)(nil)

func NewSqliteStorage(storageId string) *SqliteStorage {
	return &SqliteStorage{
		storageId: storageId,
	}
}

func (s *SqliteStorage) Client() (*ent.Client, error) {
	if s.client == nil {
		return nil, fmt.Errorf("client not initialized")
	}
	return s.client, nil
}

func (s *SqliteStorage) Init(ctx context.Context) error {
	_ = helpers.EnsureDirExists(DbDirectory)
	dbPath := path.Join(DbDirectory, s.storageId+".db")

	var err error
	if _, err = os.Stat(dbPath); err == nil {
		if err = os.Remove(dbPath); err != nil {
			log.Fatalf("failed to remove existing database file: %v", err)
			return err
		}
		log.Debugf("Existing database file deleted")
	}

	s.client, err = ent.Open("sqlite3", "file:"+dbPath+"?_fk=1")
	if err != nil {
		log.Fatalf("failed connecting to database: %v", err)
		return err
	}

	if err := s.client.Schema.Create(context.Background()); err != nil {
		log.Fatalf("failed creating schema resources: %v", err)
		return err
	}

	log.Debugf("Database schema created successfully!")
	return nil
}

func (s *SqliteStorage) Close() error {
	return s.client.Close()
}
