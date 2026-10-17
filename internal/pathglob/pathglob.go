// Package pathglob matches slash paths against globs. A segment of a glob
// has the syntax of path.Match, and the segment ** matches zero or more
// segments.
package pathglob

import (
	"fmt"
	"path"
	"strings"
)

// Validate returns an error for a glob that path.Match rejects.
func Validate(glob string) error {
	for _, seg := range strings.Split(clean(glob), "/") {
		if _, err := path.Match(seg, ""); err != nil {
			return fmt.Errorf("glob %q: %w", glob, err)
		}
	}
	return nil
}

// Match reports whether the path matches the glob. A bad glob matches
// nothing, so a caller validates the glob first.
func Match(glob, p string) bool {
	return match(strings.Split(clean(glob), "/"), strings.Split(p, "/"))
}

// MatchAny reports whether the path matches a glob of the list. A glob with
// no "/" matches any one segment of the path, as a file name or a directory
// name.
func MatchAny(globs []string, p string) bool {
	segs := strings.Split(p, "/")
	for _, g := range globs {
		if strings.Contains(g, "/") {
			if Match(g, p) {
				return true
			}
			continue
		}
		for _, s := range segs {
			if ok, _ := path.Match(g, s); ok {
				return true
			}
		}
	}
	return false
}

func clean(glob string) string {
	return strings.TrimSuffix(strings.TrimPrefix(glob, "./"), "/")
}

func match(glob, segs []string) bool {
	for len(glob) > 0 {
		if glob[0] == "**" {
			for i := 0; i <= len(segs); i++ {
				if match(glob[1:], segs[i:]) {
					return true
				}
			}
			return false
		}
		if len(segs) == 0 {
			return false
		}
		if ok, _ := path.Match(glob[0], segs[0]); !ok {
			return false
		}
		glob, segs = glob[1:], segs[1:]
	}
	return len(segs) == 0
}
