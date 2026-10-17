package callgraph

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/safedep/code/core"
	"github.com/safedep/code/langcode"
	sitter "github.com/smacker/go-tree-sitter"
)

var rustProfile = shapedProfile(languageProfile{
	code:            core.LanguageCodeRust,
	separators:      []string{"::", "."},
	instanceKeyword: "self",
	builtins:        langcode.Builtins(langcode.Rust),
	literals: map[string]bool{
		"float_literal": true, "integer_literal": true, "char_literal": true, "boolean_literal": true,
		"raw_string_literal": true, "string_literal": true, "true": true, "false": true,
	},
	ignored: map[string]bool{"line_comment": true, "block_comment": true},
	transparent: map[string]childPosition{
		"parenthesized_expression": firstChild,
		"try_expression":           firstChild,
		"await_expression":         firstChild,
		"reference_expression":     firstChild,
		"mut_pattern":              firstChild,
	},
	argumentList: "arguments",
	processors: map[string]nodeProcessor{
		"scoped_type_identifier": scopedIdentifierProcessor,
		"type_arguments":         skippedProcessor,
		"type_identifier":        skippedProcessor,
		"parameter":              parameterProcessor,
		"token_tree":             rustTokenTreeProcessor,
	},
}, syntaxShapes{
	calls: map[string]callShape{
		"call_expression":  {function: named("function"), arguments: named("arguments")},
		"macro_invocation": {function: named("macro")},
	},
	creations: map[string]creationShape{
		"struct_expression": {typeField: named("name"), arguments: named("body")},
	},
	members: map[string]memberShape{
		"field_expression": {object: named("value"), name: named("field")},
	},
	assignments: map[string]assignmentShape{
		"let_declaration":       {left: named("pattern"), right: named("value")},
		"assignment_expression": {left: named("left"), right: named("right")},
	},
	scopes: map[string]scopeShape{
		"function_item": {name: named("name"), body: named("body"), params: named("parameters"), reachable: true},
		"impl_item":     {name: named("type"), body: named("body"), class: true},
		"trait_item":    {name: named("name"), body: named("body"), class: true},
		"mod_item":      {name: named("name"), body: named("body"), bodyRequired: true},
		// A macro runs the tokens of its rules where a call names it.
		"macro_definition": {name: named("name"), header: true, reachable: true},
	},
	skipped:            []string{"use_declaration", "extern_crate_declaration", "attribute_item", "inner_attribute_item"},
	values:             map[string]childRef{"generic_function": named("function"), "type_cast_expression": named("value")},
	constructorMethods: []string{"new", "default"},
	classKeywords:      []string{"Self"},
	computedCallees:    map[string]bool{"index_expression": true},
})

// rustTokenTreeProcessor reads the calls in the tokens of a macro, as
// helper(2) in println!("{}", helper(2)). The grammar keeps the arguments of
// a macro as tokens. A path before a group in parentheses is a call of the
// path, and a type name before a group in braces makes an instance of the
// type, as Pretty {} in println!("{}", Pretty {}). A name after a dot is a method call on a receiver that the tokens
// do not type, so it stays a record.
func rustTokenTreeProcessor(node *sitter.Node, treeData []byte, currentNamespace string, callGraph *CallGraph, metadata processorMetadata) processorResult {
	tokens := make([]*sitter.Node, 0, node.ChildCount())
	for i := range int(node.ChildCount()) {
		tokens = append(tokens, node.Child(i))
	}
	shaped := shapedProcessors{separators: callGraph.profile.separators}
	for i, token := range tokens {
		if token.Type() != "token_tree" {
			continue
		}
		rustTokenTreeProcessor(token, treeData, currentNamespace, callGraph, metadata)
		if i == 0 || tokens[i-1].Type() != "identifier" {
			continue
		}
		creation := strings.HasPrefix(token.Content(treeData), "{")
		if !creation && !strings.HasPrefix(token.Content(treeData), "(") {
			continue
		}
		start := i - 1
		for start >= 2 && tokens[start-1].Type() == "::" && tokens[start-2].Type() == "identifier" {
			start -= 2
		}
		name := tokens[i-1]
		if r, _ := utf8.DecodeRuneInString(name.Content(treeData)); creation && !unicode.IsUpper(r) {
			continue
		}
		if start > 0 && tokens[start-1].Type() == "." {
			callGraph.addRecord(currentNamespace, name, "", name.Content(treeData), treeData, UnresolvedUnknownReceiver)
			continue
		}
		var path strings.Builder
		for _, t := range tokens[start:i] {
			path.WriteString(t.Content(treeData))
		}
		kind := CallCall
		if creation {
			kind = CallNew
		}
		callee := shaped.resolvePath(path.String(), currentNamespace, callGraph)
		callGraph.addEdge(currentNamespace, kind, name, callee, nil, []CallArgument{})
	}
	return newProcessorResult()
}
