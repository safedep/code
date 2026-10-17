package projectgraph

import (
	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/projectgraph/modules"
)

// profile is what the link step knows of one language. Each language file
// declares its profile, and profiles lists them. The shared link code reads
// the rules of a language only from here. A language with no profile gets
// the zero profile: its imports resolve to no module and can name any file
// of the project.
type profile struct {
	code langcode.Code
	// family joins the languages whose files call each other, as
	// TypeScript with JavaScript.
	family langcode.Code

	// resolver returns the module resolver of the language for one space
	// of a build: the project, or a unit of dependency code.
	resolver func(b *builder, sp *space) modules.Resolver
	// dependencyCode marks a language whose files of dependency code the
	// link step links, each unit in a space of its own.
	dependencyCode bool
	// units groups the files of the language into units, as Go packages.
	units func(b *builder)
	// aliases joins the definitions of one class, as the partial classes
	// of C#.
	aliases func(b *builder)
	// link adds the edges that the loader of the language runs, as the
	// init functions of Go.
	link func(b *builder, ix *fileindex.Index)
	// qualified resolves a qualified name, as com.x.Service in Java.
	qualified func(b *builder, name string) (string, bool)
	// scopeFallback resolves a name that no scope of the file binds, as a
	// Java class of a wildcard import of a dependency.
	scopeFallback func(b *builder, sc *scope, caller, first string, rest []string, target string) []resolution
	// relativePath turns a relative module into a path, as ./pkg/ for ..pkg.
	// in Python. A module that ends in its separator names a directory.
	relativePath func(spec string) string

	// instance names the receiver that holds the instance in a method. A
	// call on it can reach the method of any subclass.
	instance []string
	// thisCalls marks a language where a call with no receiver in a method
	// runs on this, as in Java.
	thisCalls bool
	// lateStatic names the receiver of a static call that binds the class
	// at run time, as static of PHP. Such a call can run the static method
	// of a subclass.
	lateStatic string
	// superCalls marks a language where super names the base class, so a
	// call of super runs the constructor of the base and super.m() runs
	// its method.
	superCalls bool
	// superModule marks a language where super names the parent module, as
	// super::helper in Rust, and no base class.
	superModule bool
	// objectBase is the builtin base of a class with no base, as object in
	// Python. A call on super of such a class runs its method.
	objectBase string
	// moduleGetattr names the function of a module that runs for a name
	// that the module does not define, as __getattr__ of Python. The index
	// flags a module that defines it.
	moduleGetattr string
	// defaultExport is the name of the value of a whole module, as default
	// in JavaScript.
	defaultExport string

	// classBodyInDefiner marks a language where a class body runs when the
	// scope that defines the class runs, as in Python.
	classBodyInDefiner bool
	// c3 marks a language whose method lookup reads the C3 linearization of
	// the bases, as Python.
	c3 bool
	// openBases marks a language where a base can be a value of the
	// program, as a call or a parameter, so a class can extend any class.
	openBases bool
	// implicitBases marks a language where a base that no import names is a
	// type of an implicit package, as java.lang.
	implicitBases bool
	// implicitFields marks a language where a method names a field of its
	// class with no instance keyword, as Java.
	implicitFields bool
	// nestedTypes marks a language where a class names the classes nested
	// in the classes that hold it with no qualifier, as Java.
	nestedTypes bool
	// implicitInterfaces marks a language where a type implements an
	// interface with no declaration, as Go. Only an interface type can
	// hold a value of another type, and a type of a dependency names the
	// methods that a call on it runs.
	implicitInterfaces bool
	// typeConversions marks a language where a call of a type name
	// converts a value and runs no code, and a value of a type is no
	// function, as Go.
	typeConversions bool
	// structuralTypes marks a language where a value of an interface type
	// can be any object with the methods, as TypeScript.
	structuralTypes bool
	// memberFields marks a language where a member call reads a property of
	// an object, so a call on an unknown receiver gets field edges.
	memberFields bool
	// annotationsData marks a language whose decorators are data that a
	// framework reads, as Java annotations, and run no code where they
	// stand.
	annotationsData bool
	// definitionExports marks a language with no export statement, whose
	// files export their exported definitions to a path rule, as C#.
	definitionExports bool
	// classOf names the method of an instance that gives its class, as
	// class of Ruby.
	classOf string
	// classMethodPrefix starts the name of a method of the class itself, as
	// self.build in Ruby, which a call on the class runs.
	classMethodPrefix string
	// implicitMethods name the methods that the language calls with no
	// call of the name, as an operator of Kotlin.
	implicitMethods map[string]bool
	// forwardMethod names the method of a class that forwards a call that
	// the class has no method for to another type, as deref of Rust. A
	// declared type with it does not narrow the receiver.
	forwardMethod string
	// methodAliases name the methods that a call of a method name can run
	// by another name, as from for into of Rust.
	methodAliases map[string][]string
	// constructorCall names the method of a class that makes an instance,
	// as new of Ruby. A call of it is a call of the class.
	constructorCall string

	// itemPaths joins the module and the item of an import into one path,
	// as store::file::FileStore for use store::{file::FileStore} in Rust.
	itemPaths string
	// unitImports marks a language where a module gives on the names that
	// its imports bind, as a pub use of Rust.
	unitImports bool
	// importBindsRoot marks a language where an import of a dotted module
	// with no item and no alias binds the first segment, as os for import
	// os.path in Python.
	importBindsRoot bool
	// moduleNames marks a language where a module gives on each name that
	// it binds, imports too, unless it lists its exports, and where a
	// package gives on its submodules, as Python.
	moduleNames bool
	// starImports marks a language where an import of * puts the names of
	// a module in the scope of the file, as from m import * of Python.
	starImports bool
	// scopeWildcards marks a language where a wildcard import puts the
	// names of a package or a class in the scope of the file, as a Go dot
	// import or a Java wildcard import.
	scopeWildcards bool
	// exactWildcards marks a language whose wildcard import of a package of
	// the project names a class exactly, so its edge is direct.
	exactWildcards bool
	// mainLoadsPackage marks a language whose program initializes the
	// package of main before main runs, as Go.
	mainLoadsPackage bool
}

// profiles are the languages of the link step, in a fixed order. The units
// of the languages are built in this order.
var profiles = []*profile{&pythonProfile, &javascriptProfile, &typescriptProfile, &goProfile, &javaProfile, &kotlinProfile, &csharpProfile, &phpProfile, &rubyProfile, &rustProfile}

// profilesByCode is filled in init. The functions of a profile call
// profileOf, so a variable initializer would make an initialization cycle.
var profilesByCode = map[langcode.Code]*profile{}

func init() {
	for _, p := range profiles {
		profilesByCode[p.code] = p
	}
}

var noProfile = &profile{}

// profileOf returns the profile of a language.
func profileOf(lang langcode.Code) *profile {
	if p, ok := profilesByCode[lang]; ok {
		return p
	}
	return noProfile
}

// languageFamily joins the languages whose files call each other.
func languageFamily(lang langcode.Code) langcode.Code {
	if f := profileOf(lang).family; f != "" {
		return f
	}
	return lang
}

func isInstanceKeyword(lang langcode.Code, name string) bool {
	for _, k := range profileOf(lang).instance {
		if k == name {
			return true
		}
	}
	return false
}

// profileAt returns the profile of the language of the file of a key.
func (b *builder) profileAt(key string) *profile {
	return profileOf(b.languageOf(key))
}
