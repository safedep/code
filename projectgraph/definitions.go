package projectgraph

import (
	"cmp"
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
