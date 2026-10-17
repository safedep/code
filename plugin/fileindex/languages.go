package fileindex

import (
	"strings"

	"github.com/safedep/code/core"
	fidx "github.com/safedep/code/fileindex"
	sitter "github.com/smacker/go-tree-sitter"
)

// languageRules are the parts of the projection of one language that a
// table holds. Each language file of Kotlin, Ruby, PHP, C# and Rust
// declares its rules. A nil function adds nothing.
type languageRules struct {
	// packageOf returns the package or the namespace of the file, which the
	// files of one unit share.
	packageOf func(p *projection) string
	// kindOf returns the kind of a definition when its node type alone does
	// not tell it, as a Kotlin interface, which is a class declaration.
	kindOf func(p *projection, node *sitter.Node, kind fidx.DefKind) fidx.DefKind
	// bases returns the classes and the interfaces that a class names as
	// its bases, as written.
	bases func(p *projection, class *sitter.Node) []string
	// fields returns the fields of a class that declare a type.
	fields func(p *projection, class *sitter.Node) []fidx.Typed
	// decorators returns the annotations or the attributes of a definition
	// as written, with no arguments.
	decorators func(p *projection, node *sitter.Node) []string
	// signature fills the static mark, the typed parameters and the typed
	// locals of a definition.
	signature func(p *projection, def *fidx.Definition, node *sitter.Node)
	// definitions adds the definitions that the call graph does not name, as
	// the top-level statements of C#.
	definitions func(p *projection)
	// visitors read the walk of the tree, as for the references and the
	// flags of the language.
	visitors []func(p *projection) visitor
	// memberScopes marks a language where a class binds its members for a
	// call with no receiver in a method, as Java.
	memberScopes bool
}

// languageRulesByCode is filled in init. The functions of the rules call
// rules, so a variable initializer would make an initialization cycle.
var languageRulesByCode = map[core.LanguageCode]*languageRules{}

func init() {
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

// memberScopes reports whether a class binds its members for a call with no
// receiver.
func (p *projection) memberScopes() bool {
	return p.isJava() || p.rules().memberScopes
}

// withoutTypeArgs removes the type arguments of a name, as List for
// List<string>.
func withoutTypeArgs(name string) string {
	if i := strings.IndexByte(name, '<'); i >= 0 {
		return strings.TrimSpace(name[:i])
	}
	return name
}
