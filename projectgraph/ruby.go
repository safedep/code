package projectgraph

import (
	"path"
	"slices"
	"strings"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/projectgraph/modules"
)

// rubyProfile reads Ruby as one namespace: a file defines its modules,
// classes and top-level methods for every file that the program loads, and
// a second file can reopen a class. So the Ruby files are one unit, and the
// definitions of one constant path are one class.
var rubyProfile = profile{
	code:               langcode.Ruby,
	family:             langcode.Ruby,
	resolver:           func(b *builder, _ *space) modules.Resolver { return rubyResolver{b} },
	units:              func(b *builder) { b.addLanguageUnit(langcode.Ruby) },
	aliases:            (*builder).joinReopenedClasses,
	link:               (*builder).linkImplicitMethods,
	scopeFallback:      (*builder).constantFallback,
	instance:           []string{"self"},
	thisCalls:          true,
	superCalls:         true,
	objectBase:         "Object",
	classBodyInDefiner: true,
	implicitBases:      true,
	definitionExports:  true,
	constructorCall:    "new",
	implicitMethods:    rubyImplicit,
	classMethodPrefix:  "self.",
	classOf:            "class",
	nestedTypes:        true,
}

// rubyImplicit name the methods that Ruby code calls with no call of the
// name: the operators, the conversions and the hooks of the runtime.
var rubyImplicit = map[string]bool{
	"to_s": true, "inspect": true, "to_str": true, "to_ary": true, "to_proc": true, "to_h": true,
	"each": true, "<=>": true, "==": true, "eql?": true, "hash": true, "call": true, "coerce": true,
	"method_missing": true, "respond_to_missing?": true, "[]": true, "[]=": true, "+": true,
	"-": true, "*": true, "/": true, "<<": true, "initialize_copy": true, "included": true,
	"inherited": true, "extended": true, "%": true, "**": true, "===": true, "=~": true, "!": true,
	"!=": true, "<": true, ">": true, "<=": true, ">=": true, "-@": true, "+@": true, "~": true,
	"&": true, "|": true, "^": true, ">>": true, "to_a": true, "to_i": true, "to_int": true,
	"to_hash": true, "to_sym": true, "respond_to?": true, "self.included": true,
	"self.inherited": true, "self.extended": true, "self.prepended": true,
	"self.method_added": true, "self.const_missing": true, "self.method_missing": true,
}

// addLanguageUnit makes the files of the language one unit, whose top-level
// names are in scope in each file.
func (b *builder) addLanguageUnit(lang langcode.Code) {
	var files []string
	for _, p := range b.paths {
		if ix := b.byPath[p]; ix != nil && ix.Language == lang {
			files = append(files, p)
		}
	}
	if len(files) > 0 {
		b.addUnit(string(lang)+":", files)
	}
}

// joinReopenedClasses makes the classes and modules of one constant path
// one class, as Shop::Loader in two files.
func (b *builder) joinReopenedClasses() {
	b.constants = map[string][]string{}
	for _, p := range b.paths {
		ix := b.byPath[p]
		if ix == nil || ix.Language != langcode.Ruby {
			continue
		}
		for _, c := range ix.Classes {
			path := strings.Join(fileindex.Segments(c.Key), "::")
			b.constants[path] = append(b.constants[path], c.Key)
		}
	}
	var list [][]string
	for _, g := range b.constants {
		list = append(list, slices.Clone(g))
	}
	b.joinAliases(list)
}

// constantFallback resolves a constant that the project does not name, as
// YAML or Net::HTTP, to the external key of a library. Ruby names a
// constant with an upper case letter, and a method with a lower case one,
// which stays a record.
func (b *builder) constantFallback(_ *scope, _, first string, _ []string, target string) []resolution {
	if !startsUpper(first) {
		return nil
	}
	return []resolution{{key: b.external(target), kind: EdgeDirect, rule: "external"}}
}

// constantPath returns the Ruby constant path of a name in a class, as
// Shop::Loader for Loader in Shop.
func constantPath(class, name string) string {
	return strings.Join(append(fileindex.Segments(class), name), "::")
}

// rubyResolver resolves a require_relative by the path of the importer and a
// require by the lib directories of the project and its root.
type rubyResolver struct{ b *builder }

func (r rubyResolver) Module(importer, spec string) (string, string, bool) {
	file := strings.TrimSuffix(spec, ".rb") + ".rb"
	if strings.HasPrefix(spec, ".") {
		if p, ok := modules.Join(modules.Dir(importer), file); ok && r.b.project.files.Has(p) {
			return p, "relative", true
		}
		return "", "", false
	}
	for _, dir := range r.b.rubyLibs() {
		if p, ok := modules.Join(dir, file); ok && r.b.project.files.Has(p) {
			return p, "lib", true
		}
	}
	return "", "", false
}

// FirstParty reports whether a require is relative or starts with the name
// of a file or a directory of a lib directory of the project.
func (r rubyResolver) FirstParty(spec string) bool {
	first, _, _ := strings.Cut(spec, "/")
	return strings.HasPrefix(spec, ".") || r.b.project.files.HasName(strings.TrimSuffix(first, ".rb"))
}

// rubyLibs returns the directories that a require reads, as Bundler and a
// gemspec put them on the load path: each lib directory, and the root.
func (b *builder) rubyLibs() []string {
	if b.rubyLoadPath == nil {
		b.rubyLoadPath = []string{}
		for _, d := range b.project.files.Dirs() {
			if path.Base(d) == "lib" {
				b.rubyLoadPath = append(b.rubyLoadPath, d)
			}
		}
		b.rubyLoadPath = append(b.rubyLoadPath, "")
	}
	return b.rubyLoadPath
}
