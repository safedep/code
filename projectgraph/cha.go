package projectgraph

import (
	"slices"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
)

// isType reports whether the key names a first-party class or interface.
func (b *builder) isType(key string) bool {
	k := b.kindOf(key)
	return k == NodeClass || k == NodeInterface
}

// typeBases returns the first-party classes and interfaces that a class
// extends or implements, in source order.
func (b *builder) typeBases(class string) []string {
	rec, ok := b.classes[class]
	if !ok {
		return nil
	}
	return slices.DeleteFunc(b.bases(rec), func(base string) bool { return !b.isType(base) })
}

// mro returns the first-party classes in the order that a method lookup on
// the class reads them, the class first. Python reads the C3 linearization
// of the bases, and falls back to a depth-first order when the bases have
// no linearization. The other languages read the bases depth first.
func (b *builder) mro(class string) []string {
	if order, ok := b.mros[class]; ok {
		return order
	}
	b.mros[class] = []string{class}
	var order []string
	if rec, ok := b.classes[class]; ok && profileOf(rec.ix.Language).c3 {
		order = b.c3(class)
	}
	if order == nil {
		order = b.depthFirst(class, 0, map[string]bool{})
	}
	b.mros[class] = order
	return order
}

func (b *builder) depthFirst(class string, depth int, seen map[string]bool) []string {
	if seen[class] || depth > b.opts.maxExportHops {
		return nil
	}
	seen[class] = true
	order := []string{class}
	for _, base := range b.typeBases(class) {
		order = append(order, b.depthFirst(base, depth+1, seen)...)
	}
	return order
}

// method returns the method of a class or of its bases in the order of the
// MRO. A method that no first-party class defines falls to the first
// external base, because a dependency can give it.
func (b *builder) method(class, name string) (string, *failure) {
	if key, ok := b.ownMethod(class, name); ok {
		return key, nil
	}
	if base, ok := b.externalBase(class); ok {
		return b.external(fileindex.Join(base, name)), nil
	}
	return "", noDefinition(name)
}

// ownMethod returns the member of the name that a first-party class of the
// MRO of the class defines.
func (b *builder) ownMethod(class, name string) (string, bool) {
	for _, c := range b.mro(class) {
		if n := b.g.Nodes[fileindex.Join(c, name)]; n != nil {
			return n.Key, true
		}
		if key, ok := b.member(c, name); ok {
			return key, true
		}
	}
	return "", false
}

// externalBase returns the first external base of a class of the MRO of
// the class.
func (b *builder) externalBase(class string) (string, bool) {
	for _, c := range b.mro(class) {
		rec, ok := b.classes[c]
		if !ok {
			continue
		}
		for _, base := range b.bases(rec) {
			if !b.isType(base) && b.g.IsExternal(base) {
				return base, true
			}
		}
	}
	return "", false
}

// constructor returns the constructor that runs when a call makes an
// instance of the class.
func (b *builder) constructor(class string) (string, bool) {
	for _, c := range b.mro(class) {
		rec, ok := b.classes[c]
		if !ok {
			continue
		}
		for _, m := range rec.class.Methods {
			if b.kindOf(m) == NodeConstructor {
				return m, true
			}
		}
	}
	return "", false
}

// subclasses returns the first-party classes that extend or implement the
// class, at any depth, in path order.
func (b *builder) subclasses(class string) []string {
	if b.children == nil {
		b.children = map[string][]string{}
		for _, p := range b.paths {
			ix := b.byPath[p]
			if ix == nil {
				continue
			}
			for _, c := range ix.Classes {
				for _, base := range b.typeBases(c.Key) {
					b.children[base] = append(b.children[base], c.Key)
				}
			}
		}
	}
	var out []string
	seen := map[string]bool{class: true}
	for queue := []string{class}; len(queue) > 0; queue = queue[1:] {
		for _, child := range b.children[queue[0]] {
			if !seen[child] {
				seen[child] = true
				out = append(out, child)
				queue = append(queue, child)
			}
		}
	}
	return out
}

// overrides returns the first-party methods that a call of the method on
// an instance of the class can run besides found: for each subclass, the
// method that its MRO gives. This is class hierarchy analysis. A Go type
// implements an interface with no declaration, so a Go interface gets the
// methods of the project with the name and the arity of the call whose
// type implements the interface.
func (b *builder) overrides(class string, call fileindex.Call, found string) []string {
	if b.profileAt(class).implicitInterfaces {
		return b.implementations(class, call)
	}
	var out []string
	for _, sub := range b.subclasses(class) {
		key, fail := b.method(sub, call.Method)
		if fail == nil && key != found && b.isMethod(key) && !slices.Contains(out, key) {
			out = append(out, key)
		}
	}
	family := languageFamily(b.languageOf(class))
	for _, open := range b.openClasses() {
		if languageFamily(b.languageOf(open)) != family {
			continue
		}
		if key, ok := b.ownMethod(open, call.Method); ok && key != found && b.isMethod(key) && !slices.Contains(out, key) {
			out = append(out, key)
		}
	}
	slices.Sort(out)
	return out
}

// openClasses returns the first-party classes of Python, JavaScript and
// TypeScript with a base that names no class, in path order: a call, as
// Loading(Base) in class App extends Loading(Base), or a parameter, as B in
// (B) => class extends B {}. Such a base is a value of the program, so the
// class can extend any class. A Go or Java base is a type, and it names a
// class or a type of a dependency.
func (b *builder) openClasses() []string {
	if b.open != nil {
		return b.open
	}
	b.open = []string{}
	for _, p := range b.paths {
		ix := b.byPath[p]
		if ix == nil || !profileOf(ix.Language).openBases {
			continue
		}
		sc := b.scopes[p]
		for _, c := range ix.Classes {
			if slices.ContainsFunc(c.Bases, func(base string) bool {
				_, ok := b.resolveNameIn(sc, fileindex.Parent(c.Key), base)
				return !ok && !langcode.IsBuiltin(ix.Language, base)
			}) {
				b.open = append(b.open, c.Key)
			}
		}
	}
	return b.open
}
