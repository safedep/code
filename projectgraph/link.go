package projectgraph

import (
	"cmp"
	"context"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"time"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/fileindex/roots"
	"github.com/safedep/code/internal/pathglob"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/projectgraph/modules"
)

const (
	defaultMaxEdges      = 5_000_000
	defaultMaxExportHops = 10
)

// Options configures the link step.
type Options struct {
	// Root holds the manifests of the project, as pyproject.toml. A nil
	// Root reads no manifest.
	Root fs.FS
	// SysPath lists more Python import roots, relative to the project root.
	SysPath []string
	// Files is the status of each file that the index step reports. A path
	// with no index and a status that is not complete, as missing or
	// failed, becomes a module node with that status.
	Files map[string]fileindex.FileStatus
	// TestPaths marks the test files. A glob with no "/" matches one part
	// of the path, as tests or test_*.py.
	TestPaths []string
	// Rules go over the embedded root rules by id.
	Rules []roots.Rule
	// Units holds the files of each package unit of dependency code, by
	// unit, as Root holds the files of the project. The root of each one is
	// the directory that the paths inside the package start from, as
	// node_modules/lodash for npm:lodash@4.17.21//template.js. A unit with
	// no entry reads no manifest.
	Units map[string]fs.FS
	// Surface links an external node to each exported symbol of each unit
	// of dependency code, as an import of the package by its name would. A
	// search can then start from the symbols of a package, as a summary of
	// the package does.
	Surface bool
	// MaxEdges caps the edges of the graph. Past it the link step adds no
	// edge and records a truncation. Zero selects 5,000,000.
	MaxEdges int
	// MaxExportHops caps the hops of a walk through re-exports, bases and
	// the modules of a Go package. Zero selects 10.
	MaxExportHops int
}

// Build links the indexes of one project into a graph. Every index must have
// the Version of this build, and no two indexes may share a path. The graph
// does not depend on the order of the indexes.
//
// An index with a Unit is a file of dependency code. The link step resolves
// the imports of a unit inside the unit, as a project of its own. An import
// that no file of its own project or unit gives stays an external node, as
// lodash//template, and a package edge joins that node to the definition of
// each unit that gives it. Dependency code holds no root.
func Build(ctx context.Context, indexes []*fileindex.Index, opts Options) (*Graph, error) {
	start := time.Now()
	b, err := newBuilder(indexes, opts)
	if err != nil {
		return nil, err
	}
	if err := b.linkAll(ctx); err != nil {
		return nil, err
	}
	if err := b.markRoots(ctx); err != nil {
		return nil, err
	}
	g := b.finish()
	g.Stats.Duration = time.Since(start)
	return g, nil
}

type nameKey struct{ module, name string }

type builder struct {
	opts   Options
	g      *Graph
	byPath map[string]*fileindex.Index
	paths  []string
	scopes map[string]*scope
	test   map[string]bool
	rules  []roots.Rule

	// project is the space of the project, spaces holds the space of the
	// project and of each unit by unit, and ordered holds them in the
	// order of their units, the project first.
	project *space
	spaces  map[string]*space
	ordered []*space
	// origins holds the imports that each external key comes from, and
	// linked the external keys that linkPackages read. named holds the
	// units that an import of the project or of another unit names, and
	// hasProject whether the build has a file of the project.
	origins    map[string][]origin
	linked     map[string]bool
	named      map[string]bool
	hasProject bool
	// provisions holds the providers of each module by language family and
	// form.
	provisions map[provisionKey][]provision

	edges       *edgeSet
	unresolved  map[UnresolvedSite]bool
	truncations map[Truncation]bool
	problems    []modules.Problem

	names    map[nameKey]resolution
	visiting map[nameKey]bool
	bindings map[string]string
	classes  map[string]classRecord
	// defs holds the definition records by key.
	defs map[string]*fileindex.Definition
	// reading holds the variables whose value a walk reads now, so a cycle
	// of variables across files stops.
	reading map[string]bool

	// groupMembers holds the members of each group of aliases by its first
	// alias, and nodesByParent the keys of the nodes by their parent.
	groupMembers  map[string]map[string]string
	nodesByParent map[string][]string
	// groupRecords holds the class records of each group of aliases, and
	// externalBases the first external base of each class, or "".
	groupRecords  map[string][]classRecord
	externalBases map[string]string
	// state holds the state of each language that keeps one, as the crates
	// of Rust. langState reads it.
	state  map[langcode.Code]any
	units  map[string]*unit
	unitOf map[string]*unit
	// namespaces holds the table of the units of each language family whose
	// files declare a package or a namespace.
	namespaces map[langcode.Code]*namespaces

	mros     map[string][]string
	children map[string][]string
	// aliases holds the definitions of each class that the language joins
	// to one class, the class too.
	aliases map[string][]string
	// extensionDefs holds the extension functions by family and name.
	extensionDefs map[familyName][]string
	// open holds the classes whose base can be any class.
	open []string
	// narrowed holds the callees of the calls whose receiver a class
	// narrowed, by site. Such a call gets no field or name edge, and the
	// link step owns its record and the records of its receiver, as s.enc
	// of s.enc.Encode(). A chain of calls, as a.b().c(), has one site, and
	// the call of c stays a record when the class of a narrows the call of
	// b.
	narrowed map[Site][]string
	// structural holds the narrowed calls whose type an object of another
	// class can satisfy, so they keep their field edges.
	structural map[siteCall]bool
	// loads holds whether the module top level of each file runs code.
	loads map[string]bool
}

// langState returns the state of a language of the build. The first call
// makes it. Each language file declares its own state type, so the builder
// holds no field of one language.
func langState[T any](b *builder, code langcode.Code) *T {
	if s, ok := b.state[code]; ok {
		return s.(*T)
	}
	s := new(T)
	b.state[code] = s
	return s
}

type classRecord struct {
	ix    *fileindex.Index
	class fileindex.Class
}

func newBuilder(indexes []*fileindex.Index, opts Options) (*builder, error) {
	opts.MaxEdges = cmp.Or(opts.MaxEdges, defaultMaxEdges)
	opts.MaxExportHops = cmp.Or(opts.MaxExportHops, defaultMaxExportHops)
	if err := opts.validate(); err != nil {
		return nil, err
	}
	b := newTables(indexes, opts)
	if err := b.addIndexes(indexes); err != nil {
		return nil, err
	}
	for p, status := range opts.Files {
		if _, ok := b.byPath[p]; !ok && !status.Complete() {
			b.paths = append(b.paths, p)
		}
	}
	slices.Sort(b.paths)
	if err := b.addSpaces(); err != nil {
		return nil, err
	}
	for _, p := range b.paths {
		b.test[p] = pathglob.MatchAny(opts.TestPaths, b.spaceOf(p).inner(p))
	}
	for _, p := range profiles {
		if p.units != nil {
			p.units(b)
		}
	}
	for _, sp := range b.ordered {
		for _, p := range profiles {
			if p.resolver != nil && (sp == b.project || p.dependencyCode) {
				sp.resolvers[p.code] = p.resolver(b, sp)
			}
		}
	}
	return b, nil
}

// validate checks the test paths and the root rules of the options.
func (opts Options) validate() error {
	for _, g := range opts.TestPaths {
		if err := pathglob.Validate(g); err != nil {
			return fmt.Errorf("test path: %w", err)
		}
	}
	for _, r := range opts.Rules {
		if err := r.Validate(); err != nil {
			return fmt.Errorf("root rule %q: %w", r.ID, err)
		}
	}
	return nil
}

// newTables returns a builder with its tables, sized for the indexes.
func newTables(indexes []*fileindex.Index, opts Options) *builder {
	defs, sites := len(indexes), 0
	for _, ix := range indexes {
		defs += len(ix.Definitions)
		sites += len(ix.Imports) + len(ix.Calls) + len(ix.References)
	}
	return &builder{
		opts: opts,
		g: &Graph{
			Nodes: make(map[string]*Node, defs),
			Files: make(map[string]fileindex.FileStatus, len(indexes)),
		},
		byPath:      make(map[string]*fileindex.Index, len(indexes)),
		scopes:      make(map[string]*scope, len(indexes)),
		edges:       newEdgeSet(sites),
		unresolved:  map[UnresolvedSite]bool{},
		truncations: map[Truncation]bool{},
		names:       map[nameKey]resolution{},
		visiting:    map[nameKey]bool{},
		bindings:    map[string]string{},
		classes:     map[string]classRecord{},
		defs:        make(map[string]*fileindex.Definition, defs),
		reading:     map[string]bool{},
		state:       map[langcode.Code]any{},
		units:       map[string]*unit{},
		unitOf:      map[string]*unit{},
		spaces:      map[string]*space{},
		origins:     map[string][]origin{},
		provisions:  map[provisionKey][]provision{},
		linked:      map[string]bool{},
		named:       map[string]bool{},
		namespaces:  map[langcode.Code]*namespaces{},
		loads:       map[string]bool{},
		mros:        map[string][]string{},
		aliases:     map[string][]string{},
		narrowed:    map[Site][]string{},
		structural:  map[siteCall]bool{},
		test:        map[string]bool{},
		rules:       roots.Merge(roots.Default(), opts.Rules),
	}
}

// addIndexes checks each index and adds it by path. The tables of a
// language with no space per unit, as the class table of Java, would join
// the classes of the project and of its dependencies, and a call of the
// project would skip the external key that a symbol record names. So an
// index of dependency code of such a language is a problem, not a file.
func (b *builder) addIndexes(indexes []*fileindex.Index) error {
	skipped := map[modules.Problem]bool{}
	for _, ix := range indexes {
		if ix.Version != fileindex.Version {
			return fmt.Errorf("index of %s has version %q, want %q", ix.Path, ix.Version, fileindex.Version)
		}
		if unit, _ := fileindex.SplitUnit(ix.Path); unit != ix.Unit {
			return fmt.Errorf("index of %s has the unit %q, and its path names the unit %q", ix.Path, ix.Unit, unit)
		}
		if _, dup := b.byPath[ix.Path]; dup {
			return fmt.Errorf("two indexes for the path %s", ix.Path)
		}
		if ix.Unit != "" && !profileOf(ix.Language).dependencyCode {
			skipped[modules.Problem{Path: ix.Unit, Text: fmt.Sprintf("the link step does not link the %s code of %s", ix.Language, ix.Unit)}] = true
			continue
		}
		b.byPath[ix.Path] = ix
		b.paths = append(b.paths, ix.Path)
	}
	b.problems = append(b.problems, sortedKeys(skipped, compareProblems)...)
	return nil
}

// linkAll adds the nodes and the edges of every file, then the surfaces,
// the package edges and the field edges.
func (b *builder) linkAll(ctx context.Context) error {
	for _, p := range b.paths {
		if err := ctx.Err(); err != nil {
			return err
		}
		ix := b.byPath[p]
		if ix == nil {
			b.g.Files[p] = b.opts.Files[p]
			b.addNode(&Node{Key: p, Kind: NodeModule, Path: p, Test: b.test[p]})
			continue
		}
		b.g.Files[p] = b.status(ix)
		b.addDefinitions(ix)
		b.scopes[p] = newScope(ix)
	}
	for _, p := range profiles {
		if p.aliases != nil {
			p.aliases(b)
		}
	}
	// The classes are whole from here, so the members of a group of
	// aliases can be one table.
	b.groupMembers = map[string]map[string]string{}
	b.groupRecords = map[string][]classRecord{}
	b.externalBases = map[string]string{}

	for _, p := range b.paths {
		if err := ctx.Err(); err != nil {
			return err
		}
		ix := b.byPath[p]
		if ix == nil {
			continue
		}
		b.linkImports(ix)
		if link := profileOf(ix.Language).link; link != nil {
			link(b, ix)
		}
		b.linkConstructors(ix)
		b.linkOverrides(ix)
		b.linkArgumentFunctions(ix)
		b.linkCalls(ix)
		b.linkReferences(ix)
		b.linkDecorators(ix)
		b.copyUnresolved(ix)
	}
	if b.opts.Surface {
		b.addSurfaces()
	}
	if err := b.linkPackages(ctx); err != nil {
		return err
	}
	b.checkUnits()
	b.linkFields()
	return nil
}

func (b *builder) status(ix *fileindex.Index) fileindex.FileStatus {
	if len(ix.Truncated) > 0 || slices.ContainsFunc(ix.DynamicSites(), func(f fileindex.Flag) bool {
		return f.Kind == fileindex.FlagParseError
	}) {
		return fileindex.FileTruncated
	}
	if s, ok := b.opts.Files[ix.Path]; ok {
		return s
	}
	return fileindex.FileIndexed
}

// addEdge adds an edge by the rule of the edge set, unless the edge is new
// and the graph is at its edge limit.
func (b *builder) addEdge(e Edge) {
	if !b.edges.holds(e) && b.edges.len() >= b.opts.MaxEdges {
		b.truncations[Truncation{Kind: TruncationEdges}] = true
		return
	}
	b.edges.add(e)
}

func (b *builder) addUnresolved(u UnresolvedSite) {
	b.unresolved[u] = true
}

// siteCall is a call at a site, by the text of its callee.
type siteCall struct {
	site   Site
	callee string
}

func (b *builder) narrow(site Site, callee string) {
	if !slices.Contains(b.narrowed[site], callee) {
		b.narrowed[site] = append(b.narrowed[site], callee)
	}
}

// isNarrowed reports whether the call of the callee text at the site is a
// narrowed call or a part of the receiver of one.
func (b *builder) isNarrowed(site Site, text string) bool {
	return slices.ContainsFunc(b.narrowed[site], func(callee string) bool {
		rest, ok := strings.CutPrefix(callee, text)
		return ok && (rest == "" || strings.IndexFunc(rest[:1], func(r rune) bool { return r == '.' || r == ':' || r == '-' }) == 0)
	})
}

func (b *builder) copyUnresolved(ix *fileindex.Index) {
	for _, u := range ix.Unresolved {
		// The link step resolves a call on super through the bases of the
		// class, and the call on a narrowed receiver through the class. It
		// records a call that it cannot resolve.
		at := Site{Path: ix.Path, Line: u.Line, Column: u.Column}
		if u.Receiver == superKeyword && profileOf(ix.Language).superCalls || b.isNarrowed(at, u.Text) {
			continue
		}
		site := UnresolvedSite{
			Caller:   b.runner(u.Caller),
			Site:     at,
			Text:     u.Text,
			Receiver: u.Receiver,
			Method:   u.Method,
			Reason:   u.Reason,
		}
		if u.Reason == fileindex.UnresolvedDynamicImport {
			site.ImportPrefix = b.importPrefix(ix, u.Line)
		}
		b.addUnresolved(site)
	}
}

func (b *builder) finish() *Graph {
	g := b.g
	g.Edges = b.edges.sorted()
	g.Unresolved = sortedKeys(b.unresolved, compareUnresolved)
	g.Truncated = sortedKeys(b.truncations, compareTruncations)
	slices.SortFunc(g.Roots, func(a, b Root) int {
		return cmp.Or(cmp.Compare(a.Key, b.Key), cmp.Compare(a.Kind, b.Kind), cmp.Compare(a.Rule, b.Rule))
	})
	if g.Roots == nil {
		g.Roots = []Root{}
	}
	g.Problems = b.problems
	g.Stats.Version = Version
	g.Stats.Files, g.Stats.Nodes, g.Stats.Edges = len(g.Files), len(g.Nodes), len(g.Edges)
	g.Stats.Unresolved, g.Stats.Roots = len(g.Unresolved), len(g.Roots)
	return g
}

func sortedKeys[T comparable](set map[T]bool, compare func(a, b T) int) []T {
	out := make([]T, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	slices.SortFunc(out, compare)
	return out
}

func compareSites(a, b Site) int {
	return cmp.Or(cmp.Compare(a.Path, b.Path), cmp.Compare(a.Line, b.Line), cmp.Compare(a.Column, b.Column))
}

func compareUnresolved(a, b UnresolvedSite) int {
	return cmp.Or(
		compareSites(a.Site, b.Site),
		cmp.Compare(a.Caller, b.Caller),
		cmp.Compare(a.Text, b.Text),
		cmp.Compare(a.Receiver, b.Receiver),
		cmp.Compare(a.Method, b.Method),
		cmp.Compare(a.Reason, b.Reason),
		cmp.Compare(a.ImportPrefix, b.ImportPrefix),
		cmp.Compare(a.Unit, b.Unit),
	)
}

func compareProblems(a, b modules.Problem) int {
	return cmp.Or(cmp.Compare(a.Path, b.Path), cmp.Compare(a.Text, b.Text))
}

func compareTruncations(a, b Truncation) int {
	return cmp.Or(cmp.Compare(a.Kind, b.Kind), cmp.Compare(a.Key, b.Key), compareSites(a.Site, b.Site))
}
