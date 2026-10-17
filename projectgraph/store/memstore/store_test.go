package memstore

import (
	"testing"

	"github.com/safedep/code/projectgraph/store"
	"github.com/safedep/code/projectgraph/store/storetest"
)

func TestStore(t *testing.T) {
	storetest.Run(t, func(*testing.T) store.Store { return New() })
}
