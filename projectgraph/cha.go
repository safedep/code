package projectgraph

import (
	"maps"
	"slices"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
)

// isType reports whether the key names a first-party class or interface.
func (b *builder) isType(key string) bool {
	k := b.kindOf(key)
	return k == NodeClass || k == NodeInterface
}

// aliasesOf returns the definitions of one class: the class and the other
// definitions that the language joins to it, as a partial class of C# in
// another file, a Ruby class that another file reopens, or an impl block of
// Rust in another file than its type.
func (b *builder) aliasesOf(class string) []string {
	if group := b.aliases[class]; len(group) > 0 {
		return group
	}
	return []string{class}
}

// joinAliases makes the classes of each group one class.
func (b *builder) joinAliases(groups [][]string) {
	for _, group := range groups {
		slices.Sort(group)
		group = slices.Compact(group)
		if len(group) < 2 {
			continue
		}
		for _, class := range group {
			b.aliases[class] = group
		}
	}
}

// typeBases returns the first-party classes and interfaces that a class
// extends or implements, in source order, with those of its aliases.
func (b *builder) typeBases(class string) []string {
	var out []string
	for _, c := range b.aliasesOf(class) {
		rec, ok := b.classes[c]
		if !ok {
			continue
		}
		for _, base := range b.bases(rec) {
			if b.isType(base) && !slices.Contains(out, base) && !slices.Contains(b.aliasesOf(class), base) {
				out = append(out, base)
			}
		}
	}
	return out
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
	if seen[class] || depth > b.opts.MaxExportHops {
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
		return b.deriveExternal(base, []string{name}), nil
	}
	return "", noDefinition(name)
}

// ownMethod returns the member of the name that a first-party class of the
// MRO of the class defines.
func (b *builder) ownMethod(class, name string) (string, bool) {
	for _, c := range b.mro(class) {
		if group := b.aliases[c]; len(group) > 0 && b.groupMembers != nil {
			if key, ok := b.groupMember(group, name); ok {
				return key, true
			}
			continue
		}
		for _, a := range b.aliasesOf(c) {
			if key, ok := b.ownMember(a, name); ok {
				return key, true
			}
		}
	}
	return "", false
}

// ownMember returns the definition of the name under a class, or the
// member of the name that a file of the unit of the class defines.
func (b *builder) ownMember(class, name string) (string, bool) {
	if n := b.g.Nodes[fileindex.Join(class, name)]; n != nil {
		return n.Key, true
	}
	return b.member(class, name)
}

// groupMember returns the member of the name of a group of aliases, as
// ownMember of the first alias that has one. The table of a group holds
// each name once. A Ruby module that 1,000 files reopen has 1,000 aliases,
// and a scan of them at each lookup costs the square of the project.
func (b *builder) groupMember(group []string, name string) (string, bool) {
	table, ok := b.groupMembers[group[0]]
	if !ok {
		table = map[string]string{}
		for _, a := range group {
			for _, child := range b.childKeys()[a] {
				if _, ok := table[fileindex.Last(child)]; !ok {
					table[fileindex.Last(child)] = child
				}
			}
			if u, segs := b.unitOf[fileindex.Unit(a)], fileindex.Segments(a); u != nil && len(segs) == 1 {
				for _, member := range slices.Sorted(maps.Keys(u.members[segs[0]])) {
					if _, ok := table[member]; !ok {
						table[member] = u.members[segs[0]][member][0]
					}
				}
			}
		}
		b.groupMembers[group[0]] = table
	}
	key, ok := table[name]
	return key, ok
}

// childKeys returns the keys of the nodes by the key of their parent.
func (b *builder) childKeys() map[string][]string {
	if b.nodesByParent == nil {
		b.nodesByParent = map[string][]string{}
		for key := range b.g.Nodes {
			if fileindex.Unit(key) != key {
				b.nodesByParent[fileindex.Parent(key)] = append(b.nodesByParent[fileindex.Parent(key)], key)
			}
		}
	}
	return b.nodesByParent
}

// externalBase returns the first external base of a class of the MRO of
// the class.
func (b *builder) externalBase(class string) (string, bool) {
	if b.externalBases == nil {
		return b.findExternalBase(class)
	}
	base, ok := b.externalBases[class]
	if !ok {
		base, _ = b.findExternalBase(class)
		b.externalBases[class] = base
	}
	return base, base != ""
}

// findExternalBase searches the bases of the MRO of a class for an external
// one. The name of a base can resolve through a wildcard import of the class
// itself, as use self::Component::* in an impl of Component in Rust, and
// that lookup asks for the external base of the same class again. A class
// that the search reads now has no external base, so the search ends.
func (b *builder) findExternalBase(class string) (string, bool) {
	if b.findingBases[class] {
		return "", false
	}
	b.findingBases[class] = true
	defer delete(b.findingBases, class)
	for _, c := range b.mro(class) {
		for _, rec := range b.records(c) {
			for _, base := range b.bases(rec) {
				if !b.isType(base) && b.g.IsExternal(base) {
					return base, true
				}
			}
		}
	}
	return "", false
}

// records returns the class records of a class and of its aliases.
func (b *builder) records(class string) []classRecord {
	group := b.aliasesOf(class)
	if out, ok := b.groupRecords[group[0]]; ok {
		return out
	}
	var out []classRecord
	for _, a := range group {
		if rec, ok := b.classes[a]; ok {
			out = append(out, rec)
		}
	}
	if b.groupRecords != nil && len(group) > 1 {
		b.groupRecords[group[0]] = out
	}
	return out
}

// constructor returns the constructor that runs when a call makes an
// instance of the class.
func (b *builder) constructor(class string) (string, bool) {
	for _, c := range b.mro(class) {
		for _, rec := range b.records(c) {
			for _, m := range rec.class.Methods {
				if b.kindOf(m) == NodeConstructor {
					return m, true
				}
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
				// A class with a base of a dependency, as Worker(Thread), is
				// a subclass of the external key of the base, so a call on
				// a value of that declared type reaches its overrides.
				if rec, ok := b.classes[c.Key]; ok {
					for _, base := range b.bases(rec) {
						if !b.isType(base) && b.g.IsExternal(base) && !slices.Contains(b.children[base], c.Key) {
							b.children[base] = append(b.children[base], c.Key)
						}
					}
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
// method that its MRO gives. This is class hierarchy analysis. An open
// class of another unit gives no method, as a field candidate of another
// unit does not. A Go type implements an interface with no declaration, so
// a Go interface gets the methods of the project with the name and the
// arity of the call whose type implements the interface.
func (b *builder) overrides(class string, call fileindex.Call, found string) []string {
	if b.profileAt(class).implicitInterfaces {
		return b.implementations(class, call)
	}
	var out []string
	for _, c := range b.classDefinitions(class) {
		for _, sub := range b.subclasses(c) {
			key, fail := b.method(sub, call.Method)
			if fail == nil && key != found && b.isMethod(key) && !slices.Contains(out, key) {
				out = append(out, key)
			}
		}
	}
	family := b.languageOf(class).Family()
	for _, open := range b.openClasses() {
		if b.languageOf(open).Family() != family || !sameUnit(open, class) {
			continue
		}
		if key, ok := b.ownMethod(open, call.Method); ok && key != found && b.isMethod(key) && !slices.Contains(out, key) {
			out = append(out, key)
		}
	}
	slices.Sort(out)
	return out
}

// classDefinitions returns the class itself, and for the external key of a
// class of a unit of dependency code, as dep//Doer, the definitions that
// the units give for it, so the subclasses inside the unit count too. A
// class of a unit gives its external key, which a class of the project
// names as its base, so a call of the unit on self reaches the override
// of the project, as Thread.run of threading reaches the run of a Worker.
func (b *builder) classDefinitions(class string) []string {
	out := []string{class}
	if b.g.IsExternal(class) {
		defs, _ := b.unitDefinitions(b.origins[class])
		for _, d := range defs {
			if b.isType(d) {
				out = append(out, d)
			}
		}
	}
	if key, _, ok := b.unitSymbol(class); ok && key != class {
		out = append(out, key)
	}
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
