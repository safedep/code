package projectgraph

import (
	"slices"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
)

// instanceKeywords name the receiver that holds the instance in a method.
// A call on it can reach the method of any subclass.
var instanceKeywords = map[langcode.Code][]string{
	langcode.Python:     {"self", "cls"},
	langcode.Javascript: {"this"},
	langcode.Typescript: {"this"},
	langcode.Java:       {"this"},
}

func isInstanceKeyword(lang langcode.Code, name string) bool {
	return slices.Contains(instanceKeywords[lang], name)
}

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
	if rec, ok := b.classes[class]; ok && rec.ix.Language == langcode.Python {
		order = b.c3(class)
	}
	if order == nil {
		order = b.depthFirst(class, 0, map[string]bool{})
	}
	b.mros[class] = order
	return order
}

func (b *builder) c3(class string) []string {
	bases := b.typeBases(class)
	lists := make([][]string, 0, len(bases)+1)
	for _, base := range bases {
		lists = append(lists, slices.Clone(b.mro(base)))
	}
	lists = append(lists, slices.Clone(bases))
	order := []string{class}
	for {
		lists = slices.DeleteFunc(lists, func(l []string) bool { return len(l) == 0 })
		if len(lists) == 0 {
			return order
		}
		head := ""
		for _, l := range lists {
			if !slices.ContainsFunc(lists, func(other []string) bool { return slices.Contains(other[1:], l[0]) }) {
				head = l[0]
				break
			}
		}
		if head == "" || slices.Contains(order, head) {
			return nil
		}
		order = append(order, head)
		for i, l := range lists {
			if l[0] == head {
				lists[i] = l[1:]
			}
		}
	}
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
	if b.languageOf(class) == langcode.Go {
		return b.implementations(class, call)
	}
	var out []string
	for _, sub := range b.subclasses(class) {
		key, fail := b.method(sub, call.Method)
		if fail == nil && key != found && b.isMethod(key) && !slices.Contains(out, key) {
			out = append(out, key)
		}
	}
	slices.Sort(out)
	return out
}

// implementations returns the Go methods of the project that a call of a
// method of the interface can run: the methods with the name of the call
// and a signature that takes its arguments, of a type that implements the
// interface. One call as the only argument can give more values, so it
// takes any arity.
func (b *builder) implementations(iface string, call fileindex.Call) []string {
	if b.goMethods == nil {
		b.goMethods = map[string][]string{}
		for _, p := range b.paths {
			ix := b.byPath[p]
			if ix == nil || ix.Language != langcode.Go {
				continue
			}
			for _, def := range ix.Definitions {
				if def.Kind == fileindex.DefMethod && len(fileindex.Segments(def.Key)) == 2 {
					b.goMethods[def.Name] = append(b.goMethods[def.Name], def.Key)
				}
			}
		}
	}
	args := len(call.Args)
	spread := args == 1 && call.Args[0].Shape == fileindex.ArgComputed
	var out []string
	for _, key := range b.goMethods[call.Method] {
		def := b.defs[key]
		takes := spread || def.Arity == args || def.Variadic && args >= def.Arity-1
		if takes && b.implements(fileindex.Parent(key), iface) {
			out = append(out, key)
		}
	}
	slices.Sort(out)
	return out
}

// implements reports whether the method set of a Go type holds each method
// of a first-party interface by name and arity. The method set has the
// methods of the type and of the types that it embeds. A method that an
// embedded type of a dependency can give counts as held. The methods of an
// embedded interface of a dependency are not known, so the type needs none
// of them.
func (b *builder) implements(typ, iface string) bool {
	k := [2]string{typ, iface}
	if held, ok := b.implemented[k]; ok {
		return held
	}
	held := true
	for _, sig := range b.abstract(iface, 0) {
		key, fail := b.method(typ, sig.Name)
		def := b.defs[key]
		if fail != nil || def != nil && (def.Arity != sig.Arity || def.Variadic != sig.Variadic) {
			held = false
			break
		}
	}
	b.implemented[k] = held
	return held
}

// abstract returns the methods that a first-party Go interface declares,
// with the methods of the first-party interfaces that it embeds.
func (b *builder) abstract(iface string, depth int) []fileindex.Signature {
	rec, ok := b.classes[iface]
	if !ok || depth > b.opts.maxExportHops {
		return nil
	}
	sigs := slices.Clone(rec.class.Abstract)
	for _, base := range b.typeBases(iface) {
		sigs = append(sigs, b.abstract(base, depth+1)...)
	}
	return sigs
}
