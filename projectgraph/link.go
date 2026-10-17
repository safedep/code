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
	// TestPaths marks the test files, next to Index.Test. A glob with no "/"
	// matches one part of the path, as tests or test_*.py.
	TestPaths []string
	// Rules go over the embedded root rules by id.
	Rules []roots.Rule

	maxEdges      int
	maxExportHops int
}

// Build links the indexes of one project into a graph. Every index must have
// the Version of this build, and no two indexes may share a path. The graph
// does not depend on the order of the indexes.
func Build(ctx context.Context, indexes []*fileindex.Index, opts Options) (*Graph, error) {
	start := time.Now()
	b, err := newBuilder(indexes, opts)
	if err != nil {
		return nil, err
	}
	if err := b.link(ctx); err != nil {
		return nil, err
	}
	b.markRoots()
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
	files  *modules.Files
	python *modules.Python
	js     *modules.JavaScript
	scopes map[string]*scope
	test   map[string]bool
	rules  []roots.Rule

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

	golang *modules.Golang
	units  map[string]*unit
	unitOf map[string]*unit
	// goPackages holds the package that an import of each directory names.
	goPackages map[string]*unit
	// javaPackages holds the unit of each Java package by name.
	javaPackages map[string]*unit

	mros     map[string][]string
	children map[string][]string
	// narrowed holds the call sites whose receiver a class narrowed. Such a
	// site gets no field or name edge, and the link step owns its record.
	narrowed map[Site]bool
	// goMethods holds the Go methods of the project by name.
	goMethods map[string][]string
	// implemented holds whether a Go type implements an interface.
	implemented map[[2]string]bool
	// loads holds whether the module top level of each file runs code.
	loads map[string]bool
}

type classRecord struct {
	ix    *fileindex.Index
	class fileindex.Class
}

func newBuilder(indexes []*fileindex.Index, opts Options) (*builder, error) {
	opts.maxEdges = cmp.Or(opts.maxEdges, defaultMaxEdges)
	opts.maxExportHops = cmp.Or(opts.maxExportHops, defaultMaxExportHops)
	for _, g := range opts.TestPaths {
		if err := pathglob.Validate(g); err != nil {
			return nil, fmt.Errorf("test path: %w", err)
		}
	}
	for _, r := range opts.Rules {
		if err := r.Validate(); err != nil {
			return nil, fmt.Errorf("root rule %q: %w", r.ID, err)
		}
	}

	defs, sites := len(indexes), 0
	for _, ix := range indexes {
		defs += len(ix.Definitions)
		sites += len(ix.Imports) + len(ix.Calls) + len(ix.References)
	}
	b := &builder{
		opts: opts,
		g: &Graph{
			Nodes: make(map[string]*Node, defs),
			Files: make(map[string]fileindex.FileStatus, len(indexes)),
		},
		byPath:       make(map[string]*fileindex.Index, len(indexes)),
		scopes:       make(map[string]*scope, len(indexes)),
		edges:        newEdgeSet(sites),
		unresolved:   map[UnresolvedSite]bool{},
		truncations:  map[Truncation]bool{},
		names:        map[nameKey]resolution{},
		visiting:     map[nameKey]bool{},
		bindings:     map[string]string{},
		classes:      map[string]classRecord{},
		defs:         make(map[string]*fileindex.Definition, defs),
		reading:      map[string]bool{},
		units:        map[string]*unit{},
		unitOf:       map[string]*unit{},
		goPackages:   map[string]*unit{},
		javaPackages: map[string]*unit{},
		loads:        map[string]bool{},
		mros:         map[string][]string{},
		narrowed:     map[Site]bool{},
		implemented:  map[[2]string]bool{},
		test:         map[string]bool{},
		rules:        roots.Merge(roots.Default(), opts.Rules),
	}
	for _, ix := range indexes {
		if ix.Version != fileindex.Version {
			return nil, fmt.Errorf("index of %s has version %q, want %q", ix.Path, ix.Version, fileindex.Version)
		}
		if _, dup := b.byPath[ix.Path]; dup {
			return nil, fmt.Errorf("two indexes for the path %s", ix.Path)
		}
		b.byPath[ix.Path] = ix
		b.paths = append(b.paths, ix.Path)
	}
	for p, status := range opts.Files {
		if _, ok := b.byPath[p]; !ok && !status.Complete() {
			b.paths = append(b.paths, p)
		}
	}
	slices.Sort(b.paths)
	for _, p := range b.paths {
		ix := b.byPath[p]
		b.test[p] = ix != nil && ix.Test || pathglob.MatchAny(opts.TestPaths, p)
	}

	b.files = modules.NewFiles(b.paths)
	b.python = modules.NewPython(b.files, opts.Root, opts.SysPath)
	b.js = modules.NewJavaScript(b.files, opts.Root)
	b.problems = append(b.problems, b.python.Problems()...)
	b.problems = append(b.problems, b.js.Problems()...)
	b.addGoUnits()
	b.addJavaUnits()
	return b, nil
}

func (b *builder) link(ctx context.Context) error {
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

	for _, p := range b.paths {
		if err := ctx.Err(); err != nil {
			return err
		}
		ix := b.byPath[p]
		if ix == nil {
			continue
		}
		b.linkImports(ix)
		b.linkInits(ix)
		b.linkClassInits(ix)
		b.linkConstructors(ix)
		b.linkArgumentFunctions(ix)
		b.linkCalls(ix)
		b.linkReferences(ix)
		b.linkDecorators(ix)
		b.copyUnresolved(ix)
	}
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

func (b *builder) addDefinitions(ix *fileindex.Index) {
	defFlags := map[fileindex.Flag]bool{}
	for _, def := range ix.Definitions {
		for _, f := range def.Flags {
			defFlags[f] = true
		}
	}
	module := &Node{Key: ix.Path, Kind: NodeModule, Path: ix.Path, Line: 1, Exported: true, Test: b.test[ix.Path]}
	for _, f := range ix.Flags {
		if !defFlags[f] {
			module.Flags = append(module.Flags, f)
		}
	}
	b.addNode(module)

	for i := range ix.Definitions {
		def := &ix.Definitions[i]
		b.defs[def.Key] = def
		if def.Key == ix.Path {
			module.Line = cmp.Or(def.StartLine, 1)
			module.Flags = append(module.Flags, def.Flags...)
			continue
		}
		b.addNode(&Node{
			Key:      def.Key,
			Kind:     nodeKind(def.Kind),
			Path:     ix.Path,
			Line:     def.StartLine,
			Exported: def.Exported,
			Test:     b.test[ix.Path],
			Flags:    slices.Clone(def.Flags),
		})
	}
	for _, def := range ix.Definitions {
		if r := b.runner(def.Key); r != def.Key {
			n, runner := b.g.Nodes[def.Key], b.g.Nodes[r]
			runner.Flags, n.Flags = append(runner.Flags, n.Flags...), nil
		}
	}
	for _, def := range ix.Definitions {
		n := b.g.Nodes[def.Key]
		slices.SortFunc(n.Flags, fileindex.CompareFlags)
		n.Flags = slices.Compact(n.Flags)
	}
	slices.SortFunc(module.Flags, fileindex.CompareFlags)
	module.Flags = slices.Compact(module.Flags)

	for _, c := range ix.Classes {
		b.classes[c.Key] = classRecord{ix: ix, class: c}
	}
}

// runner returns the key of the definition that runs the code of the key.
// A Python class body runs when the scope that defines the class runs, so
// the calls and flags of the body belong to that scope. A call of the class
// runs only the constructor.
func (b *builder) runner(key string) string {
	for b.kindOf(key) == NodeClass && b.byPath[fileindex.Unit(key)].Language == langcode.Python {
		segs := fileindex.Segments(key)
		key = fileindex.Join(fileindex.Unit(key), segs[:len(segs)-1]...)
	}
	return key
}

func nodeKind(k fileindex.DefKind) NodeKind {
	switch k {
	case fileindex.DefFunction:
		return NodeFunction
	case fileindex.DefMethod:
		return NodeMethod
	case fileindex.DefConstructor:
		return NodeConstructor
	case fileindex.DefClass:
		return NodeClass
	case fileindex.DefVariable:
		return NodeVariable
	case fileindex.DefInterface:
		return NodeInterface
	default:
		return NodeModule
	}
}

func (b *builder) addNode(n *Node) {
	if _, ok := b.g.Nodes[n.Key]; !ok {
		b.g.Nodes[n.Key] = n
	}
}

func (b *builder) external(key string) string {
	b.addNode(&Node{Key: key, Kind: NodeExternal})
	return key
}

// addEdge adds an edge unless the caller already has an edge to the callee
// at the site with a confidence at least as high. A higher one replaces it,
// so a path through the site shows the best kind.
func (b *builder) addEdge(e Edge) {
	slot := b.edges.slot(e)
	old, ok := b.edges.kind(slot)
	switch {
	case ok && old.Confidence().Rank() >= e.Kind.Confidence().Rank():
		return
	case !ok && b.edges.len() >= b.opts.maxEdges:
		b.truncations[Truncation{Kind: TruncationEdges}] = true
		return
	}
	b.edges.put(slot, e)
}

func (b *builder) addUnresolved(u UnresolvedSite) {
	b.unresolved[u] = true
}

// linkImports adds an import edge from the definition that holds each
// import, or the module of each re-export, to the module top level that it
// runs.
func (b *builder) linkImports(ix *fileindex.Index) {
	sc := b.scopes[ix.Path]
	for _, imp := range ix.Imports {
		if imp.Kind != fileindex.ImportStatic && imp.Kind != fileindex.ImportDynamicString {
			continue
		}
		from := b.runner(sc.enclosing(imp.Line))
		site := Site{Path: ix.Path, Line: imp.Line}
		mod, via, ok := b.resolveModule(ix, imp.Module)
		if !ok {
			// The record has no method, so it makes every target uncertain
			// where the search reaches the import.
			if b.mayBeFirstParty(ix.Language, imp.Module) {
				b.addUnresolved(UnresolvedSite{Caller: from, Site: site, Text: imp.Module, Reason: fileindex.UnresolvedNoDefinition})
			}
			continue
		}
		resolver := resolverName(ix.Language, "import", via)
		for _, to := range b.moduleFiles(mod) {
			b.addEdge(Edge{From: from, To: to, Site: site, Kind: EdgeImport, Resolver: resolver})
		}
		if ix.Language != langcode.Python || imp.Item == "" || b.bindsName(mod, imp.Item) {
			continue
		}
		if sub, ok := b.python.Submodule(mod, imp.Item); ok && b.isModule(sub) {
			b.addEdge(Edge{From: from, To: sub, Site: site, Kind: EdgeImport, Resolver: resolverName(ix.Language, "import", "submodule")})
		}
	}
	for _, ex := range ix.Exports {
		if ex.From == "" {
			continue
		}
		if mod, via, ok := b.resolveModule(ix, ex.From); ok && b.isModule(mod) {
			site := Site{Path: ix.Path, Line: ex.Line}
			b.addEdge(Edge{From: ix.Path, To: mod, Site: site, Kind: EdgeImport, Resolver: resolverName(ix.Language, "import", via)})
		}
	}
}

// moduleFiles returns the module top levels that an import of the module
// runs: the module, or each file of a unit that runs code when the program
// loads it, as a Go file with an init function or a package variable that
// a call sets. A Go package can have hundreds of files, and an edge to each
// file with no such code adds nothing to a path.
func (b *builder) moduleFiles(mod string) []string {
	if u := b.units[mod]; u != nil {
		return slices.DeleteFunc(slices.Clone(u.files), func(p string) bool { return !b.runsAtLoad(b.byPath[p]) })
	}
	if b.isModule(mod) {
		return []string{mod}
	}
	return nil
}

// runsAtLoad reports whether the module top level of a file runs code: an
// init function, a call, a reference, an unresolved site, a flag, or an
// import of a package of the project that runs code at load.
func (b *builder) runsAtLoad(ix *fileindex.Index) bool {
	if runs, ok := b.loads[ix.Path]; ok {
		return runs
	}
	// The mark stops a cycle of imports.
	b.loads[ix.Path] = false
	atTop := func(caller string) bool { return caller == ix.Path }
	runs := b.kindOf(fileindex.Join(ix.Path, "init")) == NodeFunction ||
		slices.ContainsFunc(ix.Calls, func(c fileindex.Call) bool { return atTop(c.Caller) && c.Kind != fileindex.CallImport }) ||
		slices.ContainsFunc(ix.References, func(r fileindex.Reference) bool { return atTop(r.Caller) }) ||
		slices.ContainsFunc(ix.Unresolved, func(u fileindex.Unresolved) bool { return atTop(u.Caller) }) ||
		len(b.g.Nodes[ix.Path].Flags) > 0 ||
		slices.ContainsFunc(ix.Imports, func(imp fileindex.Import) bool {
			mod, _, ok := b.resolveModule(ix, imp.Module)
			return ok && len(b.moduleFiles(mod)) > 0
		})
	b.loads[ix.Path] = runs
	return runs
}

// linkInits adds an edge from the module top level of a Go file to each
// init function of the file, because the program runs it when it loads
// the package.
func (b *builder) linkInits(ix *fileindex.Index) {
	if ix.Language != langcode.Go {
		return
	}
	key := fileindex.Join(ix.Path, "init")
	if b.kindOf(key) != NodeFunction {
		return
	}
	n := b.g.Nodes[key]
	b.addEdge(Edge{From: ix.Path, To: key, Site: Site{Path: ix.Path, Line: n.Line}, Kind: EdgeDirect, Resolver: resolverName(ix.Language, "init")})
}

// linkClassInits adds an edge from the class node and each method of a
// Java class to the static initializer <clinit> of the class, which the
// index defines when the class runs code at load. The JVM runs that code
// before the first instance, the first static call and the first access to
// a static field. The edge goes to <clinit> and not to the class node,
// because the class node leads to the constructor, which a static call
// does not run.
func (b *builder) linkClassInits(ix *fileindex.Index) {
	if ix.Language != langcode.Java {
		return
	}
	for _, c := range ix.Classes {
		init := fileindex.Join(c.Key, javaClassInit)
		if !b.isMethod(init) {
			continue
		}
		for _, from := range append([]string{c.Key}, c.Methods...) {
			if from == init || b.g.Nodes[from] == nil {
				continue
			}
			n := b.g.Nodes[from]
			b.addEdge(Edge{From: from, To: init, Site: Site{Path: n.Path, Line: n.Line}, Kind: EdgeDirect, Resolver: resolverName(ix.Language, "class-init")})
		}
	}
}

// javaClassInit names the static initializer of a Java class.
const javaClassInit = "<clinit>"

// linkConstructors adds an edge from each class to the constructor that
// runs when a call makes an instance. A call to a class reaches the class
// node, through a re-export or not, so the constructor hangs off the class.
func (b *builder) linkConstructors(ix *fileindex.Index) {
	for _, c := range ix.Classes {
		ctor, ok := b.constructor(c.Key)
		if !ok {
			continue
		}
		n := b.g.Nodes[ctor]
		b.addEdge(Edge{
			From:     c.Key,
			To:       ctor,
			Site:     Site{Path: n.Path, Line: n.Line},
			Kind:     EdgeDirect,
			Resolver: resolverName(ix.Language, "constructor"),
		})
	}
}

// linkArgumentFunctions adds an edge from the scope that defines each
// function literal of a call argument to the function, as from the module
// to the handler of app.get('/x', (req, res) => {}). The callee gets the
// function and can call it at any time. Before the index named such a
// function, its body ran with its definer, so the edge keeps that with a
// direct edge. A root rule can mark the function itself.
func (b *builder) linkArgumentFunctions(ix *fileindex.Index) {
	for _, def := range ix.Definitions {
		if def.Kind != fileindex.DefFunction || !fileindex.IsAnonymous(def.Name) {
			continue
		}
		site := Site{Path: ix.Path, Line: def.StartLine}
		b.addEdge(Edge{From: b.definer(def.Key), To: def.Key, Site: site, Kind: EdgeDirect, Resolver: resolverName(ix.Language, "argument-function")})
	}
}

// linkDecorators adds an edge from the scope that defines each decorated
// definition to each decorator that the file scope resolves, because the
// decorator runs when the scope defines the definition. The index writes a
// decorator with no arguments, so @wraps(fn) is also a call of the scope,
// and it gets no second edge.
//
// A first-party decorator can return a nested function, as wrapper in a
// logged decorator, and a call of the decorated name then runs that
// function. So each nested function of the decorator gets a reference edge
// from the decorated definition. An external decorator gets no such edge.
func (b *builder) linkDecorators(ix *fileindex.Index) {
	// A Java annotation is data that a framework reads, and it runs no
	// code where it stands.
	if ix.Language == langcode.Java {
		return
	}
	sc := b.scopes[ix.Path]
	for _, def := range ix.Definitions {
		if len(def.Decorators) == 0 {
			continue
		}
		from := b.definer(def.Key)
		site := Site{Path: ix.Path, Line: def.StartLine}
		for _, d := range def.Decorators {
			key, ok := b.resolveScopeName(sc, strings.TrimPrefix(d, "@"))
			if !ok {
				continue
			}
			for _, nested := range b.nestedFunctions(key) {
				b.addEdge(Edge{From: def.Key, To: nested, Site: site, Kind: EdgeReference, Resolver: resolverName(ix.Language, "decorator-closure")})
			}
			if b.edges.joins(from, key) {
				continue
			}
			if b.g.Nodes[key] == nil {
				b.external(key)
			}
			b.addEdge(Edge{From: from, To: key, Site: site, Kind: EdgeDirect, Resolver: resolverName(ix.Language, "decorator")})
		}
	}
}

// nestedFunctions returns the keys of the functions that a first-party
// function defines, at any depth.
func (b *builder) nestedFunctions(key string) []string {
	if b.kindOf(key) != NodeFunction {
		return nil
	}
	var nested []string
	for _, def := range b.byPath[fileindex.Unit(key)].Definitions {
		if def.Kind == fileindex.DefFunction && def.Key != key && fileindex.HasKeyPrefix(def.Key, key) {
			nested = append(nested, def.Key)
		}
	}
	return nested
}

// definer returns the key of the definition that runs the definition of
// the key. A class body runs with the scope that defines the class.
func (b *builder) definer(key string) string {
	for {
		segs := fileindex.Segments(key)
		if len(segs) == 0 {
			return key
		}
		key = fileindex.Join(fileindex.Unit(key), segs[:len(segs)-1]...)
		if b.kindOf(key) != NodeClass {
			return b.runner(key)
		}
	}
}

func (b *builder) copyUnresolved(ix *fileindex.Index) {
	for _, u := range ix.Unresolved {
		// The link step resolves a call on super through the bases of the
		// class, and the call on a narrowed receiver through the class. It
		// records a call that it cannot resolve.
		at := Site{Path: ix.Path, Line: u.Line, Column: u.Column}
		if u.Receiver == superKeyword && ix.Language.IsECMAScript() || b.narrowed[at] {
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

// importPrefix returns the key form of the static prefix of a computed
// import on the line. The Module of a computed import holds that prefix.
func (b *builder) importPrefix(ix *fileindex.Index, line int) string {
	for _, imp := range ix.Imports {
		if imp.Line != line || imp.Kind != fileindex.ImportDynamicComputed || imp.Module == "" {
			continue
		}
		spec := imp.Module
		if strings.HasPrefix(spec, ".") {
			trailing := strings.HasSuffix(spec, "/") || ix.Language == langcode.Python && strings.HasSuffix(spec, ".")
			if ix.Language == langcode.Python {
				rest := strings.TrimLeft(spec, ".")
				spec = "./" + strings.Repeat("../", len(spec)-len(rest)-1) + strings.ReplaceAll(rest, ".", "/")
			}
			p, ok := modules.Join(modules.Dir(ix.Path), spec)
			if !ok {
				return ""
			}
			if trailing {
				p += "/"
			}
			return p
		}
		return fileindex.Namespace(ix.Language, spec)
	}
	return ""
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
	g.Stats = Stats{
		Version:      Version,
		Files:        len(g.Files),
		Nodes:        len(g.Nodes),
		Edges:        len(g.Edges),
		Unresolved:   len(g.Unresolved),
		Roots:        len(g.Roots),
		RootFallback: g.Stats.RootFallback,
	}
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
	)
}

func compareTruncations(a, b Truncation) int {
	return cmp.Or(cmp.Compare(a.Kind, b.Kind), cmp.Compare(a.Key, b.Key), compareSites(a.Site, b.Site))
}

func resolverName(lang langcode.Code, parts ...string) string {
	return strings.Join(append([]string{string(lang)}, parts...), ".")
}
