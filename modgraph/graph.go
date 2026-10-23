// Package modgraph builds the module graph of a Go project from the code on
// the disk: which module of the build list imports which. It reads the import
// blocks of the Go files of the main module and of each dependency module
// whose source the caller found, and the go.mod of a dependency with no
// source. It runs no go command, opens no network connection and needs no
// CGO.
//
// The graph may hold an edge that a build does not have, and it never misses
// an edge that a build has. A false absence of an edge hides a vulnerable
// module behind a false "unused". So a module whose imports Build cannot read
// is open: it may import any module.
package modgraph

import (
	"context"
	"errors"
	"fmt"
	"go/token"
	"go/version"
	"io/fs"
	"maps"
	"slices"
	"strings"
	"sync"

	"golang.org/x/mod/modfile"

	"github.com/safedep/code/fileindex/store"
	"github.com/safedep/code/projectgraph/modules"
)

// Version changes when Build gives another graph for the same input.
const Version = "1"

// StdPath is the module path of the node of the standard library. The
// standard library imports no module of a build list, so the node is a leaf.
const StdPath = "std"

// defaultMaxFiles is the file budget when Input.MaxFiles is zero.
const defaultMaxFiles = 200_000

// ErrUnpruned is the error of a main go.mod with a go directive below 1.17,
// or with none. Before module graph pruning, go.mod can leave out a module
// that the build needs, so the build list is not complete.
var ErrUnpruned = errors.New("modgraph: the go directive of go.mod is below 1.17, so go.mod need not list each module of the build")

// Module is one module of the build list. Path is the path that import
// statements use. With a replace directive, it is the path of the module
// that go.mod replaces, and the source is the source of the replacement.
type Module struct {
	Path    string
	Version string
}

func (m Module) String() string {
	if m.Version == "" {
		return m.Path
	}
	return m.Path + "@" + m.Version
}

// Source is what the caller found on the disk for one module of the build
// list.
type Source struct {
	Module Module
	// FS is the root of the source of the module, or nil.
	FS fs.FS
	// GoMod is the go.mod of the module version, which Build reads when FS is
	// nil. It is nil when the caller found none.
	GoMod []byte
	// Immutable marks a source that never changes for its module and version,
	// as a directory of the module cache. Build caches what it reads of an
	// immutable source. A vendor directory can change, so it is not immutable.
	Immutable bool
	// Origin is the module version whose code FS holds, when it is not
	// Module: the target of a replace directive. The cache keys a package
	// by Origin, so a fork never serves the imports of the module it
	// replaces.
	Origin Module
	// Sum is the h1 hash of Origin in go.sum, or "". It is a part of the
	// cache key, so a module version that a private proxy serves again with
	// other bytes gets a new entry.
	Sum string
}

// Input is the project that Build reads.
type Input struct {
	// MainFS is the root of the main module, the directory of its go.mod.
	MainFS fs.FS
	// Modules is the build list, with no entry for the main module.
	Modules []Source
	// Store caches the imports of the packages of immutable sources. A nil
	// Store keeps no cache.
	Store store.Store
	// MaxFiles caps the Go files that Build reads. Zero selects 200,000.
	MaxFiles int
}

// State says what Build knows of the imports of a module.
type State string

const (
	// StateImports is a module whose imports Build read, for each of its
	// packages that the build reaches.
	StateImports State = "imports"
	// StateRequires is a module with no source and a go.mod. Its require
	// lines show modules that it imports, but go.mod need not list each one:
	// the go.mod of the main module can give a module that a dependency
	// imports. So the node can import any module, as an open node.
	StateRequires State = "requires"
	// StateOpen is a module whose imports Build could not read. It can import
	// any module.
	StateOpen State = "open"
)

// Node is one module of the graph.
type Node struct {
	Module Module
	State  State
	// Reached is true when a path from the main module can reach the module.
	// A path through an open node can reach each module.
	Reached bool
	// Reason says why the node is not StateImports.
	Reason string
	// Walked is true when Build read a package of the module. The edges of
	// a module that Build did not walk are not known.
	Walked bool
}

// Open reports whether the node can import a module that it has no edge to.
func (n Node) Open() bool { return n.State != StateImports }

// Evidence names one import that gives an edge: the file, relative to the
// root of the module that imports, and the line.
type Evidence struct {
	File string
	Line int
}

// Edge joins a module to a module that it imports.
type Edge struct {
	From, To Module
	// Kind is StateImports for an import that Build read, and StateRequires
	// for a require line of the go.mod of a module with no source.
	Kind State
	// TestOnly marks an edge from the main module that only its test files
	// give.
	TestOnly bool
	Evidence Evidence
}

// Stats counts the work of Build.
type Stats struct {
	// Files is the count of Go files that Build read from the disk.
	Files int
	// Packages is the count of packages whose imports Build used, read or
	// from the cache.
	Packages int
	// CachedPackages is the count of packages that came from the cache.
	CachedPackages int
	// Truncated is true when the file budget ran out, so some module is open.
	Truncated bool
}

// Graph is the module graph of one main module.
type Graph struct {
	Version string
	// Main is the main module. Its Version is "".
	Main Module
	// Nodes holds the main module, each module of the build list, and the
	// standard library when the build imports it, sorted by path. A module
	// that no path reaches has no edge, because the build reads none of its
	// packages.
	Nodes []Node
	// Edges is sorted by the path of From, then of To.
	Edges []Edge
	Stats Stats

	index     sync.Once
	next      map[string][]string
	nodeIndex map[string]int
}

func (g *Graph) build() {
	g.index.Do(func() {
		g.next = map[string][]string{}
		for _, e := range g.Edges {
			g.next[e.From.Path] = append(g.next[e.From.Path], e.To.Path)
		}
		g.nodeIndex = make(map[string]int, len(g.Nodes))
		for i, n := range g.Nodes {
			g.nodeIndex[n.Module.Path] = i
		}
	})
}

// Node returns the node of a module path.
func (g *Graph) Node(path string) (Node, bool) {
	g.build()
	i, ok := g.nodeIndex[path]
	if !ok {
		return Node{}, false
	}
	return g.Nodes[i], true
}

// Reaches reports whether a path from one of the modules can reach a
// module. See Path.
func (g *Graph) Reaches(from []string, to string) bool { return g.Path(from, to) != nil }

// Path returns a shortest path of module paths from one of the modules to
// a module, or nil. A path that meets a node whose edges are not known, an
// open node or a module that Build did not walk, can reach each module:
// the path ends at that node, and to follows it. A module is not on a path
// from itself.
func (g *Graph) Path(from []string, to string) []string {
	g.build()
	prev := map[string]string{}
	var queue []string
	for _, f := range from {
		if _, ok := prev[f]; !ok && f != to {
			prev[f] = ""
			queue = append(queue, f)
		}
	}
	back := func(p string) []string {
		var out []string
		for ; p != ""; p = prev[p] {
			out = append(out, p)
		}
		slices.Reverse(out)
		return out
	}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		if n, ok := g.Node(p); !ok || n.Open() || !n.Walked {
			return append(back(p), to)
		}
		for _, q := range g.next[p] {
			if q == to {
				return append(back(p), to)
			}
			if _, seen := prev[q]; !seen {
				prev[q] = p
				queue = append(queue, q)
			}
		}
	}
	return nil
}

// Build reads the module graph of the main module in MainFS. It returns
// ErrUnpruned for a main go.mod before module graph pruning.
func Build(ctx context.Context, in Input) (*Graph, error) {
	if in.MainFS == nil {
		return nil, errors.New("modgraph: no main module")
	}
	data, err := fs.ReadFile(in.MainFS, "go.mod")
	if err != nil {
		return nil, fmt.Errorf("modgraph: read the go.mod of the main module: %w", err)
	}
	mf, err := modfile.ParseLax("go.mod", data, nil)
	if err != nil {
		return nil, fmt.Errorf("modgraph: parse the go.mod of the main module: %w", err)
	}
	if mf.Module == nil || mf.Module.Mod.Path == "" {
		return nil, errors.New("modgraph: the go.mod of the main module has no module directive")
	}
	if mf.Go == nil || version.Compare("go"+mf.Go.Version, "go1.17") < 0 {
		return nil, ErrUnpruned
	}
	b, err := newBuilder(ctx, in, mf.Module.Mod.Path)
	if err != nil {
		return nil, err
	}
	return b.run()
}

// module is the state of one node during a build.
type module struct {
	src  Source
	main bool
	node Node
	// walked is true when Build read a package of the module, so its edges
	// are those of the packages that the build reaches.
	walked bool
}

type pkgRef struct {
	mod *module
	dir string
}

type edgeKey struct{ from, to string }

type builder struct {
	ctx      context.Context
	in       Input
	maxFiles int
	fset     *token.FileSet

	main   *module
	std    *module
	byPath map[string]*module
	// paths holds the module paths of the build list and of the main module,
	// longest first, for the resolver.
	paths []string

	queue []pkgRef
	seen  map[pkgRef]bool
	edges map[edgeKey]*Edge
	stats Stats
}

func newBuilder(ctx context.Context, in Input, mainPath string) (*builder, error) {
	b := &builder{
		ctx:      ctx,
		in:       in,
		maxFiles: in.MaxFiles,
		fset:     token.NewFileSet(),
		byPath:   map[string]*module{},
		seen:     map[pkgRef]bool{},
		edges:    map[edgeKey]*Edge{},
	}
	if b.maxFiles <= 0 {
		b.maxFiles = defaultMaxFiles
	}
	b.main = &module{src: Source{Module: Module{Path: mainPath}, FS: in.MainFS}, main: true, node: Node{Module: Module{Path: mainPath}, State: StateImports}}
	b.byPath[mainPath] = b.main
	for _, s := range in.Modules {
		p := s.Module.Path
		switch {
		case p == "":
			return nil, errors.New("modgraph: a module of the build list has no path")
		case p == StdPath:
			return nil, fmt.Errorf("modgraph: the build list holds the reserved path %q", StdPath)
		case b.byPath[p] != nil:
			return nil, fmt.Errorf("modgraph: the build list holds %s twice", p)
		}
		m := &module{src: s, node: Node{Module: s.Module}}
		switch {
		case s.FS != nil:
			m.node.State = StateImports
		case s.GoMod != nil:
			m.node.State, m.node.Reason = StateRequires, "no source, so only the require lines of its go.mod give edges"
		default:
			m.node.State, m.node.Reason = StateOpen, "no source and no go.mod"
		}
		b.byPath[p] = m
	}
	for p := range b.byPath {
		b.paths = append(b.paths, p)
	}
	slices.SortFunc(b.paths, func(x, y string) int {
		if len(x) != len(y) {
			return len(y) - len(x)
		}
		return strings.Compare(x, y)
	})
	return b, nil
}

func (b *builder) run() (*Graph, error) {
	if err := b.walkMain(); err != nil {
		return nil, err
	}
	for len(b.queue) > 0 {
		if err := b.ctx.Err(); err != nil {
			return nil, err
		}
		ref := b.queue[0]
		b.queue = b.queue[1:]
		if err := b.walkPackage(ref); err != nil {
			return nil, err
		}
	}
	b.finish()
	return b.graph(), nil
}

// walkMain reads each package of the main module, with its test files. The
// walk does not follow a link to a directory, so a package that an import
// reaches through a link is queued as any other.
func (b *builder) walkMain() error {
	b.main.walked = true
	return fs.WalkDir(b.in.MainFS, ".", func(dir string, d fs.DirEntry, err error) error {
		if err != nil {
			b.setOpen(b.main, fmt.Sprintf("read %s: %v", dir, err))
			return fs.SkipDir
		}
		if !d.IsDir() {
			return nil
		}
		if dir != "." && (skippedDir(d.Name()) || nestedModule(b.in.MainFS, dir)) {
			return fs.SkipDir
		}
		if err := b.ctx.Err(); err != nil {
			return err
		}
		ref := pkgRef{mod: b.main, dir: dir}
		b.seen[ref] = true
		return b.readAndLink(ref)
	})
}

func (b *builder) walkPackage(ref pkgRef) error {
	if ref.mod.node.State == StateOpen {
		return nil
	}
	ref.mod.walked = true
	return b.readAndLink(ref)
}

func (b *builder) readAndLink(ref pkgRef) error {
	imps, err := b.packageImports(ref)
	switch {
	case errors.Is(err, errBudget):
		b.stats.Truncated = true
		b.setOpen(ref.mod, "the file budget ran out")
		return nil
	case errors.Is(err, errCache):
		return err
	case err != nil && b.ctx.Err() != nil:
		return b.ctx.Err()
	case err != nil:
		b.setOpen(ref.mod, err.Error())
		return nil
	}
	for _, imp := range imps {
		b.link(ref.mod, imp)
	}
	return nil
}

// link adds the edges of one import of a module, and queues the package
// that it names.
func (b *builder) link(from *module, imp importRef) {
	switch {
	case imp.Path == "C":
		return
	case imp.Path == "" || strings.HasPrefix(imp.Path, ".") || strings.HasPrefix(imp.Path, "/"):
		b.setOpen(from, fmt.Sprintf("%s imports the path %q, which the build cannot resolve", imp.File, imp.Path))
		return
	}
	cands := b.resolve(imp.Path)
	if len(cands) == 0 {
		if modules.IsStdPath(imp.Path) && !b.prefixed(imp.Path) {
			b.addEdge(from, b.stdNode(), imp, StateImports)
			return
		}
		b.setOpen(from, fmt.Sprintf("%s imports %s, which no module of the build list holds", imp.File, imp.Path))
		return
	}
	for _, c := range cands {
		if c.mod != from {
			b.addEdge(from, c.mod, imp, StateImports)
		}
		if c.provides {
			b.enqueue(pkgRef{mod: c.mod, dir: c.dir})
		}
	}
}

func (b *builder) enqueue(ref pkgRef) {
	if b.seen[ref] {
		return
	}
	b.seen[ref] = true
	b.queue = append(b.queue, ref)
}

func (b *builder) addEdge(from, to *module, imp importRef, kind State) {
	k := edgeKey{from.node.Module.Path, to.node.Module.Path}
	if e, ok := b.edges[k]; ok {
		if e.TestOnly && !imp.Test {
			e.TestOnly = false
			e.Evidence = Evidence{File: imp.File, Line: imp.Line}
		}
		return
	}
	b.edges[k] = &Edge{
		From: from.node.Module, To: to.node.Module, Kind: kind,
		TestOnly: from.main && imp.Test,
		Evidence: Evidence{File: imp.File, Line: imp.Line},
	}
}

func (b *builder) stdNode() *module {
	if b.std == nil {
		b.std = &module{node: Node{Module: Module{Path: StdPath}, State: StateImports}, walked: true}
		b.byPath[StdPath] = b.std
	}
	return b.std
}

func (b *builder) setOpen(m *module, reason string) {
	if m.node.State == StateOpen {
		return
	}
	m.node.State, m.node.Reason = StateOpen, reason
}

// finish adds the require edges of each reached module with no source, then
// sets Reached. Once a reached node is open, each node counts as reached,
// and a module that no walk read becomes open, because its edges are not
// known.
func (b *builder) finish() {
	expanded := map[*module]bool{}
	for {
		reached := b.reachable()
		added := false
		for _, p := range slices.Sorted(maps.Keys(b.byPath)) {
			m := b.byPath[p]
			if reached[m] && m.node.State == StateRequires && !expanded[m] {
				expanded[m] = true
				b.addRequires(m)
				added = true
			}
		}
		if !added {
			anyOpen := false
			for m := range reached {
				anyOpen = anyOpen || m.node.Open()
			}
			for _, m := range b.byPath {
				m.node.Reached = reached[m] || anyOpen
				if anyOpen && !m.walked && m.node.State == StateImports {
					b.setOpen(m, "only an open node reaches it, so the build read none of its packages")
				}
			}
			return
		}
	}
}

// addRequires adds an edge from a module with no source to each module of
// the build list that its go.mod requires.
func (b *builder) addRequires(m *module) {
	f, err := modfile.ParseLax("go.mod", m.src.GoMod, nil)
	if err != nil {
		b.setOpen(m, fmt.Sprintf("its go.mod does not parse: %v", err))
		return
	}
	for _, r := range f.Require {
		to := b.byPath[r.Mod.Path]
		if to == nil || to == m || to.main {
			continue
		}
		line := 0
		if r.Syntax != nil {
			line = r.Syntax.Start.Line
		}
		b.addEdge(m, to, importRef{File: "go.mod", Line: line}, StateRequires)
	}
}

// reachable returns the modules that the edges reach from the main module.
func (b *builder) reachable() map[*module]bool {
	next := map[string][]string{}
	for k := range b.edges {
		next[k.from] = append(next[k.from], k.to)
	}
	seen := map[*module]bool{b.main: true}
	queue := []*module{b.main}
	for len(queue) > 0 {
		m := queue[0]
		queue = queue[1:]
		for _, p := range next[m.node.Module.Path] {
			if to := b.byPath[p]; to != nil && !seen[to] {
				seen[to] = true
				queue = append(queue, to)
			}
		}
	}
	return seen
}

func (b *builder) graph() *Graph {
	g := &Graph{Version: Version, Main: b.main.node.Module, Stats: b.stats}
	for _, p := range slices.Sorted(maps.Keys(b.byPath)) {
		m := b.byPath[p]
		n := m.node
		n.Walked = m.walked
		g.Nodes = append(g.Nodes, n)
	}
	for _, e := range b.edges {
		g.Edges = append(g.Edges, *e)
	}
	slices.SortFunc(g.Edges, func(x, y Edge) int {
		if c := strings.Compare(x.From.Path, y.From.Path); c != 0 {
			return c
		}
		return strings.Compare(x.To.Path, y.To.Path)
	})
	return g
}
