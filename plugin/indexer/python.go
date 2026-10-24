package indexer

import (
	"github.com/safedep/code/plugin/callgraph"
	"regexp"
	"slices"
	"strings"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/pkg/ts"
	sitter "github.com/smacker/go-tree-sitter"
)

var pythonRules = languageRules{
	kinds: map[string]fileindex.DefKind{
		"class_definition":    fileindex.DefClass,
		"function_definition": fileindex.DefFunction,
	},
	constructor:    "__init__",
	typedReceivers: true,
	bases:          pythonBases,
	fields:         pythonFields,
	decorators:     decorators,
	signature:      pythonSignature,
	assigned:       pythonAssigned,
	exports:        pythonExports,
	visitors:       []func(p *projection) visitor{flagVisitor, mainGuardVisitor, referenceVisitor},
	dynamic: &dynamicSyntax{
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
		// A Python subscript is a lookup in a dict, so a string index
		// computes the callee too.
		stringIndexComputes: true,
	},
	references: &referenceSyntax{
		holders: map[string]string{
			"argument_list":            "",
			"keyword_argument":         "value",
			"list":                     "",
			"tuple":                    "",
			"set":                      "",
			"expression_list":          "",
			"pair":                     "value",
			"assignment":               "right",
			"return_statement":         "",
			"default_parameter":        "value",
			"typed_default_parameter":  "value",
			"conditional_expression":   "",
			"boolean_operator":         "",
			"parenthesized_expression": "",
		},
		member:      "attribute",
		object:      "object",
		property:    "attribute",
		identifiers: map[string]bool{"identifier": true},
		instances:   map[string]bool{"self": true, "cls": true},
		bases:       map[string]string{"class_definition": "superclasses"},
	},
	arguments: &argumentSyntax{
		calls: map[string]string{"call": "function"},
		// A keyword argument has no position, so a rule that names a
		// position does not read it.
		skipped:  map[string]bool{"comment": true, "keyword_argument": true, "dictionary_splat": true},
		strings:  map[string]bool{"string": true, "concatenated_string": true},
		literals: map[string]bool{"integer": true, "float": true, "true": true, "false": true, "none": true, "list": true, "dictionary": true, "tuple": true, "set": true},
	},
	mainGuard: &mainGuard{statement: "if_statement", condition: regexp.MustCompile(`^(__name__\s*==\s*["']__main__["']|["']__main__["']\s*==\s*__name__)$`)},
}

// pythonBases returns the bases of a Python class as written, as Base and
// mod.Base in class A(Base, mod.Base, metaclass=M). A keyword argument, as
// the metaclass, is no base.
func pythonBases(p *projection, class *sitter.Node) []string {
	if class.Type() != "class_definition" {
		return nil
	}
	var bases []string
	if list := class.ChildByFieldName("superclasses"); list != nil {
		for _, base := range ts.NamedChildren(list) {
			if base.Type() != "keyword_argument" {
				bases = append(bases, p.text(base))
			}
		}
	}
	return bases
}

// pythonAssigned returns the name of a top-level assignment, as x in x = f().
func pythonAssigned(_ *projection, statement *sitter.Node) []*sitter.Node {
	if statement.Type() != "expression_statement" || statement.NamedChildCount() == 0 {
		return nil
	}
	if assignment := statement.NamedChild(0); assignment.Type() == "assignment" {
		return []*sitter.Node{assignment.ChildByFieldName("left")}
	}
	return nil
}

// pythonExports adds the names of the __all__ of the file as its exports.
func pythonExports(p *projection) {
	for _, statement := range ts.NamedChildren(p.tree.Tree().RootNode()) {
		if statement.Type() != "expression_statement" || statement.NamedChildCount() == 0 {
			continue
		}
		assignment := statement.NamedChild(0)
		left, right := assignment.ChildByFieldName("left"), assignment.ChildByFieldName("right")
		if left != nil && right != nil && left.Type() == "identifier" && p.text(left) == "__all__" {
			p.addAll(right, lineOf(statement))
		}
	}
}

// addAll adds the names of __all__ when it is a list or a tuple of string
// literals. A computed __all__ gives no export.
func (p *projection) addAll(value *sitter.Node, line int) {
	if value.Type() != "list" && value.Type() != "tuple" {
		return
	}
	var names []string
	for _, item := range ts.NamedChildren(value) {
		name, whole := p.stringPrefix(item)
		if !whole {
			return
		}
		names = append(names, name)
	}
	for _, name := range names {
		p.export(name, name, line)
	}
}

// pythonSignature fills the typed parameters, the return type and the typed
// locals of a Python function from its type hints. A hint is the only
// declared type that Python has, and the link step trusts it as it trusts
// a TypeScript type: both are erased at run time. A module gets the typed
// names of its top level that are no variable of their own, as q in
// q, n = pair().
func pythonSignature(p *projection, def *fileindex.Definition, node *sitter.Node) {
	if def.Kind == fileindex.DefModule {
		for _, t := range p.pythonLocals(node, "") {
			if _, defined := p.defs[fileindex.Join(def.Key, t.Name)]; !defined {
				def.Locals = append(def.Locals, t)
			}
		}
		return
	}
	if def.Kind != fileindex.DefFunction && def.Kind != fileindex.DefMethod && def.Kind != fileindex.DefConstructor {
		return
	}
	def.Params = p.pythonParams(node.ChildByFieldName("parameters"))
	if ret := node.ChildByFieldName("return_type"); ret != nil {
		def.Returns = p.pythonTypeName(ret)
		if results := p.pythonTupleTypes(ret); len(results) > 1 {
			def.Results = results
		}
	}
	def.Locals = p.pythonLocals(node.ChildByFieldName("body"), "")
}

// pythonParams returns the parameters of a function that declare a type
// name, as parser in run(parser: Parser) or run(parser: Parser = None).
func (p *projection) pythonParams(list *sitter.Node) []fileindex.Typed {
	if list == nil {
		return nil
	}
	var typed []fileindex.Typed
	for _, param := range ts.NamedChildren(list) {
		var name *sitter.Node
		switch param.Type() {
		case "typed_parameter":
			if param.NamedChildCount() > 0 && param.NamedChild(0).Type() == "identifier" {
				name = param.NamedChild(0)
			}
		case "typed_default_parameter":
			name = param.ChildByFieldName("name")
		default:
			continue
		}
		if name == nil {
			continue
		}
		if typ := p.pythonTypeName(param.ChildByFieldName("type")); typ != "" {
			typed = append(typed, fileindex.Typed{Name: p.text(name), Type: typ})
		}
	}
	return typed
}

// pythonOptional names the hints that wrap one type and let it be None.
var pythonOptional = map[string]bool{"Optional": true, "typing.Optional": true, "Union": true, "typing.Union": true}

// pythonTuples names the hints of a tuple, whose members a tuple return
// gives by position.
var pythonTuples = map[string]bool{"tuple": true, "Tuple": true, "typing.Tuple": true}

// pythonTypeName returns the class name that a type hint names, as written:
// Parser, dep.Parser, the Parser of Optional[Parser], Parser | None or
// "Parser", and "" for a builtin, a container or a union of two classes.
func (p *projection) pythonTypeName(node *sitter.Node) string {
	if node == nil {
		return ""
	}
	switch node.Type() {
	case "type":
		if node.NamedChildCount() == 0 {
			return ""
		}
		return p.pythonTypeName(node.NamedChild(0))
	case "identifier", "attribute":
		if name := p.text(node); !langcode.IsBuiltin(langcode.Python, name) {
			return name
		}
	case "string":
		for _, child := range ts.NamedChildren(node) {
			if child.Type() == "string_content" {
				if name := p.text(child); isDottedName(name) && !langcode.IsBuiltin(langcode.Python, name) {
					return name
				}
			}
		}
	case "generic_type", "subscript":
		head, args := p.pythonGenericParts(node)
		if !pythonOptional[head] {
			return ""
		}
		return p.pythonOneOf(args)
	case "binary_operator":
		return p.pythonOneOf([]*sitter.Node{node.ChildByFieldName("left"), node.ChildByFieldName("right")})
	}
	return ""
}

// pythonOneOf returns the one class among the members of a union, with
// None left out, and "" for two classes or none.
func (p *projection) pythonOneOf(members []*sitter.Node) string {
	name := ""
	for _, m := range members {
		if m == nil || m.Type() == "none" {
			continue
		}
		if m.Type() == "binary_operator" {
			if inner := p.pythonOneOf([]*sitter.Node{m.ChildByFieldName("left"), m.ChildByFieldName("right")}); inner != "" && name == "" {
				name = inner
				continue
			}
			return ""
		}
		t := p.pythonTypeName(m)
		if t == "" || name != "" {
			return ""
		}
		name = t
	}
	return name
}

// pythonGenericParts returns the head of a hint with type arguments and
// the argument nodes, as Optional and Parser for Optional[Parser]. The
// grammar writes the hint as a generic_type, or as a subscript in an
// older version.
func (p *projection) pythonGenericParts(node *sitter.Node) (string, []*sitter.Node) {
	if node.Type() == "subscript" {
		return p.text(node.ChildByFieldName("value")), childrenByField(node, "subscript")
	}
	head := ""
	var args []*sitter.Node
	for _, child := range ts.NamedChildren(node) {
		switch child.Type() {
		case "identifier", "attribute":
			if head == "" {
				head = p.text(child)
			}
		case "type_parameter":
			args = append(args, ts.NamedChildren(child)...)
		}
	}
	return head, args
}

// pythonTupleTypes returns the class name of each member of a tuple hint,
// as a return of tuple[Parser, int] gives them by position, and nil for a
// hint that is no tuple.
func (p *projection) pythonTupleTypes(node *sitter.Node) []string {
	if node != nil && node.Type() == "type" && node.NamedChildCount() > 0 {
		node = node.NamedChild(0)
	}
	if node == nil || node.Type() != "generic_type" && node.Type() != "subscript" {
		return nil
	}
	head, args := p.pythonGenericParts(node)
	if !pythonTuples[head] {
		return nil
	}
	out := make([]string, 0, len(args))
	for _, arg := range args {
		out = append(out, p.pythonTypeName(arg))
	}
	return out
}

// pythonLocals returns the names that a function body binds with a type:
// an annotated assignment, as t: Thing = make(), and a name that holds the
// result of a call, as t = make() or a, b = make(). With an instance name,
// as self, it returns the attributes of the instance that the body sets
// instead, as the fields of the class.
func (p *projection) pythonLocals(body *sitter.Node, instance string) []fileindex.Typed {
	var typed []fileindex.Typed
	stop := map[string]bool{"function_definition": true, "class_definition": true, "lambda": true}
	walkBody(body, stop, func(node *sitter.Node) {
		if node.Type() != "assignment" {
			return
		}
		left := node.ChildByFieldName("left")
		if left == nil {
			return
		}
		var names []*sitter.Node
		switch {
		case instance != "" && left.Type() == "attribute":
			object, attr := left.ChildByFieldName("object"), left.ChildByFieldName("attribute")
			if object == nil || attr == nil || object.Type() != "identifier" || p.text(object) != instance {
				return
			}
			names = []*sitter.Node{attr}
		case instance == "" && left.Type() == "identifier":
			names = []*sitter.Node{left}
		case instance == "" && (left.Type() == "pattern_list" || left.Type() == "tuple_pattern"):
			names = ts.NamedChildren(left)
		default:
			return
		}
		if typ := p.pythonTypeName(node.ChildByFieldName("type")); typ != "" && len(names) == 1 {
			typed = append(typed, fileindex.Typed{Name: p.text(names[0]), Type: typ})
			return
		}
		typed = append(typed, p.pythonAssignedTypes(names, node.ChildByFieldName("right"))...)
	})
	return typed
}

// pythonAssignedTypes returns the names of an assignment that hold the
// result of a call, each at its position, as a and b in a, b = make(). A
// call that reads its value by reflection, as importlib.import_module(name),
// gives no type.
func (p *projection) pythonAssignedTypes(names []*sitter.Node, right *sitter.Node) []fileindex.Typed {
	if right == nil || right.Type() != "call" {
		return nil
	}
	function := right.ChildByFieldName("function")
	if function == nil || function.Type() != "identifier" && function.Type() != "attribute" || callgraph.PythonReflectiveCallee(p.text(function)) {
		return nil
	}
	var typed []fileindex.Typed
	for i, name := range names {
		if name.Type() == "identifier" && p.text(name) != "_" {
			typed = append(typed, fileindex.Typed{Name: p.text(name), Type: p.text(function), Result: true, Index: i})
		}
	}
	return typed
}

// pythonFields returns the fields of a Python class that declare a type or
// hold the result of a call: an annotated name of the class body, as the
// field of a dataclass, and an attribute of the instance that a method
// sets, as self.parser: Parser = Parser() or self.parser = Parser(). The
// first setting of a name wins.
func pythonFields(p *projection, class *sitter.Node) []fileindex.Typed {
	body := class.ChildByFieldName("body")
	if class.Type() != "class_definition" || body == nil {
		return nil
	}
	var fields []fileindex.Typed
	seen := map[string]bool{}
	add := func(ts []fileindex.Typed) {
		for _, t := range ts {
			if !seen[t.Name] {
				seen[t.Name] = true
				fields = append(fields, t)
			}
		}
	}
	for _, statement := range ts.NamedChildren(body) {
		if statement.Type() != "expression_statement" || statement.NamedChildCount() == 0 {
			continue
		}
		assignment := statement.NamedChild(0)
		left := assignment.ChildByFieldName("left")
		if assignment.Type() != "assignment" || left == nil || left.Type() != "identifier" {
			continue
		}
		if typ := p.pythonTypeName(assignment.ChildByFieldName("type")); typ != "" {
			add([]fileindex.Typed{{Name: p.text(left), Type: typ}})
		}
	}
	for _, member := range ts.NamedChildren(body) {
		if member.Type() == "decorated_definition" {
			member = member.ChildByFieldName("definition")
		}
		if member == nil || member.Type() != "function_definition" {
			continue
		}
		params := member.ChildByFieldName("parameters")
		if params == nil || params.NamedChildCount() == 0 || params.NamedChild(0).Type() != "identifier" {
			continue
		}
		add(p.pythonLocals(member.ChildByFieldName("body"), p.text(params.NamedChild(0))))
	}
	return fields
}

// isDottedName reports whether a text is a name or a dotted name, as a
// forward reference in a string hint writes it.
func isDottedName(text string) bool {
	if text == "" {
		return false
	}
	for _, part := range strings.Split(text, ".") {
		if !isIdentifier(part) {
			return false
		}
	}
	return true
}

// isIdentifier reports whether a text is one name.
func isIdentifier(text string) bool {
	if text == "" {
		return false
	}
	for i, r := range text {
		letter := r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
		digit := i > 0 && r >= '0' && r <= '9'
		if !letter && !digit {
			return false
		}
	}
	return true
}
