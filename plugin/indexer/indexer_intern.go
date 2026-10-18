package indexer

import "github.com/safedep/code/fileindex"

// interner makes the indexes of a run share one copy of each string. An
// index holds the key of its caller in each call and the same targets and
// kinds in many records, and a JSON decode gives each of them its own
// copy. The result of a run holds every index, so the copies add up.
type interner map[string]string

// intern returns the shared copy of a string. An empty string is shared by
// the language.
func (t interner) intern(s string) string {
	if s == "" {
		return s
	}
	if v, ok := t[s]; ok {
		return v
	}
	t[s] = s
	return s
}

// fit copies a slice into one of its length. An append that builds the
// records of an index can leave up to half of the room free.
func fit[T any](s []T) []T {
	if cap(s) == len(s) {
		return s
	}
	return append(make([]T, 0, len(s)), s...)
}

// index interns every string of the index in place, and fits its record
// slices to their length.
func (t interner) index(ix *fileindex.Index) {
	ix.Imports = fit(ix.Imports)
	ix.Exports = fit(ix.Exports)
	ix.Definitions = fit(ix.Definitions)
	ix.Classes = fit(ix.Classes)
	ix.Calls = fit(ix.Calls)
	ix.References = fit(ix.References)
	ix.Unresolved = fit(ix.Unresolved)
	ix.EachString(t.intern)
}
