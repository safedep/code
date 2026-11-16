package langcode

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRustFromFile(t *testing.T) {
	cases := []struct {
		path string
		mods []string
		want string
	}{
		{"self::tls::conn", []string{"client"}, "self::client::tls::conn"},
		{"super::tls", []string{"client"}, "self::tls"},
		{"super::sub", []string{"outer", "inner"}, "self::outer::sub"},
		{"super::super::x", []string{"inner"}, "super::x"},
		{"super", []string{"tests"}, "self"},
		{"crate::store::disk", []string{"client"}, "crate::store::disk"},
		{"dep::html", []string{"client"}, "dep::html"},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			assert.Equal(t, c.want, RustFromFile(c.path, c.mods))
		})
	}
}
