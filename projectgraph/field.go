package projectgraph

import (
	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
)

// maxNameCandidates caps the field and name edges of one site. A site with
// more candidates is unresolved with the reason ambiguous, as the
// field-based edges of ACG do.
const maxNameCandidates = 20

// linkFields adds an edge from the caller of each unresolved site with a
// method name to each first-party definition that the call can run by that
// name. The site stays unresolved, so an external target with the name
// stays uncertain. A test file holds no candidate for a caller outside the
// tests. A site whose receiver a class narrowed has its cha edges instead.
//
// A JavaScript or TypeScript member call, as x.parse(), reads a property of
// x. So its field edges go to the methods, the object properties and the
// property assignments of the name, as ACG does. An exported function is a
// property of the module object, so it is a candidate too. Any other site,
// as a Python member call or a call of a parameter, gets name edges to the
// functions, methods, constructors and classes of the name.
func (b *builder) linkFields() {
	fields, names := b.definitionsByName()
	for _, u := range sortedKeys(b.unresolved, compareUnresolved) {
		if u.Method == "" || ReachesAny(u.Reason) || b.narrowed[u.Site] {
			continue
		}
		ix := b.byPath[u.Site.Path]
		if ix == nil {
			continue
		}
		kind, byName := EdgeName, names
		if ix.Language.IsECMAScript() && u.Receiver != "" {
			kind, byName = EdgeField, fields
		}
		family := languageFamily(ix.Language)
		var candidates []string
		for _, key := range byName[familyName{family: family, name: u.Method}] {
			if !b.test[fileindex.Unit(key)] || b.test[u.Site.Path] {
				candidates = append(candidates, key)
			}
		}
		if len(candidates) > maxNameCandidates {
			delete(b.unresolved, u)
			u.Reason, u.Method = fileindex.UnresolvedAmbiguous, ""
			b.addUnresolved(u)
			continue
		}
		for _, key := range candidates {
			b.addEdge(Edge{From: u.Caller, To: key, Site: u.Site, Kind: kind, Resolver: resolverName(ix.Language, string(kind))})
		}
	}
}

type familyName struct {
	family langcode.Code
	name   string
}

// definitionsByName returns the keys of the definitions of the project by
// language family and name, in path order: the methods and the exported
// functions for field edges, and the functions, methods, constructors and
// classes for name edges. An object property or a property assignment that
// holds a function is a method in the index.
func (b *builder) definitionsByName() (fields, names map[familyName][]string) {
	fields, names = map[familyName][]string{}, map[familyName][]string{}
	for _, p := range b.paths {
		ix := b.byPath[p]
		if ix == nil || isDeclarationFile(p) {
			continue
		}
		family := languageFamily(ix.Language)
		for _, def := range ix.Definitions {
			k := familyName{family: family, name: def.Name}
			switch def.Kind {
			case fileindex.DefMethod:
				fields[k] = append(fields[k], def.Key)
				names[k] = append(names[k], def.Key)
			case fileindex.DefFunction:
				if def.Exported {
					fields[k] = append(fields[k], def.Key)
				}
				names[k] = append(names[k], def.Key)
			case fileindex.DefConstructor:
				names[k] = append(names[k], def.Key)
			case fileindex.DefClass:
				// A call of a Go type name is a conversion and runs no
				// code.
				if ix.Language != langcode.Go {
					names[k] = append(names[k], def.Key)
				}
			}
		}
	}
	return fields, names
}

// ReachesAny reports whether an unresolved site of the reason can call any
// target. A computed name or a computed module can name every target, even
// when the site names the function that it calls, as import_module.
func ReachesAny(reason fileindex.UnresolvedReason) bool {
	return reason == fileindex.UnresolvedComputedName || reason == fileindex.UnresolvedDynamicImport
}

// languageFamily joins the languages whose files call each other.
func languageFamily(lang langcode.Code) langcode.Code {
	if lang == langcode.Typescript {
		return langcode.Javascript
	}
	return lang
}
