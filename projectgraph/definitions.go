package projectgraph

import (
	"cmp"
	"maps"
	"slices"
	"strings"

	"github.com/safedep/code/fileindex"
)

func (b *builder) addDefinitions(ix *fileindex.Index) {
	defFlags := map[fileindex.Flag]bool{}
	for _, def := range ix.Definitions {
		for _, f := range def.Flags {
			defFlags[f] = true
		}
	}
	module := &Node{Key: ix.Path, Kind: NodeModule, Path: ix.Path, Line: 1, Exported: true, Test: b.test[ix.Path], Language: ix.Language}
	for _, f := range ix.Flags {
		if !defFlags[f] {
			module.Flags = append(module.Flags, f)
		}
	}
	b.addNode(module)

	// No edge goes to a definition of data, so it has no node. A walk
	// finds it by its definition.
	for i := range ix.Definitions {
		b.defs[ix.Definitions[i].Key] = &ix.Definitions[i]
	}
	for def := range ix.CodeDefinitions() {
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
	for def := range ix.CodeDefinitions() {
		if r := b.runner(def.Key); r != def.Key {
			n, runner := b.g.Nodes[def.Key], b.g.Nodes[r]
			runner.Flags, n.Flags = append(runner.Flags, n.Flags...), nil
		}
	}
	for def := range ix.CodeDefinitions() {
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
	for b.kindOf(key) == NodeClass && b.profileAt(key).classBodyInDefiner {
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
	case fileindex.DefAlias:
		return NodeExport
	default:
		return NodeModule
	}
}

// holdsData reports whether the key names a definition that holds data and
// no code, as a field or a constant of Go.
func (b *builder) holdsData(key string) bool {
	def := b.defs[key]
	return def != nil && def.HoldsData()
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

// linkConstructors adds an edge from each class to the constructor that
// runs when a call makes an instance. A call to a class reaches the class
// node, through a re-export or not, so the constructor hangs off the class.
func (b *builder) linkConstructors(ix *fileindex.Index) {
	for _, c := range ix.Classes {
		ctor, ok := b.constructor(c.Key)
		if !ok {
			continue
		}
		b.link(ix.Language, c.Key, ctor, b.siteOf(ctor), EdgeDirect, ruleConstructor)
	}
}

// linkInstances adds a reference edge from each edge that can make an
// instance of a class to the method of each instance method name that the
// instance runs, from the class or from a base in its MRO. A call makes an
// instance, and so can the code that gets a reference to the class, as
// functools.partial(C) or map(C, xs). The code that makes the instance
// holds it, so a search reaches the method as it reaches a stored function
// value. A reference that only tests a value against the class, as in
// isinstance(x, C), makes no instance and gets no edge. An edge reaches the
// class through export hops and the package edge of a unit.
func (b *builder) linkInstances() {
	str := b.edges.symbols.strings
	hops := map[string][]string{}
	for slot, v := range b.edges.slots {
		if k := EdgeKind(str[v.kind]); k == EdgeExport || k == EdgePackage {
			hops[str[slot.from]] = append(hops[str[slot.from]], str[slot.to])
		}
	}
	memo := map[string][]string{}
	methodsOf := func(key string) []string {
		m, ok := memo[key]
		if !ok {
			m = b.instanceMethods(key, hops)
			memo[key] = m
		}
		return m
	}
	var out []Edge
	add := func(from string, site Site, methods []string) {
		for _, key := range methods {
			out = append(out, newEdge(from, key, site, EdgeReference, resolverName(b.profileAt(key).code, ruleImplicit)))
		}
	}
	for slot, v := range b.edges.slots {
		kind := EdgeKind(str[v.kind])
		if kind == EdgeImport || kind == EdgeExport || kind == EdgePackage || b.checks[slot] {
			continue
		}
		add(str[slot.from], Site{Path: str[slot.path], Line: int(slot.line), Column: int(slot.column)}, methodsOf(str[slot.to]))
	}
	for _, c := range b.classCalls() {
		for _, class := range append([]string{c.class}, b.subclasses(c.class)...) {
			add(c.from, c.site, methodsOf(class))
		}
	}
	slices.SortFunc(out, func(x, y Edge) int {
		return cmp.Or(cmp.Compare(x.From, y.From), cmp.Compare(x.Site.Path, y.Site.Path), cmp.Compare(x.Site.Line, y.Site.Line),
			cmp.Compare(x.Site.Column, y.Site.Column), cmp.Compare(x.To, y.To))
	})
	for _, e := range out {
		b.addEdge(e)
	}
}

// instanceMethods returns the instance methods of each class that a key
// names through its hops. A call of a class also runs __call__ of its
// metaclass, as M in class A(metaclass=M).
func (b *builder) instanceMethods(key string, hops map[string][]string) []string {
	var out []string
	seen := map[string]bool{}
	next := []string{key}
	for depth := 0; len(next) > 0 && depth <= b.opts.MaxExportHops; depth++ {
		var later []string
		for _, k := range next {
			if seen[k] {
				continue
			}
			seen[k] = true
			later = append(later, hops[k]...)
			p := b.profileAt(k)
			if b.kindOf(k) != NodeClass || len(p.instanceMethods) == 0 {
				continue
			}
			for _, name := range slices.Sorted(maps.Keys(p.instanceMethods)) {
				if method, ok := b.ownMethod(k, name); ok && b.isMethod(method) {
					out = append(out, method)
				}
			}
			if meta, ok := b.metaclass(k); ok {
				if method, ok := b.ownMethod(meta, p.metaclassCall); ok && b.isMethod(method) {
					out = append(out, method)
				}
			}
		}
		next = later
	}
	return out
}

// metaclass returns the class that a class of the MRO of a class names as
// its metaclass.
func (b *builder) metaclass(class string) (string, bool) {
	for _, c := range b.mro(class) {
		for _, rec := range b.records(c) {
			if rec.class.Metaclass == "" {
				continue
			}
			key, ok := b.resolveNameIn(b.scopes[rec.ix.Path], fileindex.Parent(rec.class.Key), rec.class.Metaclass)
			return key, ok && b.kindOf(key) == NodeClass
		}
	}
	return "", false
}

// classCall is a call of the class of a method from the method itself, as
// cls() in a classmethod of Python.
type classCall struct {
	from, class string
	site        Site
}

// classCalls returns each call that makes an instance of the class of the
// method that holds it, or of a subclass: a call of the first parameter of
// a classmethod, also from a function that the method defines, and a call
// of the class of the instance, as self.__class__() or type(self)().
func (b *builder) classCalls() []classCall {
	var out []classCall
	for _, p := range b.paths {
		ix := b.byPath[p]
		if ix == nil || len(profileOf(ix.Language).instanceClass) == 0 {
			continue
		}
		for _, call := range ix.Calls {
			if class, ok := b.callsClass(ix, call); ok {
				out = append(out, classCall{from: b.runner(call.Caller), class: class, site: Site{Path: ix.Path, Line: call.Line, Column: call.Column}})
			}
		}
	}
	return out
}

// callsClass returns the class that a call makes an instance of, by the
// method that holds the call.
func (b *builder) callsClass(ix *fileindex.Index, call fileindex.Call) (string, bool) {
	instance := slices.Contains(profileOf(ix.Language).instanceClass, call.Callee)
	if caller := b.defs[call.Caller]; !instance && (caller == nil || caller.ClassParam == "" || call.Callee != caller.ClassParam) {
		return "", false
	}
	for key := call.Caller; key != "" && key != ix.Path; key = fileindex.Parent(key) {
		if def := b.defs[key]; def != nil && def.Parent != "" && b.kindOf(def.Parent) == NodeClass {
			return def.Parent, true
		}
	}
	return "", false
}

// linkOverrides adds a reference edge from each class with a base of a
// dependency to the method that an instance of the class runs for each
// method name of the first-party classes of its MRO, as run of a subclass
// of threading.Thread or do_GET of a subclass of BaseHTTPRequestHandler.
// The code of the dependency calls such a method on an instance of the
// class, and the graph has no edge for that call. The method can come from
// a first-party base, as the run of Base for class Worker(Base) with
// class Base(Thread). A constructor runs with the class and gets its own
// edge.
func (b *builder) linkOverrides(ix *fileindex.Index) {
	for _, c := range ix.Classes {
		if _, ok := b.externalBase(c.Key); !ok {
			continue
		}
		seen := map[string]bool{}
		for _, k := range b.mro(c.Key) {
			rec, ok := b.classes[k]
			if !ok {
				continue
			}
			for _, m := range rec.class.Methods {
				name := fileindex.Last(m)
				if seen[name] || b.kindOf(m) != NodeMethod || name == javaClassInit {
					continue
				}
				seen[name] = true
				key, ok := b.ownMethod(c.Key, name)
				if !ok || b.kindOf(key) != NodeMethod {
					continue
				}
				b.link(ix.Language, c.Key, key, b.siteOf(key), EdgeReference, ruleOverride)
			}
		}
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
		b.link(ix.Language, b.definer(def.Key), def.Key, site, EdgeDirect, ruleArgumentFunction)
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
	if profileOf(ix.Language).annotationsData {
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
				b.link(ix.Language, def.Key, nested, site, EdgeReference, ruleDecoratorClosure)
			}
			if b.edges.joins(from, key) {
				continue
			}
			if b.g.Nodes[key] == nil {
				b.external(key)
			}
			b.link(ix.Language, from, key, site, EdgeDirect, ruleDecorator)
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
