package sink

import (
	"database/sql"
	"os"
)

type DB struct{ db *sql.DB }

func (d *DB) Write(v string) error {
	_, err := d.db.Exec("insert", v)
	return err
}

type Log struct{}

func (Log) Write() {
	os.Exit(1)
}
