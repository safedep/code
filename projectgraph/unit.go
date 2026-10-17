package projectgraph

import (
	"slices"

	"github.com/safedep/code/fileindex"
)

// unit is a set of files that share one namespace, as a Go package. A name
// that a file of the unit defines at its top level is in scope in every
// file of the unit. The members of a type can come from more than one file
// of the unit, as the methods of a Go type. Two files of the unit can define
// one name, as two files under other build tags, and a call of the name
// then reaches both.
type unit struct {
	key     string
	files   []string
	names   map[string][]string
	members map[string]map[string][]string
}

func newUnit(key string, files []string) *unit {
	return &unit{key: key, files: files, names: map[string][]string{}, members: map[string]map[string][]string{}}
}

// addUnit adds a unit of the files and the names that they define.
func (b *builder) addUnit(key string, files []string) *unit {
	u := newUnit(key, files)
	for _, p := range files {
		b.unitOf[p] = u
		for _, def := range b.byPath[p].Definitions {
			switch segs := fileindex.Segments(def.Key); {
			case len(segs) == 1:
				u.names[segs[0]] = append(u.names[segs[0]], def.Key)
			case len(segs) == 2 && def.Kind == fileindex.DefMethod:
				if u.members[segs[0]] == nil {
					u.members[segs[0]] = map[string][]string{}
				}
				u.members[segs[0]][segs[1]] = append(u.members[segs[0]][segs[1]], def.Key)
			}
		}
	}
	b.units[key] = u
	return u
}

// unitNames returns the definitions of the unit of the file with the name.
func (b *builder) unitNames(path, name string) []string {
	if u := b.unitOf[path]; u != nil {
		return u.names[name]
	}
	return nil
}

// member returns the method of a type that a file of the unit of the type
// defines.
func (b *builder) member(class, name string) (string, bool) {
	u := b.unitOf[fileindex.Unit(class)]
	segs := fileindex.Segments(class)
	if u == nil || len(segs) != 1 {
		return "", false
	}
	keys := u.members[segs[0]][name]
	if len(keys) == 0 {
		return "", false
	}
	return keys[0], true
}

// twins returns the other definitions of the unit with the name of a
// top-level definition or a member, as a function that two files under
// other build tags define.
func (b *builder) twins(key string) []string {
	u := b.unitOf[fileindex.Unit(key)]
	if u == nil {
		return nil
	}
	var keys []string
	switch segs := fileindex.Segments(key); len(segs) {
	case 1:
		keys = u.names[segs[0]]
	case 2:
		keys = u.members[segs[0]][segs[1]]
	}
	if len(keys) < 2 {
		return nil
	}
	return slices.DeleteFunc(slices.Clone(keys), func(k string) bool { return k == key })
}
