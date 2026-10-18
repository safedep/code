package testutil

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// Hasher hashes named byte sequences in order, so a set of golden files or
// of outputs has one hash that a version table records.
type Hasher struct {
	h hash.Hash
}

func NewHasher() *Hasher {
	return &Hasher{h: sha256.New()}
}

// Add hashes the name, the length and the bytes of one item.
func (h *Hasher) Add(t testing.TB, name string, data []byte) {
	t.Helper()
	_, err := fmt.Fprintf(h.h, "%s\n%d\n", name, len(data))
	require.NoError(t, err)
	_, err = h.h.Write(data)
	require.NoError(t, err)
}

// Sum returns the hash so far in hex.
func (h *Hasher) Sum() string {
	return hex.EncodeToString(h.h.Sum(nil))
}

// HashFiles returns the hash of the files, by base name, in the order
// given.
func HashFiles(t testing.TB, names []string) string {
	t.Helper()
	h := NewHasher()
	for _, name := range names {
		data, err := os.ReadFile(name)
		require.NoError(t, err)
		h.Add(t, filepath.Base(name), data)
	}
	return h.Sum()
}
