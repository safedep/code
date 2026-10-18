package memstore

import (
	"testing"

	"github.com/safedep/code/fileindex/store"
	"github.com/safedep/code/fileindex/store/storetest"
)

func TestStore(t *testing.T) {
	storetest.Run(t, func(*testing.T) store.Store { return New() })
}
