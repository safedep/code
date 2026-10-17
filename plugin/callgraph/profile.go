package callgraph

import (
	"strings"

	"github.com/safedep/code/core"
)

// languageProfile is what the call graph knows of one language. Each
// language file declares its profile, and languageProfiles lists them.
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
	entryFunction string

	// builtins are the functions and types that a file uses with no import.
	builtins []string

	// processors are the node processors of the grammar. A node type with
	// no processor here gets the processor of neutralProcessor.
	processors map[string]nodeProcessor

	// reachable are the node types of the definitions that DFS starts from,
	// because a file often exports a definition and does not call it.
	reachable map[string]bool

	// transparent are the node types whose value is the value of one child.
	transparent map[string]bool

	// importBindsRoot marks a language where an import of a dotted module
	// with no item and no alias binds its first segment, as os for import
	// os.path in Python. The full module is a node too.
	importBindsRoot bool

	// defaultExport is the item that names the module itself, as default
	// in import { default as y } from 'x'.
	defaultExport string

	// trimModule removes the part of an imported module name that names no
	// namespace. It can be nil.
	trimModule func(string) string
}

// languageProfiles are the languages of the call graph, in a fixed order.
var languageProfiles = []*languageProfile{
	&pythonProfile,
	&javaProfile,
	&goProfile,
	&javascriptProfile,
	&typescriptProfile,
	&csharpProfile,
	&rustProfile,
	&phpProfile,
	&rubyProfile,
}

var profilesByCode = indexProfiles(languageProfiles)

func indexProfiles(profiles []*languageProfile) map[core.LanguageCode]*languageProfile {
	byCode := make(map[core.LanguageCode]*languageProfile, len(profiles))
	for _, profile := range profiles {
		byCode[profile.code] = profile
	}
	return byCode
}

func supportedLanguages() []core.LanguageCode {
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
