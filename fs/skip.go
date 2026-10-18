package fs

import (
	"regexp"
	"strings"
)

// DefaultSkipDirs name the directories that hold installed code or caches
// at any depth of a project, as node_modules.
var DefaultSkipDirs = []string{
	"node_modules", "bower_components", ".git", ".hg", ".svn", "venv", ".venv", ".tox", ".nox",
	"__pycache__", ".mypy_cache", ".pytest_cache", "site-packages", ".next", ".nuxt",
}

// DefaultRootSkipDirs name the directories of vendored or built code directly
// under a project root. A package of the project can have one of these
// names below the root, as src/build, and its files are first-party.
var DefaultRootSkipDirs = []string{"vendor", "third_party", "env", "dist", "build", "out", "target", "coverage"}

// separator matches a path separator of any platform.
const separator = `[\\/]`

// DefaultSkipPatterns returns the exclude patterns of a project root: the
// DefaultSkipDirs at any depth below it and the DefaultRootSkipDirs
// directly under it. A root under a directory named build keeps its code.
func DefaultSkipPatterns(root string) []*regexp.Regexp {
	return []*regexp.Regexp{
		SkipPattern(root),
		regexp.MustCompile(`^` + regexp.QuoteMeta(root) + separator + alternation(DefaultRootSkipDirs) + `(?:` + separator + `|$)`),
	}
}

// SkipPattern returns a pattern that matches the DefaultSkipDirs at any
// depth below dir. The patterns of a project root do not apply to the walk
// of an import directory, so a caller adds one per import directory.
func SkipPattern(dir string) *regexp.Regexp {
	return regexp.MustCompile(`^` + regexp.QuoteMeta(dir) + separator + `(?:.*` + separator + `)?` + alternation(DefaultSkipDirs) + `(?:` + separator + `|$)`)
}

func alternation(names []string) string {
	quoted := make([]string, len(names))
	for i, name := range names {
		quoted[i] = regexp.QuoteMeta(name)
	}
	return `(?:` + strings.Join(quoted, "|") + `)`
}
