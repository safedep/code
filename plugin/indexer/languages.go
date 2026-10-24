package indexer

import (
	"github.com/safedep/code/core"
	"github.com/safedep/code/fileindex"
	sitter "github.com/smacker/go-tree-sitter"
)

// languageRules are the parts of the projection of one language that a
// table holds. Each language file declares its rules, and init registers
// them. The shared projection code reads the syntax of a language only from
// here. A nil function adds nothing.
type languageRules struct {
	// kinds maps the tree-sitter type of a definition of the language to its
	// kind. A function in a class becomes a method.
	kinds map[string]fileindex.DefKind
	// constructor names the method that makes an instance of a class, as
	// __init__ of Python.
	constructor string
	// memberScopes marks a language where a class binds its members for a
	// call with no receiver in a method, as Java.
	memberScopes bool
	// rawUnbound marks a language where a call of a name that the file does
	// not define keeps the name as its target, as Box for new Box<>() in
	// Java.
	rawUnbound bool
	// typedReceivers marks a language whose call graph does not type a
	// receiver that a declared type of the scope names, as Python. A call
	// on such a receiver gets the key of the name in its scope as its
	// target, as main.py//run//t//do, so the link step reads the type.
	typedReceivers bool

	// packageOf returns the package or the namespace of the file, which the
	// files of one unit share.
	packageOf func(p *projection) string
	// kindOf returns the kind of a definition when its node type alone does
	// not tell it, as a Kotlin interface, which is a class declaration.
	kindOf func(p *projection, node *sitter.Node, kind fileindex.DefKind) fileindex.DefKind
	// bases returns the classes and the interfaces that a class or an
	// interface names as its bases, as written.
	bases func(p *projection, class *sitter.Node) []string
	// fields returns the fields of a class that declare a type.
	fields func(p *projection, class *sitter.Node) []fileindex.Typed
	// abstract returns the methods that an interface declares.
	abstract func(p *projection, node *sitter.Node) []fileindex.Signature
	// decorators returns the decorators, the annotations or the attributes
	// of a definition as written, with no arguments.
	decorators func(p *projection, node *sitter.Node) []string
	// signature fills the static mark, the typed parameters and the typed
	// locals of a definition.
	signature func(p *projection, def *fileindex.Definition, node *sitter.Node)
	// exported reports that the definition is visible outside the file. The
	// default exports a top-level definition and a method with its class.
	exported func(p *projection, def *fileindex.Definition) bool

	// definitions adds the definitions that the call graph does not name, as
	// the top-level statements of C#.
	definitions func(p *projection)
	// assigned returns the name nodes that a top-level statement assigns, as
	// x in x = f(). Each such name is a variable.
	assigned func(p *projection, statement *sitter.Node) []*sitter.Node
	// exports adds the export records of the file.
	exports func(p *projection)
	// inits adds the definitions that run when a class loads, after the
	// signatures and before the walk of the tree.
	inits func(p *projection)
	// afterWalk fills the definitions that the walk of the tree adds, as the
	// typed parameters of a TypeScript function that an argument holds.
	afterWalk func(p *projection)
	// prune drops the definitions that hold no record, after the records.
	prune func(p *projection)

	// visitors read the walk of the tree, as for the references and the
	// flags of the language, in this order.
	visitors []func(p *projection) visitor
	// dynamic names the syntax that the flags pass reads.
	dynamic *dynamicSyntax
	// references names the syntax that holds a value.
	references *referenceSyntax
	// arguments names the syntax of the calls and the arguments.
	arguments *argumentSyntax
	// mainGuard names the if statement that runs its body only when the
	// file is the program.
	mainGuard *mainGuard
}

// languageRulesByCode is filled in init. The functions of the rules call
// rules, so a variable initializer would make an initialization cycle.
var languageRulesByCode = map[core.LanguageCode]*languageRules{}

func init() {
	languageRulesByCode[core.LanguageCodePython] = &pythonRules
	languageRulesByCode[core.LanguageCodeJavascript] = &javascriptRules
	languageRulesByCode[core.LanguageCodeTypescript] = &typescriptRules
	languageRulesByCode[core.LanguageCodeGo] = &goRules
	languageRulesByCode[core.LanguageCodeJava] = &javaRules
	languageRulesByCode[core.LanguageCodeKotlin] = &kotlinRules
	languageRulesByCode[core.LanguageCodeCSharp] = &csharpRules
	languageRulesByCode[core.LanguageCodePHP] = &phpRules
	languageRulesByCode[core.LanguageCodeRuby] = &rubyRules
	languageRulesByCode[core.LanguageCodeRust] = &rustRules
}

// rules returns the rules of the language of the file, or empty rules.
func (p *projection) rules() *languageRules {
	if r, ok := languageRulesByCode[p.ix.Language]; ok {
		return r
	}
	return &languageRules{}
}

// addSignatures fills the signature of each definition by the rules of the
// language.
func (p *projection) addSignatures() {
	signature := p.rules().signature
	if signature == nil {
		return
	}
	for key, def := range p.defs {
		if node := p.syntax[key]; node != nil {
			signature(p, def, node)
		}
	}
}
