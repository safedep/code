package projectgraph

import (
	"maps"
	"slices"
	"strings"
	"unicode"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/projectgraph/modules"
)

var javaProfile = profile{
	code:            langcode.Java,
	family:          langcode.Java,
	resolver:        func(b *builder) modules.Resolver { return javaResolver{b} },
	units:           (*builder).addJavaUnits,
	link:            (*builder).linkClassInits,
	qualified:       (*builder).resolveJava,
	scopeFallback:   (*builder).javaScopeFallback,
	instance:        []string{"this"},
	thisCalls:       true,
	superCalls:      true,
	implicitBases:   true,
	implicitFields:  true,
	nestedTypes:     true,
	annotationsData: true,
	scopeWildcards:  true,
	exactWildcards:  true,
}

// javaResolver resolves a Java import to a class or a package of the
// project.
type javaResolver struct{ b *builder }

func (r javaResolver) Module(_, spec string) (string, string, bool) {
	if key, ok := r.b.resolveJava(spec); ok {
		return key, "class", true
	}
	return "", "", false
}

// FirstParty reports whether a qualified name starts with a package of the
// project.
func (r javaResolver) FirstParty(spec string) bool {
	return r.b.javaFirstParty(spec)
}

// javaScopeFallback resolves a name that no scope of the file binds: a
// class or a member of a wildcard import of a dependency, or a qualified
// name, as org.yaml.Yaml.load.
func (b *builder) javaScopeFallback(sc *scope, first string, rest []string, target string) []resolution {
	var out []resolution
	for _, key := range b.externalWildcards(sc, first) {
		out = append(out, resolution{key: b.external(fileindex.Join(key, rest...)), kind: EdgeWildcard, rule: "wildcard-external"})
	}
	if len(out) == 0 && qualifiedJava(target) {
		dotted := strings.ReplaceAll(target, fileindex.Separator, ".")
		if key, ok := b.resolveJava(dotted); ok {
			return []resolution{b.walk(key, nil)}
		}
		return []resolution{{key: b.external(target), kind: EdgeDirect, rule: "external"}}
	}
	return out
}

// addJavaUnits groups the Java files by their package clause. A Java
// package is the class table of its classes: an import names a class by
// its package and its name, and a class names the classes of its package
// with no import.
func (b *builder) addJavaUnits() {
	files := map[string][]string{}
	for _, p := range b.paths {
		if ix := b.byPath[p]; ix != nil && ix.Language == langcode.Java {
			files[ix.Package] = append(files[ix.Package], p)
		}
	}
	for _, pkg := range slices.Sorted(maps.Keys(files)) {
		b.javaPackages[pkg] = b.addUnit("java:"+pkg, files[pkg])
	}
}

// resolveJava resolves a qualified name, as com.x.Service or
// com.x.Outer.Inner, to the unit of a package or to a class of the project
// and a member under it. The longest package that the name starts with
// wins. The default package has no name, so no import names it.
func (b *builder) resolveJava(name string) (string, bool) {
	parts := strings.Split(name, ".")
	for i := len(parts); i > 0; i-- {
		u := b.javaPackages[strings.Join(parts[:i], ".")]
		if u == nil {
			continue
		}
		if i == len(parts) {
			return u.key, true
		}
		if r := b.walk(u.key, parts[i:]); r.fail == nil {
			return r.key, true
		}
		return "", false
	}
	return "", false
}

// javaFirstParty reports whether a qualified name starts with a package of
// the project. It looks up each dotted prefix of the name, because a scan
// of the packages at each import grows with the square of the project.
func (b *builder) javaFirstParty(name string) bool {
	for prefix := name; prefix != ""; {
		if _, ok := b.javaPackages[prefix]; ok {
			return true
		}
		i := strings.LastIndexByte(prefix, '.')
		if i < 0 {
			return false
		}
		prefix = prefix[:i]
	}
	return false
}

// linkClassInits adds an edge from the class node and each method of a
// Java class to the static initializer <clinit> of the class, which the
// index defines when the class runs code at load. The JVM runs that code
// before the first instance, the first static call and the first access to
// a static field. The edge goes to <clinit> and not to the class node,
// because the class node leads to the constructor, which a static call
// does not run.
func (b *builder) linkClassInits(ix *fileindex.Index) {
	for _, c := range ix.Classes {
		init := fileindex.Join(c.Key, javaClassInit)
		if !b.isMethod(init) {
			continue
		}
		for _, from := range append([]string{c.Key}, c.Methods...) {
			if from == init || from != c.Key && !b.isMethod(from) {
				continue
			}
			n := b.g.Nodes[from]
			b.addEdge(Edge{From: from, To: init, Site: Site{Path: n.Path, Line: n.Line}, Kind: EdgeDirect, Resolver: resolverName(ix.Language, "class-init")})
		}
	}
}

// javaClassInit names the static initializer of a Java class.
const javaClassInit = "<clinit>"

// qualifiedJava reports whether a key is a qualified Java name, as
// org//yaml//Yaml//load: two or more package names in lower case and then
// a class name in upper case.
func qualifiedJava(key string) bool {
	segs := strings.Split(key, fileindex.Separator)
	for i, seg := range segs {
		if startsUpper(seg) {
			return i >= 2
		}
	}
	return false
}

// externalWildcards returns the external keys that a name can have through
// the Java wildcard imports that name no package of the project. Java
// names a class with an upper case letter and a member with a lower case
// letter, so a class name comes from a package, as List from java.util.*,
// and a member name from a class, as assertEquals from a static import of
// org.junit.Assert.*.
func (b *builder) externalWildcards(sc *scope, name string) []string {
	var keys []string
	for _, imp := range sc.wildcards {
		if _, _, ok := b.resolveModule(sc.ix, imp.Module); ok {
			continue
		}
		if startsUpper(lastDotted(imp.Module)) != startsUpper(name) {
			keys = append(keys, fileindex.Join(fileindex.Namespace(sc.ix.Language, imp.Module), name))
		}
	}
	return keys
}

func startsUpper(s string) bool {
	for _, r := range s {
		return unicode.IsUpper(r)
	}
	return false
}

// thisMethod resolves a call with no receiver in a Java method to a method
// of a class that holds the caller, or of its bases. The receiver is this,
// so a subclass can override the method.
func (b *builder) thisMethod(caller, name string) (resolution, bool) {
	for class, ok := b.enclosingClass(caller); ok; class, ok = b.enclosingClass(fileindex.Parent(class)) {
		if _, fail := b.method(class, name); fail == nil {
			return b.walkTyped(class, []string{name}), true
		}
	}
	return resolution{}, false
}

// nestedType returns the class that a Java name names in the classes that
// hold the key, as Inner in a method of Outer for Outer.Inner.
func (b *builder) nestedType(sc *scope, from, name string) (string, bool) {
	if !sc.profile.nestedTypes {
		return "", false
	}
	for class, ok := b.enclosingClass(from); ok; class, ok = b.enclosingClass(fileindex.Parent(class)) {
		if nested := fileindex.Join(class, name); b.isType(nested) {
			return nested, true
		}
	}
	return "", false
}
