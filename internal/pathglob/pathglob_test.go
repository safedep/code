package pathglob

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatch(t *testing.T) {
	tests := []struct {
		glob string
		path string
		want bool
	}{
		{glob: "packages/*", path: "packages/a", want: true},
		{glob: "./packages/*", path: "packages/a", want: true},
		{glob: "packages/*", path: "packages/a/b", want: false},
		{glob: "app/**/route.ts", path: "app/route.ts", want: true},
		{glob: "app/**/route.ts", path: "app/x/y/route.ts", want: true},
		{glob: "pages/api/**", path: "pages/api/a/b.ts", want: true},
		{glob: "**/*_test.py", path: "a/b_test.py", want: true},
		{glob: "**/*_test.py", path: "b.py", want: false},
		{glob: "[", path: "[", want: false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, Match(tt.glob, tt.path), "%s %s", tt.glob, tt.path)
	}
}

func TestMatchAny(t *testing.T) {
	globs := []string{"tests", "test_*.py", "src/**/*.spec.ts"}
	assert.True(t, MatchAny(globs, "pkg/tests/a.py"))
	assert.True(t, MatchAny(globs, "pkg/test_a.py"))
	assert.True(t, MatchAny(globs, "src/a/b.spec.ts"))
	assert.False(t, MatchAny(globs, "lib/b.spec.ts"))
	assert.False(t, MatchAny(globs, "pkg/a.py"))
}

func TestValidate(t *testing.T) {
	assert.NoError(t, Validate("app/**/route.ts"))
	assert.Error(t, Validate("app/[/x"))
}
