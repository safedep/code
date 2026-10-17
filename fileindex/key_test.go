package fileindex

import (
	"testing"

	"github.com/safedep/code/langcode"
	"github.com/stretchr/testify/assert"
)

func TestKeys(t *testing.T) {
	key := "src/a.py//Parser//parse"
	assert.Equal(t, key, Join("src/a.py", "Parser", "parse"))
	assert.Equal(t, "src/a.py", Join("src/a.py"))
	assert.Equal(t, "src/a.py", Unit(key))
	assert.Equal(t, []string{"Parser", "parse"}, Segments(key))
	assert.Nil(t, Segments("src/a.py"))
	assert.Equal(t, "parse", Last(key))
	assert.Equal(t, "src/a.py", Last("src/a.py"))
	assert.Equal(t, "src/a.py//Parser", Parent(key))
	assert.Equal(t, "src/a.py", Parent("src/a.py"))
	assert.True(t, HasKeyPrefix(key, "src/a.py//Parser"))
	assert.True(t, HasKeyPrefix(key, key))
	assert.False(t, HasKeyPrefix(key, "src/a.py//Pars"))
}

func TestNamespace(t *testing.T) {
	assert.Equal(t, "yaml//loader", Namespace(langcode.Python, "yaml.loader"))
	assert.Equal(t, "fs//promises", Namespace(langcode.Javascript, "fs/promises"))
	assert.Equal(t, ".//utils", Namespace(langcode.Typescript, "./utils"))
	assert.Equal(t, "java//util//List", Namespace(langcode.Java, "java.util.List"))
	assert.Equal(t, "net//http", Namespace(langcode.Go, "net/http"))
	assert.Equal(t, "System//IO", Namespace(langcode.CSharp, "System.IO"))
	assert.Equal(t, "App//Services//Loader", Namespace(langcode.PHP, `\App\Services\Loader`))
	assert.Equal(t, "crate//loader", Namespace(langcode.Rust, "crate::loader"))
	assert.Equal(t, "net//http", Namespace(langcode.Ruby, "net/http"))
	assert.Equal(t, "Net//HTTP", Namespace(langcode.Ruby, "Net::HTTP"))
}

func TestAnonymous(t *testing.T) {
	assert.Equal(t, "<anonymous:12>", Anonymous(12, 1))
	assert.Equal(t, "<anonymous:12#2>", Anonymous(12, 2))
	assert.True(t, IsAnonymous(Anonymous(3, 1)))
	assert.False(t, IsAnonymous("handler"))
}

func TestJoinNonEmptyAndCompareBool(t *testing.T) {
	assert.Equal(t, "lib", JoinNonEmpty("lib", ""))
	assert.Equal(t, "lib//load", JoinNonEmpty("lib", "load"))
	assert.Equal(t, -1, CompareBool(false, true))
	assert.Equal(t, 0, CompareBool(true, true))
	assert.Equal(t, 1, CompareBool(true, false))
}
