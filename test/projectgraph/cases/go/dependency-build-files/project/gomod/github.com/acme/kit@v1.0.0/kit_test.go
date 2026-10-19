package kit

import (
	"os"
	"testing"
)

func TestRun(t *testing.T) {
	os.RemoveAll(t.TempDir())
	Run("a")
}
