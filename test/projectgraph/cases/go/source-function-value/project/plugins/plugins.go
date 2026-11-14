package plugins

import (
	"os"

	"example.com/app/registry"
)

func init() {
	registry.Register("rm", os.Remove)
}

func unused() {
	os.Exit(1)
}
