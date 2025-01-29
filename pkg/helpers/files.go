package helpers

import (
	"os"

	"github.com/safedep/dry/log"
)

func EnsureDirExists(dir string) error {
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		log.Fatalf("failed to create data directory: %v", err)
		return err
	}
	return nil
}
