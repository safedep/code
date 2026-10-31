// Package distgraph builds the distribution graph of a Python project from
// the code on the disk: which installed distribution of a site-packages
// directory imports which. It reads the import statements of the Python
// files of the project and of each distribution, and the Requires-Dist
// lines of a distribution whose code it cannot read. It runs no Python,
// opens no network connection and reads no byte code.
//
// The graph may hold an edge that a run of the program does not have, and
// it never misses an edge that a run has. A false absence of an edge hides
// a vulnerable distribution behind a false "unused". So a distribution
// whose imports Build cannot read in full is open: it may import any
// distribution.
package distgraph

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
	"sync"

	"github.com/safedep/code/projectgraph/modules"
)

// Version changes when Build gives another graph for the same input.
const Version = "1"

// StdName is the name of the node of the standard library. The standard
// library imports no distribution, so the node is a leaf.
const StdName = "std"

// defaultMaxFiles is the file budget when Input.MaxFiles is zero.
const defaultMaxFiles = 200_000

// Distribution is one node of the graph: the project, an installed
// distribution, or the standard library. Name is the name that METADATA
// gives, and two names are one distribution when PEP 503 normalizes them
// to one.
type Distribution struct {
	Name    string
	Version string
}

func (d Distribution) String() string {
	if d.Version == "" {
		return d.Name
	}
	return d.Name + "@" + d.Version
}

// key returns the normalized name of the distribution, which the graph
// indexes its nodes by.
func (d Distribution) key() string { return modules.NormalizePythonName(d.Name) }

// Input is the project that Build reads.
type Input struct {
	// ProjectFS is the root of the project.
	ProjectFS fs.FS
	// ProjectName is the name of the node of the project. An empty name
	// gives "project".
	ProjectName string
	// SitePackages holds each site-packages directory of the environment
	// of the project, as the .dist-info directories describe its
	// distributions.
	SitePackages []fs.FS
	// MaxFiles caps the Python files that Build reads. Zero selects 200,000.
	MaxFiles int
	// TrustCompiled keeps a distribution with an extension module closed.
	// The code of a .so or a .pyd file can import any module, so Build
	// opens its distribution. A caller that reads the graph for a path
	// into a distribution, and not for the absence of one, can trust the
	// Python files alone.
	TrustCompiled bool
}

// State says what Build knows of the imports of a distribution.
type State string

const (
	// StateImports is a distribution whose imports Build read, in each of
	// its Python files.
	StateImports State = "imports"
	// StateRequires is a distribution with no Python file that Build could
	// read. Its Requires-Dist lines show distributions that it needs, but a
	// requirement is no import and an import needs no requirement. So the
	// node can import any distribution, as an open node.
	StateRequires State = "requires"
	// StateOpen is a distribution whose imports Build could not read in
	// full. It can import any distribution.
	StateOpen State = "open"
)

// Node is one distribution of the graph.
type Node struct {
	Distribution Distribution
	State        State
	// Reached is true when a path from the project can reach the
	// distribution. A path through an open node can reach each one.
	Reached bool
	// Reason says why the node is not StateImports.
	Reason string
	// Walked is true when Build read the files of the distribution. The
	// edges of a distribution that Build did not walk are not known.
	Walked bool
}

// Open reports whether the node can import a distribution that it has no
// edge to.
func (n Node) Open() bool { return n.State != StateImports }

// Evidence names one import that gives an edge: the file, relative to the
// root of the project or of the site-packages directory, and the line.
type Evidence struct {
	File string
	Line int
}

// Edge joins a distribution to a distribution that it imports.
type Edge struct {
	From, To Distribution
	// Kind is StateImports for an import that Build read, and
	// StateRequires for a Requires-Dist line of a distribution whose code
	// Build could not read.
	Kind State
	// TestOnly marks an edge from the project that only its test files
	// give.
	TestOnly bool
	Evidence Evidence
}

// Stats counts the work of Build.
type Stats struct {
	// Files is the count of Python files that Build read.
	Files int
	// Distributions is the count of distributions of the site-packages
	// directories.
	Distributions int
	// Truncated is true when the file budget ran out, so some node is open.
	Truncated bool
}

// Graph is the distribution graph of one project.
type Graph struct {
	Version string
	// Main is the project. Its Version is "".
	Main Distribution
	// Nodes holds the project, each distribution of the site-packages
	// directories, and the standard library when the code imports it,
	// sorted by name.
	Nodes []Node
	// Edges is sorted by the name of From, then of To.
	Edges []Edge
	// Problems names each .dist-info directory that Build could not read.
	// It describes no distribution, so the graph has no node for it.
	Problems []modules.Problem
	Stats    Stats

	index     sync.Once
	next      map[string][]string
	nodeIndex map[string]int
}

func (g *Graph) build() {
	g.index.Do(func() {
		g.next = map[string][]string{}
		for _, e := range g.Edges {
			g.next[e.From.key()] = append(g.next[e.From.key()], e.To.key())
		}
		g.nodeIndex = make(map[string]int, len(g.Nodes))
		for i, n := range g.Nodes {
			g.nodeIndex[n.Distribution.key()] = i
		}
	})
}

// Node returns the node of a distribution name, in any spelling that PEP
// 503 normalizes to the name of the node.
func (g *Graph) Node(name string) (Node, bool) {
	g.build()
	i, ok := g.nodeIndex[modules.NormalizePythonName(name)]
	if !ok {
		return Node{}, false
	}
	return g.Nodes[i], true
}

// Reaches reports whether a path from one of the distributions can reach a
// distribution. See Path.
func (g *Graph) Reaches(from []string, to string) bool { return g.Path(from, to) != nil }

// Path returns a shortest path of distribution names from one of the
// distributions to a distribution, or nil. A path that meets a node whose
// edges are not known, an open node or a distribution that Build did not
// walk, can reach each distribution: the path ends at that node, and to
// follows it. A distribution is not on a path from itself.
func (g *Graph) Path(from []string, to string) []string {
	g.build()
	to = modules.NormalizePythonName(to)
	prev := map[string]string{}
	var queue []string
	for _, f := range from {
		f = modules.NormalizePythonName(f)
		if _, ok := prev[f]; !ok && f != to {
			prev[f] = ""
			queue = append(queue, f)
		}
	}
	name := func(key string) string {
		if i, ok := g.nodeIndex[key]; ok {
			return g.Nodes[i].Distribution.Name
		}
		return key
	}
	back := func(p string) []string {
		var out []string
		for ; p != ""; p = prev[p] {
			out = append(out, name(p))
		}
		slices.Reverse(out)
		return out
	}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		if n, ok := g.Node(p); !ok || n.Open() || !n.Walked {
			return append(back(p), name(to))
		}
		for _, q := range g.next[p] {
			if q == to {
				return append(back(p), name(to))
			}
			if _, seen := prev[q]; !seen {
				prev[q] = p
				queue = append(queue, q)
			}
		}
	}
	return nil
}

// Build reads the distribution graph of the project in ProjectFS.
func Build(ctx context.Context, in Input) (*Graph, error) {
	if in.ProjectFS == nil {
		return nil, errors.New("distgraph: no project")
	}
	b, err := newBuilder(ctx, in)
	if err != nil {
		return nil, err
	}
	return b.run()
}

// dist is the state of one node during a build.
type dist struct {
	node Node
	// fsys is the site-packages directory that holds the distribution, or
	// the project for the project node.
	fsys fs.FS
	// meta is the distribution as its .dist-info describes it. The project
	// has none.
	meta modules.Distribution
	// files holds the Python files of the distribution that RECORD lists,
	// for the choice between two distributions of one namespace package.
	files map[string]bool
}

type edgeKey struct{ from, to string }

type builder struct {
	ctx      context.Context
	in       Input
	main     string
	std      string
	dists    map[string]*dist
	order    []string
	byTop    map[string][]string
	project  map[string]bool
	edges    map[edgeKey]*Edge
	problems []modules.Problem
	budget   int
	stats    Stats
	parser   *importReader
}

func newBuilder(ctx context.Context, in Input) (*builder, error) {
	reader, err := newImportReader()
	if err != nil {
		return nil, err
	}
	b := &builder{
		ctx:     ctx,
		in:      in,
		dists:   map[string]*dist{},
		byTop:   map[string][]string{},
		project: map[string]bool{},
		edges:   map[edgeKey]*Edge{},
		budget:  in.MaxFiles,
		parser:  reader,
	}
	if b.budget <= 0 {
		b.budget = defaultMaxFiles
	}
	name := in.ProjectName
	if name == "" {
		name = "project"
	}
	b.main = Distribution{Name: name}.key()
	b.std = Distribution{Name: StdName}.key()
	if b.main == b.std {
		return nil, fmt.Errorf("distgraph: the project holds the reserved name %q", StdName)
	}
	b.add(&dist{node: Node{Distribution: Distribution{Name: name}, State: StateImports}, fsys: in.ProjectFS})
	for _, sp := range in.SitePackages {
		found, problems := modules.PythonDistributions(sp)
		b.problems = append(b.problems, problems...)
		for _, d := range found {
			// The code of an editable install is the project, which the
			// walk of the project reads.
			if !d.Editable {
				b.addDistribution(sp, d)
			}
		}
	}
	b.stats.Distributions = len(b.order) - 1
	b.readProjectNames()
	return b, nil
}

func (b *builder) add(d *dist) {
	key := d.node.Distribution.key()
	b.dists[key] = d
	b.order = append(b.order, key)
}

// addDistribution adds a node for a distribution. Two .dist-info
// directories of one name, as a stale one next to the current one, keep
// the first and name the second as a problem: Build cannot tell which code
// the program runs.
func (b *builder) addDistribution(sp fs.FS, meta modules.Distribution) {
	d := &dist{node: Node{Distribution: Distribution{Name: meta.Name, Version: meta.Version}, State: StateImports}, fsys: sp, meta: meta, files: map[string]bool{}}
	key := d.node.Distribution.key()
	if prev, dup := b.dists[key]; dup {
		b.problems = append(b.problems, modules.Problem{Path: meta.Dir, Text: fmt.Sprintf("a second .dist-info of %s, next to %s", meta.Name, prev.meta.Dir)})
		prev.node.State, prev.node.Reason = StateOpen, "two .dist-info directories describe the distribution"
		return
	}
	for _, f := range meta.Files {
		if path.Ext(f) == ".py" {
			d.files[f] = true
		}
	}
	b.add(d)
	for _, top := range meta.TopLevel {
		b.byTop[top] = append(b.byTop[top], key)
	}
}

// readProjectNames reads the top-level names of the project: a module or a
// package at the root, and under a src directory.
func (b *builder) readProjectNames() {
	for _, dir := range []string{".", "src"} {
		entries, err := fs.ReadDir(b.in.ProjectFS, dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			name := e.Name()
			switch {
			case e.IsDir():
				if _, err := fs.Stat(b.in.ProjectFS, path.Join(dir, name, "__init__.py")); err == nil || dir == "src" {
					b.project[name] = true
				}
			case strings.HasSuffix(name, ".py"):
				b.project[strings.TrimSuffix(name, ".py")] = true
			}
		}
	}
}

func (b *builder) run() (*Graph, error) {
	if err := b.walk(b.main); err != nil {
		return nil, err
	}
	reached := b.reach()
	// A reached open node can import any distribution, so each node is
	// reached, and the edges of each one matter.
	if slices.ContainsFunc(b.order, func(key string) bool { return reached[key] && b.dists[key].node.Open() }) {
		for _, key := range b.order {
			if err := b.walk(key); err != nil {
				return nil, err
			}
			reached[key] = true
		}
	}
	b.requires()
	return b.graph(reached), nil
}

// reach marks each node that a path from the project reaches, through the
// edges that Build has read so far.
func (b *builder) reach() map[string]bool {
	reached := map[string]bool{b.main: true}
	queue := []string{b.main}
	for len(queue) > 0 {
		from := queue[0]
		queue = queue[1:]
		for _, e := range b.edges {
			to := e.To.key()
			if e.From.key() == from && !reached[to] {
				reached[to] = true
				queue = append(queue, to)
			}
		}
	}
	return reached
}

// walk reads the Python files of a node and resolves its imports. A
// distribution that an import reaches is walked in turn, until no new
// node is reached.
func (b *builder) walk(key string) error {
	queue := []string{key}
	for len(queue) > 0 {
		key := queue[0]
		queue = queue[1:]
		d := b.dists[key]
		if d == nil || d.node.Walked {
			continue
		}
		d.node.Walked = true
		targets, err := b.readNode(d)
		if err != nil {
			return err
		}
		queue = append(queue, targets...)
	}
	return nil
}

// open marks a node open, with the first reason.
func (b *builder) open(d *dist, reason string) {
	if d.node.State == StateImports {
		d.node.State, d.node.Reason = StateOpen, reason
	}
}

// readNode reads the Python files of a node and returns the keys of the
// distributions that its imports reach.
func (b *builder) readNode(d *dist) ([]string, error) {
	files, compiled, err := b.pythonFiles(d)
	if err != nil {
		return nil, err
	}
	if len(compiled) > 0 && !b.in.TrustCompiled {
		b.open(d, "the extension module "+compiled[0]+" can import any module")
	}
	if len(files) == 0 && d.node.Distribution.key() != b.main {
		d.node.State, d.node.Reason = StateRequires, "no Python file"
	}
	var targets []string
	for _, file := range files {
		if err := b.ctx.Err(); err != nil {
			return nil, err
		}
		if b.stats.Files >= b.budget {
			b.stats.Truncated = true
			b.open(d, "the file budget ran out")
			break
		}
		b.stats.Files++
		data, err := fs.ReadFile(d.fsys, file)
		if err != nil {
			b.open(d, "cannot read "+file)
			continue
		}
		imports, parsed := b.parser.read(b.ctx, data)
		if !parsed {
			b.open(d, file+" does not parse")
		}
		testOnly := d.node.Distribution.key() == b.main && modules.PythonTestPath(file)
		for _, imp := range imports {
			if imp.dynamic {
				b.open(d, fmt.Sprintf("%s:%d imports a computed name", file, imp.line))
				continue
			}
			for _, to := range b.resolve(d, imp, file) {
				b.edge(d, to, testOnly, Evidence{File: file, Line: imp.line})
				targets = append(targets, to)
			}
		}
	}
	return targets, nil
}

// resolve returns the keys of the nodes that an import names. The first
// segment of the module path decides: a name of the project, a top-level
// name of a distribution, or a module of the standard library. A name of
// a namespace package that two distributions share goes to the one whose
// RECORD lists a file under the module path, else to each one. An import
// that nothing holds opens its importer, unless a try statement guards it
// against an ImportError.
func (b *builder) resolve(d *dist, imp importRef, file string) []string {
	first, _, _ := strings.Cut(imp.module, ".")
	self := d.node.Distribution.key()
	switch {
	case first == "":
		return nil
	case self == b.main && b.project[first]:
		return nil
	case slices.Contains(d.meta.TopLevel, first):
		return nil
	}
	if keys := b.byTop[first]; len(keys) > 0 {
		return b.namespaceOwners(keys, imp.module)
	}
	if self != b.main && b.project[first] {
		return []string{b.main}
	}
	if stdlibModules[first] {
		b.addStd()
		return []string{b.std}
	}
	if !imp.guarded {
		b.open(d, fmt.Sprintf("%s:%d imports %s, which no distribution holds", file, imp.line, imp.module))
	}
	return nil
}

// namespaceOwners returns the distributions of a top-level name that hold
// a module path, as google.cloud.storage: the ones whose RECORD lists a
// file under the longest prefix of the path that any of them holds. With
// no RECORD, each one may hold it.
func (b *builder) namespaceOwners(keys []string, module string) []string {
	if len(keys) == 1 {
		return keys
	}
	parts := strings.Split(module, ".")
	for n := len(parts); n > 1; n-- {
		prefix := path.Join(parts[:n]...)
		var owners []string
		for _, key := range keys {
			files := b.dists[key].files
			if files[prefix+".py"] || files[prefix+"/__init__.py"] {
				owners = append(owners, key)
			}
		}
		if len(owners) > 0 {
			return owners
		}
	}
	return keys
}

// addStd adds the node of the standard library on its first import.
func (b *builder) addStd() {
	if _, ok := b.dists[b.std]; !ok {
		b.add(&dist{node: Node{Distribution: Distribution{Name: StdName}, State: StateImports, Walked: true}})
	}
}

func (b *builder) edge(from *dist, to string, testOnly bool, ev Evidence) {
	key := edgeKey{from.node.Distribution.key(), to}
	if e, ok := b.edges[key]; ok {
		if e.TestOnly && !testOnly {
			e.TestOnly, e.Evidence = false, ev
		}
		return
	}
	b.edges[key] = &Edge{From: from.node.Distribution, To: b.dists[to].node.Distribution, Kind: StateImports, TestOnly: testOnly, Evidence: ev}
}

// requires adds the Requires-Dist lines of each open distribution as
// evidence of edges that Build could not read.
func (b *builder) requires() {
	for _, key := range b.order {
		d := b.dists[key]
		if !d.node.Open() {
			continue
		}
		for _, name := range d.meta.Requires {
			to := Distribution{Name: name}.key()
			if _, ok := b.dists[to]; !ok || to == key {
				continue
			}
			if _, ok := b.edges[edgeKey{key, to}]; !ok {
				b.edges[edgeKey{key, to}] = &Edge{From: d.node.Distribution, To: b.dists[to].node.Distribution, Kind: StateRequires, Evidence: Evidence{File: path.Join(d.meta.Dir, "METADATA")}}
			}
		}
	}
}

func (b *builder) graph(reached map[string]bool) *Graph {
	g := &Graph{Version: Version, Main: b.dists[b.main].node.Distribution, Problems: b.problems, Stats: b.stats}
	for _, key := range b.order {
		n := b.dists[key].node
		n.Reached = reached[key]
		g.Nodes = append(g.Nodes, n)
	}
	slices.SortFunc(g.Nodes, func(a, c Node) int { return strings.Compare(a.Distribution.key(), c.Distribution.key()) })
	for _, e := range b.edges {
		g.Edges = append(g.Edges, *e)
	}
	slices.SortFunc(g.Edges, func(a, c Edge) int {
		if r := strings.Compare(a.From.key(), c.From.key()); r != 0 {
			return r
		}
		return strings.Compare(a.To.key(), c.To.key())
	})
	return g
}
