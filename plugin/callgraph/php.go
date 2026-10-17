package callgraph

import (
	"strings"

	"github.com/safedep/code/core"
	"github.com/safedep/code/langcode"
)

// A namespace of PHP is no namespace of the graph: the package of the
// index holds it, so a class of a namespace has the key of a top-level
// class, as in Java. self and static name the class of the code, and
// parent its base class.
var phpProfile = shapedProfile(languageProfile{
	code:            core.LanguageCodePHP,
	separators:      []string{"\\", "::", "->"},
	instanceKeyword: "$this",
	builtins:        langcode.Builtins(langcode.PHP),
	literals: map[string]bool{
		"boolean": true, "encapsed_string": true, "float": true, "integer": true, "null": true, "string": true, "heredoc": true,
	},
	ignored:      map[string]bool{"comment": true},
	transparent:  map[string]childPosition{"parenthesized_expression": firstChild},
	argumentList: "arguments",
	strings: stringShape{interpolations: map[string]bool{
		"member_call_expression": true, "function_call_expression": true, "scoped_call_expression": true,
		"nullsafe_member_call_expression": true, "member_access_expression": true, "subscript_expression": true,
	}},
	processors: map[string]nodeProcessor{
		"arrow_function":               arrowFunctionProcessor,
		"enum_declaration":             skipResultsProcessor,
		"readonly":                     skippedProcessor,
		"namespace_definition":         bodyProcessor,
		"simple_parameter":             parameterProcessor,
		"variadic_parameter":           parameterProcessor,
		"property_promotion_parameter": parameterProcessor,
		"named_type":                   skippedProcessor,
		"optional_type":                skippedProcessor,
		"union_type":                   skippedProcessor,
	},
	reachable: map[string]bool{"program": true, "arrow_function": true},
	// A fully qualified name, as in use \GuzzleHttp\Client, starts with a
	// separator that names no namespace.
	trimModule: func(name string) string { return strings.TrimPrefix(name, `\`) },
}, syntaxShapes{
	calls: map[string]callShape{
		"function_call_expression":        {function: named("function"), arguments: named("arguments")},
		"member_call_expression":          {receiver: named("object"), name: named("name"), arguments: named("arguments")},
		"nullsafe_member_call_expression": {receiver: named("object"), name: named("name"), arguments: named("arguments")},
		"scoped_call_expression":          {receiver: named("scope"), name: named("name"), arguments: named("arguments")},
	},
	creations: map[string]creationShape{
		"object_creation_expression": {},
	},
	members: map[string]memberShape{
		"member_access_expression":          {object: named("object"), name: named("name")},
		"nullsafe_member_access_expression": {object: named("object"), name: named("name")},
	},
	assignments: map[string]assignmentShape{
		"assignment_expression": {left: named("left"), right: named("right")},
	},
	scopes: map[string]scopeShape{
		"class_declaration":     {name: named("name"), body: named("body"), reachable: true, class: true},
		"trait_declaration":     {name: named("name"), body: named("body"), reachable: true, class: true},
		"interface_declaration": {name: named("name"), body: named("body"), class: true},
		"enum_declaration":      {name: named("name"), body: named("body"), class: true},
		"function_definition":   {name: named("name"), body: named("body"), params: named("parameters"), reachable: true},
		"method_declaration":    {name: named("name"), body: named("body"), params: named("parameters"), reachable: true, bodyRequired: true},
	},
	skipped: []string{"namespace_use_declaration", "attribute", "attribute_list"},
	values:  map[string]childRef{"cast_expression": named("value")},
	// $f() and $obj->$m() call a function that a value names.
	computedCallees: map[string]bool{
		"variable_name": true, "dynamic_variable_name": true, "subscript_expression": true,
		"member_access_expression": true, "anonymous_function_creation_expression": true,
	},
	classKeywords: []string{"self", "static"},
	superKeywords: []string{"parent"},
	callables: map[string]int{
		"call_user_func": 0, "call_user_func_array": 0, "forward_static_call": 0, "forward_static_call_array": 0,
		"array_map": 0, "array_filter": 1, "array_walk": 1, "array_reduce": 1, "usort": 1, "uasort": 1,
		"uksort": 1, "register_shutdown_function": 0, "set_error_handler": 0, "set_exception_handler": 0,
		"spl_autoload_register": 0, "iterator_apply": 1, "preg_replace_callback": 1,
	},
	staticCallables: map[string]bool{"string": true, "array_creation_expression": true, "arrow_function": true, "anonymous_function_creation_expression": true, "null": true},
})
