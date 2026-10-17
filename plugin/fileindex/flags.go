package fileindex

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/safedep/code/core"
	fidx "github.com/safedep/code/fileindex"
	"github.com/safedep/code/pkg/ts"
	sitter "github.com/smacker/go-tree-sitter"
)

// dynamicSyntax names the node types and the functions that the flags pass
// of one language reads.
type dynamicSyntax struct {
	call, assignment, subscript string
	subscriptIndex              string
	members                     map[string]string
	wrappers                    map[string]bool
	eval                        map[string]bool
	importers                   map[string]bool
	reflection                  func(callee string) bool

	// reflectionName is the argument that names the member of a reflection
	// call, as attr in getattr(obj, attr). A string there makes no flag.
	// It is -1 for a language whose reflection calls name no member.
	reflectionName int
	// moduleGetattr names the function of a module that runs for a name
	// that the module does not define, as __getattr__ of Python.
	moduleGetattr string
}

var ecmascriptSyntax = dynamicSyntax{
	call:           "call_expression",
	assignment:     "assignment_expression",
	subscript:      "subscript_expression",
	subscriptIndex: "index",
	members:        map[string]string{"member_expression": "object", "subscript_expression": "object"},
	wrappers:       map[string]bool{"parenthesized_expression": true, "as_expression": true, "non_null_expression": true},
	eval:           map[string]bool{"eval": true, "Function": true},
	importers:      map[string]bool{"require": true},
	reflection:     func(callee string) bool { return strings.HasPrefix(callee, "Reflect.") },
	reflectionName: -1,
}

var dynamicSyntaxes = map[core.LanguageCode]dynamicSyntax{
	core.LanguageCodeJavascript: ecmascriptSyntax,
	core.LanguageCodeTypescript: ecmascriptSyntax,
	core.LanguageCodePython: {
		call:           "call",
		assignment:     "assignment",
		subscript:      "subscript",
		subscriptIndex: "subscript",
		members:        map[string]string{"attribute": "object", "subscript": "value"},
		wrappers:       map[string]bool{"parenthesized_expression": true},
		eval:           map[string]bool{"eval": true, "exec": true},
		importers:      map[string]bool{"importlib.import_module": true, "import_module": true, "__import__": true},
		reflection:     func(callee string) bool { return slices.Contains([]string{"getattr", "setattr", "delattr"}, callee) },
		reflectionName: 1,
		moduleGetattr:  "__getattr__",
	},
}

// flagModuleGetattr marks a module that defines __getattr__. A name of the
// module that no definition binds runs it, and the index does not know
// the value that it returns. The flag lands on the module, so the link
// step reads it where it looks a name up.
func (p *projection) flagModuleGetattr() {
	name := dynamicSyntaxes[p.ix.Language].moduleGetattr
	if name == "" {
		return
	}
	def, ok := p.defs[p.local(name)]
	if !ok || def.Kind != fidx.DefFunction {
		return
	}
	flag := fidx.Flag{Kind: fidx.FlagModuleGetattr, Line: def.StartLine}
	p.ix.Flags = append(p.ix.Flags, flag)
	module := p.defs[p.cg.FileName]
	module.Flags = append(module.Flags, flag)
}

// visitor reads one node of a walk of the tree, with the type of the node.
type visitor func(node *sitter.Node, kind string)

// walkTree walks the tree once for the property functions, the flags and
// the references of the file. A walk costs about as much as the parse, so
// the passes share it.
func (p *projection) walkTree(ctx context.Context) {
	root := p.tree.Tree().RootNode()
	p.addParseErrors(ctx, root)

	var visitors []visitor
	candidates := []visitor{p.bodyVisitor(), p.flagVisitor(), p.referenceVisitor(), p.javaFlagVisitor(), p.javaReferenceVisitor()}
	for _, v := range p.rules().visitors {
		candidates = append(candidates, v(p))
	}
	for _, v := range candidates {
		if v != nil {
			visitors = append(visitors, v)
		}
	}
	if len(visitors) == 0 {
		return
	}
	err := ts.Walk(ctx, root, ts.WalkOptions{}, func(node *sitter.Node) bool {
		kind := node.Type()
		for _, v := range visitors {
			v(node, kind)
		}
		return true
	})
	p.truncateWalk(err)
}

// flagVisitor marks the dynamic features of the file. A flag lands on the
// file and on the innermost definition that holds it. The pass also adds a
// dynamic import that the import resolvers do not see, and marks a require
// in a function as dynamic.
func (p *projection) flagVisitor() visitor {
	syntax, ok := dynamicSyntaxes[p.ix.Language]
	if !ok {
		return nil
	}
	return func(node *sitter.Node, kind string) {
		switch kind {
		case syntax.call:
			p.flagCall(syntax, node)
		case "new_expression":
			if constructor := node.ChildByFieldName("constructor"); syntax.eval[p.text(constructor)] {
				p.flag(fidx.FlagEval, node)
			}
		case syntax.assignment:
			p.flagMonkeyPatch(syntax, node.ChildByFieldName("left"))
		case "decorator":
			if node.NamedChildCount() > 0 && node.NamedChild(0).Type() != "identifier" {
				p.flagAt(fidx.FlagDecoratorWrap, decorated(node).StartByte(), lineOf(node))
			}
		}
	}
}

// decorated returns the node that a decorator wraps. A TypeScript class
// body holds the decorators of a member before the member.
func decorated(decorator *sitter.Node) *sitter.Node {
	parent := decorator.Parent()
	switch {
	case parent == nil:
		return decorator
	case parent.Type() == "decorated_definition":
		if definition := parent.ChildByFieldName("definition"); definition != nil {
			return definition
		}
	case parent.Type() == "class_body":
		for sibling := decorator.NextNamedSibling(); sibling != nil; sibling = sibling.NextNamedSibling() {
			if sibling.Type() != "decorator" {
				return sibling
			}
		}
	}
	return parent
}

// addParseErrors flags each ERROR and MISSING node of the tree.
func (p *projection) addParseErrors(ctx context.Context, root *sitter.Node) {
	if !root.HasError() {
		return
	}
	diagnostics, err := ts.Diagnose(ctx, root, 0)
	for _, position := range slices.Concat(diagnostics.Errors, diagnostics.Missing) {
		p.flagAt(fidx.FlagParseError, position.StartByte, int(position.StartLine))
	}
	p.truncateWalk(err)
}

// truncateWalk records the limit that stopped a walk of the tree.
func (p *projection) truncateWalk(err error) {
	switch {
	case err == nil:
	case errors.Is(err, ts.ErrNodeLimit):
		p.truncate(fidx.TruncationNodes, p.cg.FileName, 0)
	default:
		p.truncate(fidx.TruncationDeadline, p.cg.FileName, 0)
	}
}

func (p *projection) flagCall(syntax dynamicSyntax, call *sitter.Node) {
	callee := call.ChildByFieldName("function")
	if callee == nil {
		return
	}
	var arguments []*sitter.Node
	if node := call.ChildByFieldName("arguments"); node != nil {
		arguments = ts.NamedChildren(node)
	}
	name := p.text(callee)

	switch {
	case syntax.eval[name]:
		p.flag(fidx.FlagEval, call)
	case syntax.reflection(name):
		if i := syntax.reflectionName; i < 0 || i >= len(arguments) || !p.isString(arguments[i]) {
			p.flag(fidx.FlagReflection, call)
		}
	case callee.Type() == "import":
		p.dynamicImport(call, arguments, true)
	case syntax.importers[name]:
		p.dynamicImport(call, arguments, p.ix.Language == core.LanguageCodePython || !topLevel(call))
	case callee.Type() == syntax.subscript:
		// A Python subscript is a lookup in a dict, so a string index
		// computes the callee too.
		if p.ix.Language == core.LanguageCodePython || !p.isString(callee.ChildByFieldName(syntax.subscriptIndex)) {
			p.flag(fidx.FlagComputedAttribute, call)
		}
	}
}

// dynamicImport records an import that a call makes. A string names the
// module of a dynamic-string import. Any other argument makes a
// dynamic-computed import with the static prefix of the argument as its
// module, and a flag. A static import keeps its kind.
func (p *projection) dynamicImport(call *sitter.Node, arguments []*sitter.Node, dynamic bool) {
	var argument *sitter.Node
	if len(arguments) > 0 {
		argument = arguments[0]
	}
	module, whole := p.stringPrefix(argument)
	line := lineOf(call)
	require := p.ix.Language.IsECMAScript() && p.text(call.ChildByFieldName("function")) == requireFunction
	if !whole {
		p.flag(fidx.FlagComputedImport, call)
		p.ix.Imports = append(p.ix.Imports, fidx.Import{Line: line, Module: module, Kind: fidx.ImportDynamicComputed, Require: require})
		return
	}
	if !dynamic {
		return
	}
	for i := range p.ix.Imports {
		if imp := &p.ix.Imports[i]; imp.Line == lineOf(argument) && imp.Module == module {
			imp.Kind, imp.Require = fidx.ImportDynamicString, require
			return
		}
	}
	p.ix.Imports = append(p.ix.Imports, fidx.Import{
		Line:      line,
		Module:    module,
		Kind:      fidx.ImportDynamicString,
		Namespace: fidx.Namespace(p.ix.Language, module),
		Require:   require,
	})
}

// topLevel reports that a call is a statement of the program or the value
// of a top-level declaration, as const x = require('x').
func topLevel(call *sitter.Node) bool {
	for node := call.Parent(); node != nil; node = node.Parent() {
		switch node.Type() {
		case "program":
			return true
		case "member_expression", "call_expression", "await_expression", "parenthesized_expression",
			"variable_declarator", "lexical_declaration", "variable_declaration", "assignment_expression",
			"expression_statement":
		default:
			return false
		}
	}
	return false
}

// flagMonkeyPatch flags an assignment to a member of an imported module,
// as fs.readFile = f.
func (p *projection) flagMonkeyPatch(syntax dynamicSyntax, left *sitter.Node) {
	if left == nil {
		return
	}
	if _, member := syntax.members[left.Type()]; !member {
		return
	}
	root := left
	for depth := 0; root != nil && depth < maxChainDepth; depth++ {
		if field, member := syntax.members[root.Type()]; member {
			root = root.ChildByFieldName(field)
		} else if syntax.wrappers[root.Type()] && root.NamedChildCount() > 0 {
			root = root.NamedChild(0)
		} else {
			break
		}
	}
	if root != nil && root.Type() == "identifier" && p.bindings[p.text(root)] {
		p.flag(fidx.FlagMonkeyPatch, left)
	}
}

// maxChainDepth bounds the walk down a chain of members, as a.b.c.d.
const maxChainDepth = 64

// stringPrefix returns the static start of a string, as ./locale/ for
// `./locale/${lang}` or './locale/' + lang. It reports whole for a string
// with no computed part.
func (p *projection) stringPrefix(node *sitter.Node) (string, bool) {
	if node == nil {
		return "", false
	}
	switch node.Type() {
	case "interpreted_string_literal", "raw_string_literal":
		text := p.text(node)
		if len(text) < 2 {
			return "", false
		}
		return text[1 : len(text)-1], true
	case "string", "template_string":
		var prefix strings.Builder
		for _, part := range ts.NamedChildren(node) {
			switch part.Type() {
			case "string_fragment", "string_content", "escape_sequence":
				prefix.WriteString(p.text(part))
			case "string_start", "string_end":
			default:
				return prefix.String(), false
			}
		}
		return prefix.String(), true
	case "binary_expression", "binary_operator":
		if p.text(node.ChildByFieldName("operator")) != "+" {
			return "", false
		}
		left, whole := p.stringPrefix(node.ChildByFieldName("left"))
		if !whole {
			return left, false
		}
		right, whole := p.stringPrefix(node.ChildByFieldName("right"))
		return left + right, whole
	}
	return "", false
}

func (p *projection) isString(node *sitter.Node) bool {
	_, whole := p.stringPrefix(node)
	return whole
}

func (p *projection) flag(kind fidx.FlagKind, node *sitter.Node) {
	p.flagAt(kind, node.StartByte(), lineOf(node))
}

// flagAt adds a flag to the file and to the innermost definition that holds
// the byte.
func (p *projection) flagAt(kind fidx.FlagKind, at uint32, line int) {
	flag := fidx.Flag{Kind: kind, Line: line}
	if !slices.Contains(p.ix.Flags, flag) {
		p.ix.Flags = append(p.ix.Flags, flag)
	}

	if def, ok := p.defs[p.innermost(at)]; ok && !slices.Contains(def.Flags, flag) {
		def.Flags = append(def.Flags, flag)
	}
}
