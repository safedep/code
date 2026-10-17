package projectgraph

import (
	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
)

// maxNameCandidates caps the name edges of one site. A site with more
// candidates is unresolved with the reason ambiguous, as the field-based
// edges of ACG do.
const maxNameCandidates = 20

// linkNames adds a name edge from the caller of each unresolved site with a
// method name to each first-party definition with that name. The receiver
// of such a call can be any first-party value, so each of these can run. The
// site stays unresolved, so an external target with the name stays
// uncertain. A test file holds no candidate for a caller outside the tests.
func (b *builder) linkNames() {
	byName := b.definitionsByName()
	for _, u := range sortedKeys(b.unresolved, compareUnresolved) {
		if u.Method == "" || ReachesAny(u.Reason) {
			continue
		}
		ix := b.byPath[u.Site.Path]
		if ix == nil {
			continue
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
			b.addEdge(Edge{From: u.Caller, To: key, Site: u.Site, Kind: EdgeName, Resolver: resolverName(ix.Language, "name")})
		}
	}
}

type familyName struct {
	family langcode.Code
	name   string
}

// definitionsByName returns the keys of the functions, methods,
// constructors and classes of the project by language family and name, in
// path order.
func (b *builder) definitionsByName() map[familyName][]string {
	byName := map[familyName][]string{}
	for _, p := range b.paths {
		ix := b.byPath[p]
		if ix == nil || isDeclarationFile(p) {
			continue
		}
		family := languageFamily(ix.Language)
		for _, def := range ix.Definitions {
			switch def.Kind {
			case fileindex.DefFunction, fileindex.DefMethod, fileindex.DefConstructor, fileindex.DefClass:
				k := familyName{family: family, name: def.Name}
				byName[k] = append(byName[k], def.Key)
			}
		}
	}
	return byName
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
