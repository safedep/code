package projectgraph

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalledName(t *testing.T) {
	tests := []struct {
		receiver string
		name     string
		ok       bool
	}{
		{"html.NewTokenizer(r)", "NewTokenizer", true},
		{"html.NewTokenizer(io.MultiReader(a, b))", "NewTokenizer", true},
		{"dep.NewThing().Next()", "Next", true},
		{"pkg.New[int](x)", "New", true},
		{"Vec::<u8>::new()", "new", true},
		{"smallvec::new_thing()", "new_thing", true},
		{"yaml.load(f)", "load", true},
		{"z", "", false},
		{"a.b", "", false},
		{"(a)", "", false},
		{"f(x))", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.receiver, func(t *testing.T) {
			name, ok := calledName(tt.receiver)
			assert.Equal(t, tt.ok, ok)
			if tt.ok {
				assert.Equal(t, tt.name, name)
			}
		})
	}
}
