package indexer

import (
	"github.com/safedep/code/fileindex"
	sitter "github.com/smacker/go-tree-sitter"
)

// span identifies a tree node across lookups. The tree gives a new
// *sitter.Node on each lookup, so a pointer does not identify a node.
type span struct {
	start, end uint32
	kind       string
}

func spanOf(node *sitter.Node) span {
	return span{node.StartByte(), node.EndByte(), node.Type()}
}

// functionValues are the node types of a function that an expression holds.
var functionValues = map[string]bool{
	"func_literal":        true,
	"function_expression": true,
	"function":            true,
	"arrow_function":      true,
	"generator_function":  true,
}

// addSyntax records the tree node of a definition.
func (p *projection) addSyntax(key string, node *sitter.Node) {
	p.syntax[key] = node
	if node != nil {
		p.spans[spanOf(node)] = key
	}
}

// defined reports whether a tree node already is a definition.
func (p *projection) defined(node *sitter.Node) bool {
	_, ok := p.spans[spanOf(node)]
	return ok
}

// addBody adds a definition that the call graph does not know and that
// takes the calls of the node.
func (p *projection) addBody(key string, kind fileindex.DefKind, node *sitter.Node) {
	def := &fileindex.Definition{
		Key:       key,
		Kind:      kind,
		Name:      fileindex.Last(key),
		StartLine: lineOf(node),
		EndLine:   int(node.EndPoint().Row) + 1,
	}
	p.defs[key] = def
	p.addSyntax(key, node)
	p.bodies[spanOf(node)] = key
	p.fillDefinition(def)
}

// innermost returns the key of the innermost definition that holds the
// byte, or the module key.
func (p *projection) innermost(at uint32) string {
	var inner *sitter.Node
	key := p.cg.FileName
	for k, node := range p.syntax {
		if node == nil || at < node.StartByte() || at >= node.EndByte() {
			continue
		}
		if inner == nil || node.EndByte()-node.StartByte() < inner.EndByte()-inner.StartByte() {
			inner, key = node, k
		}
	}
	return key
}

// fields returns the fields of a class that declare a type name, by the
// rules of the language.
func (p *projection) fields(class *sitter.Node) []fileindex.Typed {
	if rule := p.rules().fields; rule != nil {
		return rule(p, class)
	}
	return nil
}

// bases returns the bases of a class or an interface as written, by the
// rules of the language.
func (p *projection) bases(node *sitter.Node) []string {
	if rule := p.rules().bases; rule != nil {
		return rule(p, node)
	}
	return nil
}

// abstract returns the methods that an interface declares, by the rules of
// the language.
func (p *projection) abstract(node *sitter.Node) []fileindex.Signature {
	if rule := p.rules().abstract; rule != nil && node != nil {
		return rule(p, node)
	}
	return nil
}
