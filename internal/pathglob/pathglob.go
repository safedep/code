// Package pathglob matches slash paths against globs. A segment of a glob
// has the syntax of path.Match, and the segment ** matches zero or more
// segments.
package pathglob

import (
	"fmt"
	"path"
	"slices"
	"strings"
)

// Validate returns an error for a glob that path.Match rejects.
func Validate(glob string) error {
	for _, seg := range segments(glob) {
		if _, err := path.Match(seg, ""); err != nil {
			return fmt.Errorf("glob %q: %w", glob, err)
		}
	}
	return nil
}

// Match reports whether the path matches the glob. A bad glob matches
// nothing, so a caller validates the glob first.
func Match(glob, p string) bool {
	m := matcher{glob: segments(glob), segs: strings.Split(p, "/")}
	m.memo = make([]int8, (len(m.glob)+1)*(len(m.segs)+1))
	return m.match(0, 0)
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

// segments returns the segments of a glob with one ** for each run of **,
// because a run matches what one ** matches.
func segments(glob string) []string {
	segs := strings.Split(strings.TrimSuffix(strings.TrimPrefix(glob, "./"), "/"), "/")
	return slices.CompactFunc(segs, func(a, b string) bool { return a == "**" && b == "**" })
}

// matcher matches the segments of a path from each pair of a glob index
// and a path index at most once. Without the memo, a glob of n ** segments
// takes a time that grows as the power of n of the path length.
type matcher struct {
	glob, segs []string
	// memo holds 0 for a pair not seen, 1 for no match and 2 for a match.
	memo []int8
}

func (m *matcher) match(g, s int) bool {
	at := g*(len(m.segs)+1) + s
	if m.memo[at] != 0 {
		return m.memo[at] == 2
	}
	ok := m.matchFrom(g, s)
	m.memo[at] = 1
	if ok {
		m.memo[at] = 2
	}
	return ok
}

func (m *matcher) matchFrom(g, s int) bool {
	for g < len(m.glob) {
		if m.glob[g] == "**" {
			for i := s; i <= len(m.segs); i++ {
				if m.match(g+1, i) {
					return true
				}
			}
			return false
		}
		if s == len(m.segs) {
			return false
		}
		if ok, _ := path.Match(m.glob[g], m.segs[s]); !ok {
			return false
		}
		g, s = g+1, s+1
	}
	return s == len(m.segs)
}
