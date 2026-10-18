package helpers

import (
	"path"
	"strconv"
	"strings"
	"unicode"
)

// GoPackageName returns the name that a Go import path brings into scope
// when the import has no alias. It follows goimports: a major version
// element names the element before it, as in github.com/golang-jwt/jwt/v5,
// a go- prefix goes, as in go-openai, and the name ends at the first
// character that an identifier cannot hold, as in yaml.v3 or openai-go.
func GoPackageName(importPath string) string {
	importPath = strings.Trim(importPath, `"`)
	base := path.Base(importPath)
	if len(base) > 1 && base[0] == 'v' {
		if _, err := strconv.Atoi(base[1:]); err == nil {
			if dir := path.Dir(importPath); dir != "." {
				base = path.Base(dir)
			}
		}
	}
	base = strings.TrimPrefix(base, "go-")
	if i := strings.IndexFunc(base, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
	}); i >= 0 {
		base = base[:i]
	}
	return base
}
