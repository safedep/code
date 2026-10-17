package projectgraph

import (
	"slices"
	"strings"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/projectgraph/modules"
)

var pythonProfile = profile{
	code:               langcode.Python,
	family:             langcode.Python,
	resolver:           func(_ *builder, sp *space) modules.Resolver { return sp.keyed(sp.python) },
	dependencyCode:     true,
	relativePath:       pythonRelativePath,
	instance:           []string{"self", "cls"},
	objectBase:         "object",
	literals:           map[string]bool{"None": true, "True": true, "False": true, "Ellipsis": true, "NotImplemented": true},
	moduleGetattr:      "__getattr__",
	classBodyInDefiner: true,
	c3:                 true,
	openBases:          true,
	importBindsRoot:    true,
	moduleNames:        true,
	starImports:        true,
}

// pythonRelativePath turns a relative Python module into a relative path,
// as ./ for . and ../pkg/x for ..pkg.x.
func pythonRelativePath(spec string) string {
	rest := strings.TrimLeft(spec, ".")
	return "./" + strings.Repeat("../", len(spec)-len(rest)-1) + strings.ReplaceAll(rest, ".", "/")
}

// c3 returns the C3 linearization of the first-party bases of a class, or
// nil when the bases have none. It merges the linearizations of the bases
// and the list of the bases: the head of a list that is in no tail comes
// next. A count of each class in the tails tells that in one step, so a
// chain of n classes links in a time that grows as n squared and not as
// n cubed. The order stops at the depth cap of depthFirst, so a long chain
// holds a bounded list per class.
func (b *builder) c3(class string) []string {
	bases := b.typeBases(class)
	lists := make([][]string, 0, len(bases)+1)
	for _, base := range bases {
		lists = append(lists, b.mro(base))
	}
	lists = append(lists, bases)
	heads := make([]int, len(lists))
	tails := map[string]int{}
	left := 0
	for _, l := range lists {
		left += len(l)
		for i := 1; i < len(l); i++ {
			tails[l[i]]++
		}
	}
	order := []string{class}
	seen := map[string]bool{class: true}
	for left > 0 && len(order) <= b.opts.maxExportHops {
		head := ""
		for i, l := range lists {
			if heads[i] < len(l) && tails[l[heads[i]]] == 0 {
				head = l[heads[i]]
				break
			}
		}
		if head == "" || seen[head] {
			return nil
		}
		order = append(order, head)
		seen[head] = true
		for i, l := range lists {
			if heads[i] < len(l) && l[heads[i]] == head {
				heads[i]++
				left--
				if heads[i] < len(l) {
					tails[l[heads[i]]]--
				}
			}
		}
	}
	return order
}

// moduleGetattr returns the function of a module that runs for a name that
// the module does not define, when the module has one.
func (b *builder) moduleGetattr(module string) (string, bool) {
	name := b.profileAt(module).moduleGetattr
	n := b.g.Nodes[module]
	if name == "" || n == nil || !slices.ContainsFunc(n.Flags, func(f fileindex.Flag) bool { return f.Kind == fileindex.FlagModuleGetattr }) {
		return "", false
	}
	key := fileindex.Join(module, name)
	return key, b.kindOf(key) == NodeFunction
}

// starVisible reports whether an import of * from the module gives on the
// name. Python gives on only the names of __all__, or every name with no
// leading underscore.
func (b *builder) starVisible(module, name string) bool {
	src := b.scopes[module]
	if src == nil || !src.profile.moduleNames {
		return true
	}
	if src.hasExports {
		return src.exported(name)
	}
	return !strings.HasPrefix(name, "_")
}

// wildcard resolves a bare name through the from m import * of the file. A
// first-party module that defines the name wins. Else each external module
// of a star can define the name, so each one gets a wildcard edge.
func (b *builder) wildcard(sc *scope, name string) []resolution {
	if !sc.profile.starImports {
		return nil
	}
	var external []resolution
	for _, st := range sc.stars {
		mod, _, ok := b.resolveModule(sc.ix, st.spec)
		if !ok {
			key := b.importedExternal(sc.ix, fileindex.Join(fileindex.Namespace(sc.ix.Language, st.spec), name), st.spec, modules.FormImport, []string{name})
			external = append(external, resolution{key: key, kind: EdgeWildcard, rule: "wildcard-external"})
			continue
		}
		if !b.starVisible(mod, name) {
			continue
		}
		if key, fail := b.resolveName(mod, name, 0); fail == nil {
			return []resolution{{key: key, kind: EdgeWildcard, rule: "wildcard"}}
		}
	}
	return external
}
