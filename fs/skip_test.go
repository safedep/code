package fs

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultSkipPatterns(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "ci", "build", "app")
	patterns := DefaultSkipPatterns(root)
	skipped := func(p string) bool {
		for _, re := range patterns {
			if re.MatchString(p) {
				return true
			}
		}
		return false
	}

	tests := []struct {
		path string
		skip bool
	}{
		{filepath.Join(root, "main.py"), false},
		{filepath.Join(root, "src", "builder.py"), false},
		{filepath.Join(root, "node_modules", "yaml", "index.js"), true},
		{filepath.Join(root, "web", "node_modules"), true},
		{filepath.Join(root, ".venv", "lib", "yaml.py"), true},
		{filepath.Join(root, "pkg", "__pycache__", "a.pyc"), true},
		{filepath.Join(root, "build", "app.js"), true},
		{filepath.Join(root, "src", "build", "helper.py"), false},
		{filepath.Join(root, "env"), true},
		{filepath.Join(root, "app", "env", "settings.py"), false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.skip, skipped(tt.path), tt.path)
	}
}
