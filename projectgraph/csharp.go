package projectgraph

import (
	"cmp"
	"slices"
	"strings"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/projectgraph/modules"
)

// csharpProfile reads the namespaces of C# as Java reads its packages: the
// files of one namespace are one unit, a using names a namespace, and a
// class names the classes of its namespace and of the namespaces that hold
// it with no using.
var csharpProfile = profile{
	code:              langcode.CSharp,
	resolver:          func(b *builder, _ *space) modules.Resolver { return namespaceResolver{b, langcode.CSharp} },
	units:             func(b *builder) { b.addNamespaceUnits(langcode.CSharp, ".", langcode.CSharp) },
	aliases:           (*builder).joinPartialClasses,
	link:              (*builder).linkCSharp,
	qualified:         func(b *builder, name string) (string, bool) { return b.resolveQualified(langcode.CSharp, name) },
	scopeFallback:     (*builder).csharpScopeFallback,
	instance:          []string{"this"},
	thisCalls:         true,
	superCalls:        true,
	implicitBases:     true,
	implicitFields:    true,
	nestedTypes:       true,
	annotationsData:   true,
	scopeWildcards:    true,
	exactWildcards:    true,
	definitionExports: true,
	implicitMethods:   csharpImplicit,
}

// joinPartialClasses makes the partial classes of one name in one
// namespace one class, as C# does.
func (b *builder) joinPartialClasses() {
	t := b.namespaces[langcode.CSharp]
	if t == nil {
		return
	}
	for _, u := range t.units {
		var groups [][]string
		for _, keys := range u.names {
			var classes []string
			for _, k := range keys {
				if b.isType(k) {
					classes = append(classes, k)
				}
			}
			groups = append(groups, classes)
		}
		b.joinAliases(groups)
	}
}

// linkClassBodies adds the edges to the code of a class body that runs with
// no call: the static initializer <clinit>, which runs before the first
// instance and the first static call, as in Java, the class node, which
// holds the field and property initializers of an instance, and the
// methods that the language calls with no call of the name. A method that
// is not static runs on an instance, so it has an edge to its class.
func (b *builder) linkClassBodies(ix *fileindex.Index) {
	b.linkClassInits(ix)
	b.linkImplicitMethods(ix)
	for _, c := range ix.Classes {
		for _, m := range c.Methods {
			def := b.defs[m]
			if def == nil || def.Static || !b.isMethod(m) {
				continue
			}
			n := b.g.Nodes[m]
			b.addEdge(Edge{From: m, To: c.Key, Site: Site{Path: n.Path, Line: n.Line}, Kind: EdgeDirect, Resolver: resolverName(ix.Language, "instance")})
		}
	}
}

// csharpImplicit name the methods that C# code calls with no call of the
// name: Dispose at the end of a using, GetEnumerator and MoveNext in a
// foreach, Deconstruct in a deconstruction, GetAwaiter in an await, and the
// methods of object that the runtime calls.
var csharpImplicit = map[string]bool{
	"Dispose": true, "DisposeAsync": true, "GetEnumerator": true, "GetAsyncEnumerator": true,
	"MoveNext": true, "MoveNextAsync": true, "Deconstruct": true, "GetAwaiter": true,
	"ToString": true, "Equals": true, "GetHashCode": true, "CompareTo": true,
}

// linkCSharp adds the edges of the class bodies and of the extension
// methods. A using of a namespace makes its extension methods visible, so
// a call can run any extension method of its name.
func (b *builder) linkCSharp(ix *fileindex.Index) {
	b.linkClassBodies(ix)
	b.linkExtensions(ix, b.extensionsNamed)
}

// walkNamespace resolves the key of a name of a namespace table.
func (b *builder) walkNamespace(key string) resolution {
	r := b.walk(key, nil)
	r.rule = cmp.Or(r.rule, "namespace")
	return r
}

// csharpRoots are the first names of the namespaces of the platform, which
// a qualified name as System.IO.File.Delete starts with.
var csharpRoots = []string{"System", "Microsoft"}

// csharpScopeFallback resolves a name that no scope of the file binds: a
// name under a namespace that holds the namespace of the file, as
// Data.Store in App.Web for App.Data.Store, a name under the namespace of
// the file, a qualified name, or a class of a using of a dependency. A
// qualified name of a dependency starts with the first name of a namespace
// of the platform or of a using. A using of a dependency can name any of
// its classes, so each one gives a wildcard edge. A using names types and
// no methods, so a bare name, as a method of a base class, gets none.
func (b *builder) csharpScopeFallback(sc *scope, _, first string, rest []string, target string) []resolution {
	dotted := strings.ReplaceAll(target, fileindex.Separator, ".")
	for ns := sc.ix.Package; ns != ""; {
		ns = ns[:max(strings.LastIndexByte(ns, '.'), 0)]
		if key, ok := b.resolveQualified(langcode.CSharp, strings.TrimPrefix(ns+"."+dotted, ".")); ok {
			return []resolution{b.walkNamespace(key)}
		}
	}
	if len(rest) == 0 {
		return nil
	}
	if key, ok := b.resolveQualified(langcode.CSharp, sc.ix.Package+"."+dotted); ok {
		return []resolution{b.walkNamespace(key)}
	}
	roots := slices.Clone(csharpRoots)
	var out []resolution
	for _, imp := range sc.wildcards {
		if _, _, ok := b.resolveModule(sc.ix, imp.Module); ok {
			continue
		}
		roots = append(roots, firstDotted(imp.Module))
		key := fileindex.Join(fileindex.Namespace(sc.ix.Language, imp.Module), first)
		out = append(out, resolution{key: b.external(fileindex.Join(key, rest...)), kind: EdgeWildcard, rule: "wildcard-external"})
	}
	if slices.Contains(roots, first) {
		return []resolution{{key: b.external(target), kind: EdgeDirect, rule: "external"}}
	}
	return out
}
