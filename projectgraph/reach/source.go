package reach

import (
	"errors"
	"fmt"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/projectgraph"
)

// Source is a node of the project that a query names as a root, as an
// entry point that an engineer knows and no root rule marks. Its key is
// the key of a definition or of a module top level of a first-party
// file, as app/server.py//main or src/api.ts//Router//handle. The zero
// Source is no source, and a constructor validates the key. Its JSON
// form is the key.
type Source struct {
	key string
}

// SourceByKey returns the source of a key, as app/server.py//main.
func SourceByKey(key string) (Source, error) {
	if err := validateKey(key, false); err != nil {
		return Source{}, fmt.Errorf("source %q: %w", key, err)
	}
	return Source{key: key}, nil
}

// SourceInFile returns the source of a definition by the path of its file
// and its qualified name, as app/server.py and main, or src/api.ts and
// Router, handle. With no name, the source is the module top level of the
// file.
func SourceInFile(path string, names ...string) (Source, error) {
	if path == "" {
		return Source{}, errors.New("source: no path")
	}
	return SourceByKey(fileindex.Join(path, names...))
}

// SourcesByKey returns the sources of the keys, as a list of a
// configuration gives them. The first bad key is the error.
func SourcesByKey(keys ...string) ([]Source, error) {
	sources := make([]Source, 0, len(keys))
	for _, key := range keys {
		s, err := SourceByKey(key)
		if err != nil {
			return nil, err
		}
		sources = append(sources, s)
	}
	return sources, nil
}

// Key returns the key of the source.
func (s Source) Key() string { return s.key }

func (s Source) String() string { return s.key }

// IsZero reports whether the source is the zero Source.
func (s Source) IsZero() bool { return s.key == "" }

func (s Source) MarshalText() ([]byte, error) { return []byte(s.key), nil }

func (s *Source) UnmarshalText(text []byte) error {
	source, err := SourceByKey(string(text))
	if err != nil {
		return err
	}
	*s = source
	return nil
}

// SourceMode says what the roots of a search are when a query names
// sources.
type SourceMode int

const (
	// SourcesAndRoots adds the sources to the roots that the rules marked.
	// It is the default.
	SourcesAndRoots SourceMode = iota
	// SourcesOnly searches from the sources alone. The root fallback does
	// not apply, so no result carries a root-fallback uncertainty.
	SourcesOnly
)

// RootQuery is the kind of a root that a query names as a source. No root
// rule marks one.
const RootQuery projectgraph.RootKind = "query"

// ErrUnknownSource is the error of a source that names no first-party
// node of the graph. Reach wraps one per bad source, so a typo never
// gives not-found.
var ErrUnknownSource = errors.New("unknown source")

// queryRoots returns a root of kind query for each source, in the order
// of the sources, or the error of every source that names no first-party
// node of the graph.
func (a *adjacency) queryRoots(sources []Source) ([]projectgraph.Root, error) {
	var roots []projectgraph.Root
	var errs []error
	for _, src := range sources {
		n := a.g.Nodes[src.Key()]
		switch {
		case src.IsZero():
			errs = append(errs, errors.New("a zero source"))
		case n == nil:
			errs = append(errs, fmt.Errorf("%w: no node has the key %s", ErrUnknownSource, src))
		case n.Kind == projectgraph.NodeExternal:
			errs = append(errs, fmt.Errorf("%w: %s is an external symbol, not a definition of the project", ErrUnknownSource, src))
		case fileindex.PackageOf(n.Path) != "":
			errs = append(errs, fmt.Errorf("%w: %s is in dependency code, not in the project", ErrUnknownSource, src))
		default:
			roots = append(roots, projectgraph.Root{Key: src.Key(), Kind: RootQuery, Rule: string(RootQuery), Site: projectgraph.Site{Path: n.Path, Line: n.Line}})
		}
	}
	return roots, errors.Join(errs...)
}

// rootKinds is the kind of the root at each node of a search. A node that
// several roots name keeps the kind of the first one.
type rootKinds map[int]projectgraph.RootKind

func (a *adjacency) rootKinds(roots []projectgraph.Root) rootKinds {
	kinds := make(rootKinds, len(roots))
	for _, r := range roots {
		if id, ok := a.id[r.Key]; ok {
			if _, seen := kinds[id]; !seen {
				kinds[id] = r.Kind
			}
		}
	}
	return kinds
}
