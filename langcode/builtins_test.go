package langcode

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsBuiltin(t *testing.T) {
	tests := []struct {
		code Code
		name string
		want bool
	}{
		{Python, "print", true},
		{Python, "super", true},
		{Python, "cb", false},
		{Javascript, "console", true},
		{Typescript, "require", true},
		{Go, "len", true},
		{Java, "System", true},
		{Ruby, "puts", true},
		{Ruby, "load_data", false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, IsBuiltin(tt.code, tt.name), "%s %s", tt.code, tt.name)
	}
}
