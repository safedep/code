package projectgraph

import (
	"strings"
	"unicode"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/projectgraph/modules"
)

var javaProfile = profile{
	code:            langcode.Java,
	resolver:        func(b *builder, _ *space) modules.Resolver { return namespaceResolver{b, langcode.Java} },
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

// javaLang is the key of the package that a Java or a Kotlin file names
// with no import.
const javaLang = "java//lang"

// javaScopeFallback resolves a name that no scope of the file binds: a
// class or a member of a wildcard import of a dependency, a qualified
// name, as org.yaml.Yaml.load, or a class of java.lang, as Thread, whose
// key goes under java//lang so a symbol record matches it.
func (b *builder) javaScopeFallback(sc *scope, _, first string, rest []string, target string) []resolution {
	var out []resolution
	for _, key := range b.externalWildcards(sc, first) {
		out = append(out, resolution{key: b.external(fileindex.Join(key, rest...)), kind: EdgeWildcard, rule: "wildcard-external"})
	}
	switch {
	case len(out) > 0:
		return out
	case qualifiedJava(target):
		dotted := strings.ReplaceAll(target, fileindex.Separator, ".")
		if key, ok := b.resolveJava(dotted); ok {
			return []resolution{b.walk(key, nil)}
		}
		return []resolution{{key: b.external(target), kind: EdgeDirect, rule: "external"}}
	case langcode.IsJavaLang(first):
		return []resolution{{key: b.external(fileindex.Join(javaLang, target)), kind: EdgeDirect, rule: "java-lang"}}
	}
	return out
}

// addJavaUnits groups the Java and the Kotlin files by their package
// clause. A package is the class table of its classes: an import names a
// class by its package and its name, and a class names the classes of its
// package with no import.
func (b *builder) addJavaUnits() {
	t := b.addNamespaceUnits(langcode.Java, ".", langcode.Java, langcode.Kotlin)
	// Java code names the top-level functions of a Kotlin file through the
	// class of the file, as UtilsKt.helper().
	for _, u := range t.units {
		for _, p := range u.files {
			if b.byPath[p].Language == langcode.Kotlin {
				facade := kotlinFacade(p)
				u.names[facade] = append(u.names[facade], p)
			}
		}
	}
}

// resolveJava resolves a qualified Java or Kotlin name, as com.x.Service.
func (b *builder) resolveJava(name string) (string, bool) {
	return b.resolveQualified(langcode.Java, name)
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
	// new in a class method of Ruby makes an instance of the class, and
	// the edge to the class leads to its constructor.
	if p := profileOf(b.languageOf(caller)); p.constructorCall == name && p.classMethodPrefix != "" && strings.HasPrefix(fileindex.Last(caller), p.classMethodPrefix) {
		if class, ok := b.enclosingClass(caller); ok {
			if _, own := b.ownMethod(class, name); !own {
				return resolution{key: class, kind: EdgeDirect, rule: "constructor"}, true
			}
		}
	}
	for class, ok := b.enclosingClass(caller); ok; class, ok = b.enclosingClass(fileindex.Parent(class)) {
		if _, ok := b.ownMethod(class, name); ok {
			return b.walkTyped(class, []string{b.selfName(class, caller, name)}), true
		}
		if self := b.selfName(class, caller, name); self != name {
			return b.walkTyped(class, []string{self}), true
		}
	}
	return resolution{}, false
}

// selfName returns the name of the method of the class that a call with no
// receiver in the caller names. In a class method of Ruby, self is the
// class, so build names the class method self.build when the class has
// one.
func (b *builder) selfName(class, caller, name string) string {
	prefix := profileOf(b.languageOf(caller)).classMethodPrefix
	if prefix == "" || !strings.HasPrefix(fileindex.Last(caller), prefix) {
		return name
	}
	if _, ok := b.ownMethod(class, prefix+name); ok {
		return prefix + name
	}
	return name
}

// nestedType returns the class that a name names in the classes that hold
// the key, as Inner in a method of Outer for Outer.Inner in Java. A Ruby
// module of another file can hold it too, as Loader in module Shop for
// Shop::Loader.
func (b *builder) nestedType(sc *scope, from, name string) (string, bool) {
	if !sc.profile.nestedTypes {
		return "", false
	}
	for class, ok := b.enclosingClass(from); ok; class, ok = b.enclosingClass(fileindex.Parent(class)) {
		if nested := fileindex.Join(class, name); b.isType(nested) {
			return nested, true
		}
		if keys := b.constants[constantPath(class, name)]; len(keys) > 0 {
			return keys[0], true
		}
	}
	return "", false
}
