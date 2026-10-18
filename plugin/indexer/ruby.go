package indexer

import (
	"slices"
	"strings"

	"github.com/safedep/code/core/ast"
	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/pkg/ts"
	sitter "github.com/smacker/go-tree-sitter"
)

var (
	// rubySends call the method that their first argument names.
	rubySends = []string{"send", "public_send", "__send__", "try"}
	// rubyMixins give the methods of a module to the class.
	rubyMixins = []string{"include", "prepend", "extend"}
	// rubyMethodObjects return the method that their first argument names.
	rubyMethodObjects = []string{"method", "instance_method", "public_method", "public_instance_method"}
	// rubyDeclarations name methods in a class body and call none of them.
	rubyDeclarations = []string{"attr", "attr_reader", "attr_writer", "attr_accessor", "private", "public", "protected", "module_function", "private_class_method", "public_class_method", "private_constant", "public_constant", "ruby2_keywords", "include", "prepend", "extend", "require", "require_relative"}
	// rubyMonkeyPatches define or change methods at run time.
	rubyMonkeyPatches = []string{"instance_eval", "class_eval", "module_eval", "instance_exec", "class_exec", "define_method", "define_singleton_method"}
)

var rubyRules = languageRules{
	bases:        rubyBases,
	signature:    rubySignature,
	visitors:     []func(p *projection) visitor{rubyVisitor},
	memberScopes: true,
}

// rubyBases returns the superclass of a class and the modules that its
// body includes, prepends or extends, as written.
func rubyBases(p *projection, class *sitter.Node) []string {
	var bases []string
	if superclass := class.ChildByFieldName("superclass"); superclass != nil && superclass.NamedChildCount() > 0 {
		bases = append(bases, p.text(superclass.NamedChild(0)))
	}
	body := class.ChildByFieldName("body")
	if body == nil {
		return bases
	}
	for _, statement := range ts.NamedChildren(body) {
		if statement.Type() != "call" || statement.ChildByFieldName("receiver") != nil || !slices.Contains(rubyMixins, p.text(statement.ChildByFieldName("method"))) {
			continue
		}
		for _, arg := range ts.NamedChildren(statement.ChildByFieldName("arguments")) {
			if arg.Type() == "constant" || arg.Type() == "scope_resolution" {
				bases = append(bases, p.text(arg))
			}
		}
	}
	return bases
}

// rubySignature marks a method of self, as def self.build, static. Its key
// ends in self.build, beside the build of an instance, and its name is the
// name that a call writes.
func rubySignature(_ *projection, def *fileindex.Definition, node *sitter.Node) {
	def.Static = node.Type() == "singleton_method"
	if def.Static {
		def.Name = strings.TrimPrefix(def.Name, "self.")
	}
}

// rubyVisitor reads the calls that the call graph keeps as plain calls: a
// require_relative, which loads a file of the project, a require of a
// computed path, a send of a symbol, which calls the method of the symbol,
// method(:name), which is a reference to the method, an alias, which calls
// the old method by the new name, a method that a block defines on a class
// that the source does not name, a symbol argument of a call in a class
// body, as before_action :check, which a framework calls, and the calls
// that change methods at run time or evaluate code.
func rubyVisitor(p *projection) visitor {
	return func(node *sitter.Node, kind string) {
		switch kind {
		case "alias":
			p.addRubyReference(node, nil, symbolName(p.text(node.ChildByFieldName("alias"))))
			return
		case "method", "singleton_method":
			if block := enclosingBlock(node); block != nil {
				p.flag(fileindex.FlagMonkeyPatch, block)
			}
			return
		}
		if kind != "call" {
			return
		}
		method := p.text(node.ChildByFieldName("method"))
		receiver := node.ChildByFieldName("receiver")
		args := ts.NamedChildren(node.ChildByFieldName("arguments"))
		var first *sitter.Node
		if len(args) > 0 {
			first = args[0]
		}
		switch {
		case method == "require_relative" || method == "require" || method == "load":
			p.addRubyRequire(node, method, first)
		case slices.Contains(rubySends, method) && first != nil && first.Type() == "simple_symbol":
			p.addRubyUnresolved(node, symbolName(p.text(first)))
		case slices.Contains(rubyMethodObjects, method) && first != nil && first.Type() == "simple_symbol":
			p.addRubyReference(node, receiver, symbolName(p.text(first)))
		case method == "eval":
			p.flag(fileindex.FlagEval, node)
		case method == "const_get":
			p.flag(fileindex.FlagReflection, node)
		case slices.Contains(rubyMonkeyPatches, method):
			p.flag(fileindex.FlagMonkeyPatch, node)
		}
		if receiver != nil || slices.Contains(rubyDeclarations, method) {
			return
		}
		if body := node.Parent(); body == nil || body.Type() != "body_statement" || body.Parent() == nil || (body.Parent().Type() != "class" && body.Parent().Type() != "module") {
			return
		}
		if method == "alias_method" {
			if len(args) == 2 && args[1].Type() == "simple_symbol" {
				p.addRubyReference(args[1], nil, symbolName(p.text(args[1])))
			}
			return
		}
		for _, arg := range args {
			if arg.Type() == "simple_symbol" {
				p.addRubyReference(arg, nil, symbolName(p.text(arg)))
			}
		}
	}
}

// enclosingBlock returns the block that holds a method definition, as in
// Struct.new(:x) do ... end or Class.new do ... end. The block defines the
// method on a class that the source does not name.
func enclosingBlock(node *sitter.Node) *sitter.Node {
	for n := node.Parent(); n != nil; n = n.Parent() {
		switch n.Type() {
		case "class", "module", "singleton_class", "program":
			return nil
		case "block", "do_block", "lambda":
			return n
		}
	}
	return nil
}

// addRubyRequire adds the import of a require_relative, as ./loader for
// require_relative 'loader', and the computed import of a require whose
// path the source does not write.
func (p *projection) addRubyRequire(node *sitter.Node, method string, path *sitter.Node) {
	if path != nil && path.Type() == "string" && path.NamedChildCount() == 1 && path.NamedChild(0).Type() == "string_content" {
		if method != "require_relative" {
			return
		}
		spec := p.text(path.NamedChild(0))
		if !strings.HasPrefix(spec, ".") {
			spec = "./" + spec
		}
		p.ix.Imports = append(p.ix.Imports, fileindex.Import{Line: lineOf(node), Module: spec, Kind: fileindex.ImportStatic})
		return
	}
	if path != nil {
		p.ix.Imports = append(p.ix.Imports, fileindex.Import{Line: lineOf(node), Kind: fileindex.ImportDynamicComputed})
		p.flag(fileindex.FlagComputedImport, node)
	}
}

// addRubyReference adds a reference to the method of the name that the
// scope of the node binds, as a method of its class. A method that the
// file does not define, as one of a superclass in another file or of the
// receiver of method(:name), stays a record of a call of the name.
func (p *projection) addRubyReference(node, receiver *sitter.Node, name string) {
	caller := p.innermost(node.StartByte())
	var targets []string
	if receiver == nil {
		targets = p.rebind(caller, name)
	}
	if len(targets) == 0 {
		p.addRubyUnresolved(node, name)
		return
	}
	p.addReference(node, caller, targets)
}

// addRubyUnresolved adds the record of a call of the method of the name on
// a receiver of no known class.
func (p *projection) addRubyUnresolved(node *sitter.Node, name string) {
	site := ast.GetNodePosition(node)
	p.ix.Unresolved = append(p.ix.Unresolved, fileindex.Unresolved{
		Caller:   p.caller(p.innermost(node.StartByte()), site),
		Line:     int(site.StartLine),
		Column:   int(site.StartColumn),
		Text:     clipText(p.text(node)),
		Receiver: p.text(node.ChildByFieldName("receiver")),
		Method:   name,
		Reason:   fileindex.UnresolvedUnknownReceiver,
	})
}

func symbolName(symbol string) string {
	return strings.TrimPrefix(symbol, ":")
}
