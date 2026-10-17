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
}
