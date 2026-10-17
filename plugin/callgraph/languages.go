package callgraph

import (
	"strings"

	"github.com/safedep/code/core"
)

var csharpProfile = shapedProfile(languageProfile{
	code:            core.LanguageCodeCSharp,
	separators:      []string{"."},
	instanceKeyword: "this",
	literals:        map[string]bool{"character_literal": true, "raw_string_literal": true, "string_literal": true, "true": true, "false": true},
	ignored:         map[string]bool{"comment": true},
	transparent:     map[string]childPosition{"parenthesized_expression": firstChild, "await_expression": firstChild},
	argumentList:    "argument_list",
	processors: map[string]nodeProcessor{
		"enum_declaration": skipResultsProcessor,
		"as_expression":    skippedProcessor,
		"predefined_type":  skippedProcessor,
		"readonly":         skippedProcessor,
	},
}, syntaxShapes{
	calls: map[string]callShape{
		"invocation_expression": {function: "function", arguments: "arguments"},
	},
	creations: map[string]creationShape{
		"object_creation_expression": {typeField: "type", arguments: "arguments"},
	},
	members: map[string]memberShape{
		"member_access_expression": {object: "expression", name: "name"},
	},
	assignments: map[string]assignmentShape{
		"variable_declarator":   {left: "name"},
		"assignment_expression": {left: "left", right: "right"},
	},
	scopes: map[string]scopeShape{
		"namespace_declaration":    {name: "name", body: "body"},
		"class_declaration":        {name: "name", body: "body", reachable: true},
		"struct_declaration":       {name: "name", body: "body", reachable: true},
		"record_declaration":       {name: "name", body: "body", reachable: true},
		"interface_declaration":    {name: "name", body: "body"},
		"method_declaration":       {name: "name", body: "body", reachable: true},
		"constructor_declaration":  {name: "name", body: "body", reachable: true},
		"local_function_statement": {name: "name", body: "body", reachable: true},
	},
	skipped: []string{"using_directive", "attribute_list", "attribute"},
	values:  map[string]string{"cast_expression": "value"},
})

var rustProfile = shapedProfile(languageProfile{
	code:            core.LanguageCodeRust,
	separators:      []string{"::", "."},
	instanceKeyword: "self",
	literals:        map[string]bool{"float_literal": true, "raw_string_literal": true, "string_literal": true, "true": true, "false": true},
	ignored:         map[string]bool{"line_comment": true},
	transparent: map[string]childPosition{
		"parenthesized_expression": firstChild,
		"try_expression":           firstChild,
		"await_expression":         firstChild,
	},
	argumentList: "arguments",
	processors: map[string]nodeProcessor{
		"scoped_type_identifier": scopedIdentifierProcessor,
		"type_arguments":         skippedProcessor,
		"type_identifier":        skippedProcessor,
	},
}, syntaxShapes{
	calls: map[string]callShape{
		"call_expression":  {function: "function", arguments: "arguments"},
		"macro_invocation": {function: "macro"},
	},
	members: map[string]memberShape{
		"field_expression": {object: "value", name: "field"},
	},
	assignments: map[string]assignmentShape{
		"let_declaration":       {left: "pattern", right: "value"},
		"assignment_expression": {left: "left", right: "right"},
	},
	scopes: map[string]scopeShape{
		"function_item": {name: "name", body: "body", reachable: true},
		"impl_item":     {name: "type", body: "body"},
		"trait_item":    {name: "name", body: "body"},
		"mod_item":      {name: "name", body: "body"},
	},
	skipped:            []string{"use_declaration", "extern_crate_declaration", "attribute_item", "attribute", "token_tree"},
	values:             map[string]string{"generic_function": "function", "type_cast_expression": "value"},
	constructorMethods: []string{"new", "default"},
})

var phpProfile = shapedProfile(languageProfile{
	code:            core.LanguageCodePHP,
	separators:      []string{"\\", "::", "->"},
	instanceKeyword: "$this",
	literals: map[string]bool{
		"boolean": true, "encapsed_string": true, "float": true, "integer": true, "null": true, "string": true,
	},
	ignored:      map[string]bool{"comment": true},
	transparent:  map[string]childPosition{"parenthesized_expression": firstChild},
	argumentList: "arguments",
	processors: map[string]nodeProcessor{
		"arrow_function":   arrowFunctionProcessor,
		"enum_declaration": skipResultsProcessor,
		"readonly":         skippedProcessor,
	},
	reachable: map[string]bool{"program": true, "arrow_function": true},
	// A fully qualified name, as in use \GuzzleHttp\Client, starts with a
	// separator that names no namespace.
	trimModule: func(name string) string { return strings.TrimPrefix(name, `\`) },
}, syntaxShapes{
	calls: map[string]callShape{
		"function_call_expression":        {function: "function", arguments: "arguments"},
		"member_call_expression":          {receiver: "object", name: "name", arguments: "arguments"},
		"nullsafe_member_call_expression": {receiver: "object", name: "name", arguments: "arguments"},
		"scoped_call_expression":          {receiver: "scope", name: "name", arguments: "arguments"},
	},
	creations: map[string]creationShape{
		"object_creation_expression": {},
	},
	members: map[string]memberShape{
		"member_access_expression":          {object: "object", name: "name"},
		"nullsafe_member_access_expression": {object: "object", name: "name"},
	},
	assignments: map[string]assignmentShape{
		"assignment_expression": {left: "left", right: "right"},
	},
	scopes: map[string]scopeShape{
		"namespace_definition":  {name: "name", body: "body"},
		"class_declaration":     {name: "name", body: "body", reachable: true},
		"trait_declaration":     {name: "name", body: "body", reachable: true},
		"interface_declaration": {name: "name", body: "body"},
		"function_definition":   {name: "name", body: "body", reachable: true},
		"method_declaration":    {name: "name", body: "body", reachable: true},
	},
	skipped: []string{"namespace_use_declaration", "attribute"},
	values:  map[string]string{"cast_expression": "value"},
})

var rubyProfile = shapedProfile(languageProfile{
	code:            core.LanguageCodeRuby,
	separators:      []string{"::", "."},
	instanceKeyword: "self",
	reachable:       map[string]bool{"program": true},
	literals: map[string]bool{
		"character": true, "false": true, "float": true, "integer": true, "string": true, "true": true,
	},
	collections:  map[string]bool{"array": true},
	ignored:      map[string]bool{"comment": true},
	transparent:  map[string]childPosition{"parenthesized_statements": firstChild},
	argumentList: "argument_list",
}, syntaxShapes{
	calls: map[string]callShape{
		"call": {receiver: "receiver", name: "method", arguments: "arguments"},
	},
	assignments: map[string]assignmentShape{
		"assignment": {left: "left", right: "right"},
	},
	scopes: map[string]scopeShape{
		"class":            {name: "name", body: "body", reachable: true},
		"module":           {name: "name", body: "body", reachable: true},
		"method":           {name: "name", body: "body", reachable: true},
		"singleton_method": {name: "name", body: "body", reachable: true},
	},
	constructorMethods: []string{"new"},
})
