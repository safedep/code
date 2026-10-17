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
// external base.
func (b *builder) method(class, name string) (string, *failure) {
	order := b.mro(class)
	for _, c := range order {
		if n := b.g.Nodes[fileindex.Join(c, name)]; n != nil {
			return n.Key, nil
		}
	}
	for _, c := range order {
		rec, ok := b.classes[c]
		if !ok {
			continue
		}
		for _, base := range b.bases(rec) {
			if !b.isType(base) && b.g.IsExternal(base) {
				return b.external(fileindex.Join(base, name)), nil
			}
		}
	}
	return "", noDefinition(name)
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

// overrides returns the first-party methods that a call of the method name
// on an instance of the class can run besides found: for each subclass, the
// method that its MRO gives. This is class hierarchy analysis.
func (b *builder) overrides(class, name, found string) []string {
	var out []string
	for _, sub := range b.subclasses(class) {
		key, fail := b.method(sub, name)
		if fail == nil && key != found && b.isMethod(key) && !slices.Contains(out, key) {
			out = append(out, key)
		}
	}
	slices.Sort(out)
	return out
}
