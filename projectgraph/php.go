package projectgraph

import (
	"strings"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/projectgraph/modules"
)

// phpProfile reads the namespaces of PHP as Java reads its packages, with
// the Composer autoload map for a class whose file has no index. An
// include names a file by its path.
var phpProfile = profile{
	code:              langcode.PHP,
	resolver:          func(b *builder, _ *space) modules.Resolver { return phpResolver{b} },
	units:             (*builder).addPHPUnits,
	link:              (*builder).linkPHP,
	qualified:         func(b *builder, name string) (string, bool) { return b.resolveQualified(langcode.PHP, name) },
	scopeFallback:     (*builder).phpScopeFallback,
	dependencies:      func(b *builder) ([]string, []modules.Problem) { return b.phpState().composer.Dependencies(), nil },
	instance:          []string{"$this", "static"},
	lateStatic:        "static",
	superCalls:        true,
	implicitBases:     true,
	annotationsData:   true,
	definitionExports: true,
	implicitMethods:   phpMagic,
}

// phpMagic name the methods that PHP calls with no call of the name: the
// magic methods and the methods of the interfaces of the runtime, as
// offsetGet for $a['x'] or getIterator for a foreach.
var phpMagic = map[string]bool{
	"__destruct": true, "__toString": true, "__invoke": true, "__get": true, "__set": true,
	"__isset": true, "__unset": true, "__call": true, "__callStatic": true, "__clone": true,
	"__sleep": true, "__wakeup": true, "__serialize": true, "__unserialize": true,
	"offsetGet": true, "offsetSet": true, "offsetExists": true, "offsetUnset": true,
	"getIterator": true, "current": true, "next": true, "key": true, "valid": true,
	"rewind": true, "count": true, "jsonSerialize": true,
}

// phpState is the state of PHP in a build: the autoload maps of Composer.
type phpState struct {
	composer *modules.Composer
}

func (b *builder) phpState() *phpState {
	return langState[phpState](b, langcode.PHP)
}

func (b *builder) addPHPUnits() {
	b.addNamespaceUnits(langcode.PHP, `\`, langcode.PHP)
	composer := modules.NewComposer(b.project.files, b.opts.Root)
	b.phpState().composer = composer
	b.problems = append(b.problems, composer.Problems()...)
}

// linkPHP adds the edges of the class bodies, as C# does. PHP runs no
// code when a class loads.
func (b *builder) linkPHP(ix *fileindex.Index) {
	b.linkClassBodies(ix)
}

// phpResolver resolves an include to a file by its path, and a use to a
// class or a namespace of the project by the namespace table and then by
// the Composer autoload map.
type phpResolver struct{ b *builder }

func (r phpResolver) Module(importer, spec string) (string, string, bool) {
	if phpPath(spec) {
		for _, base := range []string{modules.Dir(importer), ""} {
			if p, ok := modules.Join(base, spec); ok && r.b.project.files.Has(p) {
				return p, "path", true
			}
			if strings.HasPrefix(spec, ".") {
				break
			}
		}
		return "", "", false
	}
	if key, ok := r.b.resolveQualified(langcode.PHP, spec); ok {
		return key, "class", true
	}
	for _, file := range r.b.phpState().composer.Files(spec) {
		if !r.b.project.files.Has(file) {
			continue
		}
		if class := fileindex.Join(file, lastName(spec)); r.b.isType(class) {
			return class, "psr-4", true
		}
		return file, "psr-4", true
	}
	return "", "", false
}

// FirstParty reports whether a path is relative or names a file of the
// project, or a qualified name starts with a namespace of the project or
// with a prefix of the autoload map.
func (r phpResolver) FirstParty(spec string) bool {
	if phpPath(spec) {
		return true
	}
	return r.b.namespaces[langcode.PHP].firstParty(spec) || r.b.phpState().composer.FirstParty(spec)
}

// phpPath reports whether the spec of an import names a file, as an
// include does.
func phpPath(spec string) bool {
	return strings.HasSuffix(spec, ".php") || strings.Contains(spec, "/")
}

func lastName(name string) string {
	return name[strings.LastIndex(name, `\`)+1:]
}

// phpScopeFallback resolves a name that no use and no definition of the
// namespace of the file binds: a qualified name of the project, relative to
// the namespace of the file, as Util\helper in App for App\Util\helper, or
// absolute, a function or a class of the global namespace of the
// project, or else a class of the runtime or of a dependency, which PHP
// names by an upper case name, as Exception or GuzzleHttp\Client.
func (b *builder) phpScopeFallback(sc *scope, _, first string, rest []string, target string) []resolution {
	qualified := strings.ReplaceAll(target, fileindex.Separator, `\`)
	for _, name := range []string{sc.ix.Package + `\` + qualified, qualified} {
		if key, ok := b.resolveQualified(langcode.PHP, name); ok && len(rest) > 0 {
			return []resolution{b.walkNamespace(key)}
		}
	}
	if u := b.namespaces[langcode.PHP].units[""]; u != nil && sc.ix.Package != "" {
		var out []resolution
		for _, key := range u.names[first] {
			out = append(out, b.walk(key, rest))
		}
		if len(out) > 0 {
			return out
		}
	}
	if len(rest) > 0 && startsUpper(first) {
		return []resolution{b.externalResolution(target)}
	}
	return nil
}
