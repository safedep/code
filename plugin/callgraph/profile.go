package callgraph

import (
	"strings"

	"github.com/safedep/code/core"
)

// languageProfile is what the call graph knows of one language. Each
// language file declares its profile, and languageProfiles lists them. The
// shared processors read the node types of a grammar only from here.
type languageProfile struct {
	code core.LanguageCode

	// separators split a qualified name, as os.path or OpenAI::Client.
	separators []string

	// instanceKeyword names the instance in a method, as self or this.
	instanceKeyword string

	// constructorMethod is the method that makes an instance, as __init__.
	// The class calls it.
	constructorMethod string

	// entryFunction is the function that the root namespace calls, as main.
	// rootCallsFunctions makes the root namespace call every function, so
	// that DFS reaches the functions of a Go library with no main.
	entryFunction      string
	rootCallsFunctions bool

	// builtins are the functions and types that a file uses with no import.
	builtins []string

	// processors are the node processors of the grammar. A node type with
	// no processor here gets the processor of neutralProcessor.
	processors map[string]nodeProcessor

	// literals are the node types of a literal value. collections are the
	// node types of a collection literal, whose items can hold calls but
	// which is no value of a name. ignored are the named node types with no
	// call below them, as an import or a comment.
	literals    map[string]bool
	collections map[string]bool
	ignored     map[string]bool

	// reachable are the node types of the definitions that DFS starts from,
	// because a file often exports a definition and does not call it.
	reachable map[string]bool

	// transparent are the node types whose value is the value of one named
	// child, the first one or, for a sequence, the last one.
	transparent map[string]childPosition

	// members are the node types of a member access, as obj.method, and of
	// a call that holds its receiver and its name, as a Java method
	// invocation. They give the receiver and the method of a call.
	members map[string]memberShape

	// memberAssignees are the member node types that an assignment can
	// assign to, as xyz.attr in Python. Their processor gives the assignee.
	memberAssignees map[string]bool

	// strings tells which nodes make a string literal.
	strings stringShape

	// argumentList is the node type of the arguments of a call.
	argumentList string

	// functionLiterals are the node types of a function with no name, as a
	// lambda. A call of one runs its body in the caller.
	functionLiterals map[string]bool

	// valueRoots are the node types at the root of a member chain that hold
	// calls of their own, as createHash('md5') in
	// crypto.createHash('md5').digest().
	valueRoots map[string]bool

	// literalReceivers map a literal node type to the builtin type of its
	// value, so that [1].map() calls Array//map.
	literalReceivers map[string]string

	// importBindsRoot marks a language where an import of a dotted module
	// with no item and no alias binds its first segment, as os for import
	// os.path in Python. The full module is a node too.
	importBindsRoot bool

	// defaultExport is the item that names the module itself, as default
	// in import { default as y } from 'x'.
	defaultExport string

	// signatureFallback is the language whose signatures apply when a
	// signature has none for this language. TypeScript uses the signatures
	// of JavaScript, because the two share their libraries.
	signatureFallback core.LanguageCode

	// trimModule removes the part of an imported module name that names no
	// namespace. It can be nil.
	trimModule func(string) string

	// callables map a function that calls a callable to the position of the
	// argument that names it, as call_user_func of PHP.
	callables map[string]int
}

// childPosition picks the named child that gives the value of a node.
type childPosition int

const (
	firstChild childPosition = iota
	lastChild
)

// memberShape names the parts of a member access, as client.chat in
// client.chat().create. An index shape holds the name as a string index, as
// obj['x'].
type memberShape struct {
	object childRef
	name   childRef
	index  bool
}

// stringShape names the node types of a string literal and of its parts.
// quotes are the named nodes of the quotes, as string_start in Python.
type stringShape struct {
	literals       map[string]bool
	fragments      map[string]bool
	quotes         map[string]bool
	interpolations map[string]bool
}

// languageProfiles are the languages of the call graph, in the order of
// langcode.All.
var languageProfiles = []*languageProfile{
	&pythonProfile,
	&javascriptProfile,
	&javaProfile,
	&goProfile,
	&typescriptProfile,
	&csharpProfile,
	&rustProfile,
	&phpProfile,
	&rubyProfile,
	&kotlinProfile,
}

var profilesByCode = indexProfiles(languageProfiles)

func indexProfiles(profiles []*languageProfile) map[core.LanguageCode]*languageProfile {
	byCode := make(map[core.LanguageCode]*languageProfile, len(profiles))
	for _, profile := range profiles {
		byCode[profile.code] = profile
	}
	return byCode
}

// SupportedLanguages returns the languages that the call graph knows.
func SupportedLanguages() []core.LanguageCode {
	codes := make([]core.LanguageCode, len(languageProfiles))
	for i, profile := range languageProfiles {
		codes[i] = profile.code
	}
	return codes
}

// profileOf returns the profile of code. A language with no profile gets
// an empty one, so its graph has only the grammar-neutral processors.
func profileOf(code core.LanguageCode) *languageProfile {
	if profile, ok := profilesByCode[code]; ok {
		return profile
	}
	return &languageProfile{code: code}
}

// languageProfile returns the profile of the language of the graph. A
// decoded graph gets its profile on the first call. The processors of the
// build read cg.profile, because a profile holds the processors.
func (cg *CallGraph) languageProfile() *languageProfile {
	if cg.profile == nil {
		cg.profile = profileOf(cg.Language)
	}
	return cg.profile
}

func (p *languageProfile) builtinSet() map[string]bool {
	set := make(map[string]bool, len(p.builtins))
	for _, builtin := range p.builtins {
		set[builtin] = true
	}
	return set
}

// splitQualifiedName splits a qualified name at every separator.
func splitQualifiedName(name string, separators []string) []string {
	parts := []string{name}
	for _, separator := range separators {
		var split []string
		for _, part := range parts {
			split = append(split, strings.Split(part, separator)...)
		}
		parts = split
	}
	return parts
}

// namespaceOf returns the namespace of a qualified name, as os//path for
// the Python module os.path.
func (p *languageProfile) namespaceOf(name string) string {
	return strings.Join(splitQualifiedName(name, p.separators), namespaceSeparator)
}

// moduleNamespace returns the namespace of an imported module name.
func (p *languageProfile) moduleNamespace(moduleName string) string {
	if p.trimModule != nil {
		moduleName = p.trimModule(moduleName)
	}
	return p.namespaceOf(moduleName)
}

// lastSegment returns the last segment of a qualified name, as path for
// os.path.
func (p *languageProfile) lastSegment(name string) string {
	parts := splitQualifiedName(name, p.separators)
	return parts[len(parts)-1]
}

// CallableArgument returns the position of the argument that names the
// callable that a function of the language calls, as 0 for call_user_func
// of PHP.
func CallableArgument(code core.LanguageCode, function string) (int, bool) {
	position, ok := profileOf(code).callables[function]
	return position, ok
}
