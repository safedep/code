package helpers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGoPackageName(t *testing.T) {
	for path, want := range map[string]string{
		"net/http":                               "http",
		`"fmt"`:                                  "fmt",
		"github.com/golang-jwt/jwt/v5":           "jwt",
		"gopkg.in/yaml.v3":                       "yaml",
		"github.com/sashabaranov/go-openai":      "openai",
		"github.com/anthropics/anthropic-sdk-go": "anthropic",
		"github.com/openai/openai-go/v2":         "openai",
		"v5":                                     "v5",
		"golang.org/x/crypto/sha3":               "sha3",
	} {
		assert.Equal(t, want, GoPackageName(path), path)
	}
}
