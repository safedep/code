package extra

import (
	"os"
	"testing"
)

var cleanups = []func(string) error{os.RemoveAll}

func TestRun(t *testing.T) {
	Run()
	cleanups[0]("/tmp/x")
}
