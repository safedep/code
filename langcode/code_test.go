package langcode

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAllHasEveryCodeOnce(t *testing.T) {
	seen := map[Code]bool{}
	for _, c := range All() {
		assert.False(t, seen[c], "duplicate code %q", c)
		seen[c] = true
	}
	assert.Len(t, seen, 10)
}

func TestFamily(t *testing.T) {
	assert.Equal(t, Javascript, Typescript.Family())
	assert.Equal(t, Java, Kotlin.Family())
	assert.Equal(t, Python, Python.Family())
	assert.Equal(t, Code("swift"), Code("swift").Family(), "a language with no partner is a family of its own")
	assert.True(t, Typescript.IsECMAScript())
	assert.False(t, Java.IsECMAScript())
}

func TestModuleSeparator(t *testing.T) {
	cases := map[Code]string{Python: ".", Java: ".", Kotlin: ".", CSharp: ".", PHP: `\`, Rust: "::", Go: "/", Javascript: "/", Typescript: "/", Ruby: "/"}
	for code, want := range cases {
		assert.Equal(t, want, code.ModuleSeparator(), code)
	}
}
