// Package fileindex holds the index of one source file: what the link step
// of a project graph needs from the file. An index holds no tree-sitter node
// and no source text past a short call text, so a store can keep it and the
// link step can run with no parse. The package imports no tree-sitter
// package, so it builds with no CGO.
package fileindex

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"

	"github.com/safedep/code/lang/grammar"
	"github.com/safedep/code/langcode"
)

// Version is the version of the index records and of the rules that fill
// them. Bump it when a field, a kind, an order or a fill rule changes. A
// store keys an index by Version, so the bump makes every older index miss.
const Version = "7"

type ImportKind string

const (
	ImportStatic          ImportKind = "static"
	ImportDynamicString   ImportKind = "dynamic-string"
	ImportDynamicComputed ImportKind = "dynamic-computed"
	// ImportTypeOnly is a TypeScript import type. It binds no value, so the
	// link step makes no edge for it.
	ImportTypeOnly ImportKind = "type-only"
)

type DefKind string

const (
	DefModule   DefKind = "module"
	DefFunction DefKind = "function"
	DefMethod   DefKind = "method"
	// DefClass is a class, or a named type of Go that is not an interface.
	DefClass       DefKind = "class"
	DefConstructor DefKind = "constructor"
	// DefVariable is a name that the top level of a module assigns, as
	// parser in parser = Parser(). Another file can import it.
	DefVariable DefKind = "variable"
	// DefInterface is a TypeScript or Go interface. It has no code, but a
	// declared type can name it and a class can implement it.
	DefInterface DefKind = "interface"
)

type CallKind string

const (
	CallCall      CallKind = "call"
	CallNew       CallKind = "new"
	CallDecorator CallKind = "decorator"
	CallImport    CallKind = "import"
)

type ArgShape string

const (
	ArgString   ArgShape = "string"
	ArgTemplate ArgShape = "template"
	ArgName     ArgShape = "name"
	ArgMember   ArgShape = "member"
	ArgLiteral  ArgShape = "literal"
	ArgComputed ArgShape = "computed"
	ArgFunction ArgShape = "function"
)

type UnresolvedReason string

const (
	UnresolvedComputedName    UnresolvedReason = "computed-name"
	UnresolvedDynamicImport   UnresolvedReason = "dynamic-import"
	UnresolvedUnknownReceiver UnresolvedReason = "unknown-receiver"
	UnresolvedNoDefinition    UnresolvedReason = "no-definition"
	UnresolvedAmbiguous       UnresolvedReason = "ambiguous"
	UnresolvedLimit           UnresolvedReason = "limit"
)

type FlagKind string

const (
	FlagEval              FlagKind = "eval"
	FlagReflection        FlagKind = "reflection"
	FlagComputedImport    FlagKind = "computed-import"
	FlagComputedAttribute FlagKind = "computed-attribute"
	FlagMonkeyPatch       FlagKind = "monkey-patch"
	FlagDecoratorWrap     FlagKind = "decorator-wrap"
	FlagParseError        FlagKind = "parse-error"
	// FlagInjection marks a value that a container sets, as a Java field
	// with @Autowired. The declared type of the value narrows a call on it.
	FlagInjection FlagKind = "injection"
)

// TruncationKind names a limit that the index of a file hit. The kinds of
// plugin/callgraph keep their names here.
type TruncationKind string

const (
	TruncationNodes           TruncationKind = "nodes"
	TruncationAssignmentNodes TruncationKind = "assignment-nodes"
	TruncationDepth           TruncationKind = "depth"
	TruncationMemberDepth     TruncationKind = "member-depth"
	TruncationAttributeDepth  TruncationKind = "attribute-depth"
	TruncationPolymorphic     TruncationKind = "polymorphic"
	TruncationDFSResults      TruncationKind = "dfs-results"
	TruncationDeadline        TruncationKind = "deadline"
)

// Index is what the link step needs from one file. Path is relative to the
// project root, with "/". ContentHash is the HashContent of the bytes. All
// lines and columns are 1-based. Sort puts every slice in its defined order,
// so one file gives one JSON form. Package is the package clause of a Go
// or a Java file. The files of one package share their top-level names.
// Unit names the package of a file of dependency code, as
// npm:lodash@4.17.21, and is empty for a file of the project.
type Index struct {
	Version     string        `json:"version"`
	Path        string        `json:"path"`
	Language    langcode.Code `json:"language"`
	ContentHash string        `json:"content_hash"`
	Test        bool          `json:"test,omitempty"`
	Package     string        `json:"package,omitempty"`
	Unit        string        `json:"unit,omitempty"`

	Imports     []Import     `json:"imports,omitempty"`
	Exports     []Export     `json:"exports,omitempty"`
	Definitions []Definition `json:"definitions,omitempty"`
	Classes     []Class      `json:"classes,omitempty"`
	Calls       []Call       `json:"calls,omitempty"`
	References  []Reference  `json:"references,omitempty"`
	Unresolved  []Unresolved `json:"unresolved,omitempty"`
	Flags       []Flag       `json:"flags,omitempty"`
	Truncated   []Truncation `json:"truncated,omitempty"`
}

// Import is one import of the file. Module is the module as written, as
// ./utils or .foo, and the link step resolves it. Namespace is the per-file
// "//" form of the import, which loses the leading dots of a relative module.
type Import struct {
	Line      int        `json:"line"`
	Module    string     `json:"module"`
	Item      string     `json:"item,omitempty"`
	Alias     string     `json:"alias,omitempty"`
	Wildcard  bool       `json:"wildcard,omitempty"`
	Kind      ImportKind `json:"kind"`
	Namespace string     `json:"namespace"`
}

// Export is one export of the file. Name is "default" for a default export.
// Target is a definition key of the file, and is empty for a re-export. A
// re-export names its module in From and its item in Item, "*" for all, and
// "" for the whole module. An export with no Target and no From gives on a
// name that the file imports, and Item holds that local name when it is not
// Name. The exports of a Python file are the names of its __all__.
type Export struct {
	Name   string `json:"name"`
	Target string `json:"target"`
	From   string `json:"from,omitempty"`
	Item   string `json:"item,omitempty"`
	Line   int    `json:"line"`
}

// Definition is one module, function, method, class, constructor, module
// variable or interface. Key is its symbol key, as src/a.py//Parser//parse.
// Parent is the class key of a method. A JavaScript function that an object
// property or a property assignment holds is a method with no parent.
// Decorators are as written, in source order, and hold the annotations of
// a Java definition. Values are the per-file targets that a variable
// holds, as the targets of a Call. Params are the parameters that declare
// the type of a class, in source order. A Go method has its receiver
// first. Locals are the local variables of a Go or Java function whose
// type the source declares or constructs.
//
// Arity, Variadic and Returns hold the signature of a Go function or
// method: the count of its parameters, a last parameter of the form ...T,
// and the type name of its first result. Static marks a static method of
// Java.
type Definition struct {
	Key        string   `json:"key"`
	Kind       DefKind  `json:"kind"`
	Name       string   `json:"name"`
	StartLine  int      `json:"start_line"`
	EndLine    int      `json:"end_line"`
	Exported   bool     `json:"exported"`
	Parent     string   `json:"parent,omitempty"`
	Decorators []string `json:"decorators,omitempty"`
	Values     []string `json:"values,omitempty"`
	Params     []Typed  `json:"params,omitempty"`
	Locals     []Typed  `json:"locals,omitempty"`
	Arity      int      `json:"arity,omitempty"`
	Variadic   bool     `json:"variadic,omitempty"`
	Returns    string   `json:"returns,omitempty"`
	Static     bool     `json:"static,omitempty"`
	Flags      []Flag   `json:"flags,omitempty"`
}

// Class is one class or interface of the file. Bases are as written, in
// source order, and the link step resolves them. Methods are definition
// keys. Fields are the fields that declare the type of a class, in source
// order. Abstract are the methods that a Go interface declares, in source
// order. They have no definition, and a type implements the interface when
// its methods hold them.
type Class struct {
	Key      string      `json:"key"`
	Bases    []string    `json:"bases,omitempty"`
	Methods  []string    `json:"methods,omitempty"`
	Fields   []Typed     `json:"fields,omitempty"`
	Abstract []Signature `json:"abstract,omitempty"`
}

// Signature is a method that an interface declares: its name, the count of
// its parameters and a last parameter of the form ...T.
type Signature struct {
	Name     string `json:"name"`
	Arity    int    `json:"arity"`
	Variadic bool   `json:"variadic,omitempty"`
}

// Typed is a parameter, a field or a local variable with the type that it
// declares, as parser in run(parser: Parser). Type is the type name as
// written, with no type arguments and no pointer, as Parser or ns.Parser.
// Result marks a name that holds the first result of a call. Type then
// names the function, as NewParser in p := NewParser().
type Typed struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Result bool   `json:"result,omitempty"`
}

// Call is one call site. Caller is the definition key that holds the site.
// A JavaScript static field or static block runs when its class is defined,
// so its calls have the caller of the class definition.
// Callee is the callee as the call writes it. Targets are the per-file
// resolutions of the callee: local keys, import namespaces or bare names. A
// call with no target keeps a bare name in Callee. Receiver is the object of
// a member call and Method the called name, so the reach query can match an
// unresolved record of the same method. Args are in source order.
type Call struct {
	Caller   string   `json:"caller"`
	Line     int      `json:"line"`
	Column   int      `json:"column"`
	Callee   string   `json:"callee"`
	Targets  []string `json:"targets,omitempty"`
	Receiver string   `json:"receiver,omitempty"`
	Method   string   `json:"method,omitempty"`
	Kind     CallKind `json:"kind"`
	Args     []Arg    `json:"args,omitempty"`
}

// Arg is one positional argument of a call, as the framework root rules
// read it. A Python keyword argument has no position, so it is no Arg.
// Targets are the per-file targets of a name or a member, as the targets of
// a Call, or the key of a function that the index defines.
type Arg struct {
	Shape   ArgShape `json:"shape"`
	Targets []string `json:"targets,omitempty"`
}

// Reference is a function, a method, a class or an import that the file
// uses as a value and does not call, as yaml.load in run(yaml.load). The
// code that gets the value can call it, so the link step counts the target
// as possibly called. Targets are per-file resolutions, as in a Call.
type Reference struct {
	Caller  string   `json:"caller"`
	Line    int      `json:"line"`
	Column  int      `json:"column"`
	Text    string   `json:"text"`
	Targets []string `json:"targets"`
}

// Unresolved is a call site whose callee the index does not know in full.
// Text is the callee expression, at most 120 bytes. Method is empty when the
// call computes the name, and then the call can reach any target.
type Unresolved struct {
	Caller   string           `json:"caller"`
	Line     int              `json:"line"`
	Column   int              `json:"column"`
	Text     string           `json:"text"`
	Receiver string           `json:"receiver,omitempty"`
	Method   string           `json:"method,omitempty"`
	Reason   UnresolvedReason `json:"reason"`
}

// Flag marks a dynamic feature at a line of the file.
type Flag struct {
	Kind FlagKind `json:"kind"`
	Line int      `json:"line"`
}

// Truncation records the first place where the index of the file hit one
// kind of limit. Line is 0 when no node marks the place.
type Truncation struct {
	Kind      TruncationKind `json:"kind"`
	Namespace string         `json:"namespace,omitempty"`
	Line      int            `json:"line"`
}

// FileStatus is what the index step and the link step know about a file.
type FileStatus string

const (
	// FileIndexed is a file that the index step parsed and indexed.
	FileIndexed FileStatus = "indexed"

	// FileCached is a file whose index came from a store with no parse.
	FileCached FileStatus = "cached"

	// FileMissing is a file that the index step did not index, because its
	// deadline came before it reached the file or completed the index.
	FileMissing FileStatus = "missing"

	// FileFailed is a file that the index step could not read, parse or
	// index.
	FileFailed FileStatus = "failed"

	// FileTruncated is a file whose index hit a limit or holds a parse
	// error, so the index is not complete.
	FileTruncated FileStatus = "truncated"
)

// Complete reports whether the file has an index that hit no limit.
func (s FileStatus) Complete() bool {
	return s == FileIndexed || s == FileCached
}

// Key identifies the stored index of a file. The index depends on the path,
// because its definition keys and its test mark hold the path. Two files with
// the same bytes at two paths have two keys.
type Key struct {
	Path           string
	ContentHash    string
	IndexVersion   string
	GrammarVersion string
	Language       langcode.Code
}

// NewKey returns the key of the current Version and grammar for a file.
func NewKey(path, contentHash string, language langcode.Code) Key {
	return Key{
		Path:           path,
		ContentHash:    contentHash,
		IndexVersion:   Version,
		GrammarVersion: grammar.Version(language),
		Language:       language,
	}
}

// HashContent returns the hex sha256 of the bytes of a file.
func HashContent(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Key returns the store key of the index, with the grammar of this build.
func (ix *Index) Key() Key {
	k := NewKey(ix.Path, ix.ContentHash, ix.Language)
	k.IndexVersion = ix.Version
	return k
}

// ExternalCalls returns the calls that leave the file. Such a call has a
// target under an import namespace, or a bare name that no definition of
// the file binds. A call with no target counts its callee. The calls keep
// the order of Calls. vet reads them in plan step 24.
func (ix *Index) ExternalCalls() []Call {
	namespaces := make([]string, 0, len(ix.Imports))
	for _, imp := range ix.Imports {
		if imp.Namespace != "" {
			namespaces = append(namespaces, imp.Namespace)
		}
	}

	defined := make(map[string]bool, len(ix.Definitions))
	for _, def := range ix.Definitions {
		defined[def.Name] = true
	}

	external := func(target string) bool {
		if !strings.Contains(target, Separator) {
			return !defined[target]
		}
		return slices.ContainsFunc(namespaces, func(ns string) bool {
			return target == ns || strings.HasPrefix(target, ns+Separator)
		})
	}

	var calls []Call
	for _, call := range ix.Calls {
		targets := call.Targets
		if len(targets) == 0 {
			targets = []string{call.Callee}
		}
		if slices.ContainsFunc(targets, external) {
			calls = append(calls, call)
		}
	}
	return calls
}

// DynamicSites returns the flags of the file and of its definitions, by line.
func (ix *Index) DynamicSites() []Flag {
	flags := slices.Clone(ix.Flags)
	for _, def := range ix.Definitions {
		flags = append(flags, def.Flags...)
	}
	slices.SortFunc(flags, CompareFlags)
	return slices.Compact(flags)
}

// Sort puts every slice of the index in its defined order. Decorators and
// class bases keep the source order, because the order has a meaning.
func (ix *Index) Sort() {
	slices.SortFunc(ix.Imports, func(a, b Import) int {
		return cmp.Or(
			cmp.Compare(a.Line, b.Line),
			cmp.Compare(a.Module, b.Module),
			cmp.Compare(a.Item, b.Item),
			cmp.Compare(a.Alias, b.Alias),
			compareBool(a.Wildcard, b.Wildcard),
			cmp.Compare(a.Kind, b.Kind),
			cmp.Compare(a.Namespace, b.Namespace),
		)
	})
	slices.SortFunc(ix.Exports, func(a, b Export) int {
		return cmp.Or(
			cmp.Compare(a.Line, b.Line),
			cmp.Compare(a.Name, b.Name),
			cmp.Compare(a.Target, b.Target),
			cmp.Compare(a.From, b.From),
			cmp.Compare(a.Item, b.Item),
		)
	})
	for i := range ix.Definitions {
		slices.SortFunc(ix.Definitions[i].Flags, CompareFlags)
	}
	slices.SortFunc(ix.Definitions, func(a, b Definition) int {
		return cmp.Or(
			cmp.Compare(a.StartLine, b.StartLine),
			cmp.Compare(a.Key, b.Key),
		)
	})
	for i := range ix.Classes {
		slices.Sort(ix.Classes[i].Methods)
	}
	slices.SortFunc(ix.Classes, func(a, b Class) int {
		return cmp.Compare(a.Key, b.Key)
	})
	for i := range ix.Calls {
		slices.Sort(ix.Calls[i].Targets)
	}
	slices.SortFunc(ix.Calls, func(a, b Call) int {
		return cmp.Or(
			cmp.Compare(a.Line, b.Line),
			cmp.Compare(a.Column, b.Column),
			cmp.Compare(a.Caller, b.Caller),
			cmp.Compare(a.Callee, b.Callee),
			cmp.Compare(a.Receiver, b.Receiver),
			cmp.Compare(a.Method, b.Method),
			cmp.Compare(a.Kind, b.Kind),
		)
	})
	for i := range ix.References {
		slices.Sort(ix.References[i].Targets)
	}
	slices.SortFunc(ix.References, func(a, b Reference) int {
		return cmp.Or(
			cmp.Compare(a.Line, b.Line),
			cmp.Compare(a.Column, b.Column),
			cmp.Compare(a.Caller, b.Caller),
			cmp.Compare(a.Text, b.Text),
		)
	})
	slices.SortFunc(ix.Unresolved, func(a, b Unresolved) int {
		return cmp.Or(
			cmp.Compare(a.Line, b.Line),
			cmp.Compare(a.Column, b.Column),
			cmp.Compare(a.Caller, b.Caller),
			cmp.Compare(a.Text, b.Text),
			cmp.Compare(a.Receiver, b.Receiver),
			cmp.Compare(a.Method, b.Method),
			cmp.Compare(a.Reason, b.Reason),
		)
	})
	slices.SortFunc(ix.Flags, CompareFlags)
	slices.SortFunc(ix.Truncated, func(a, b Truncation) int {
		return cmp.Or(
			cmp.Compare(a.Kind, b.Kind),
			cmp.Compare(a.Line, b.Line),
			cmp.Compare(a.Namespace, b.Namespace),
		)
	})
}

// CompareFlags orders flags by line, then by kind.
func CompareFlags(a, b Flag) int {
	return cmp.Or(cmp.Compare(a.Line, b.Line), cmp.Compare(a.Kind, b.Kind))
}

func compareBool(a, b bool) int {
	switch {
	case a == b:
		return 0
	case a:
		return 1
	default:
		return -1
	}
}
