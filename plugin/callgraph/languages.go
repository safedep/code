package callgraph

import "github.com/safedep/code/core"

func init() {
	registerShapedLanguage(core.LanguageCodeCSharp, syntaxShapes{
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
		skipped: []string{"using_directive", "file_scoped_namespace_declaration", "attribute_list"},
	})

	registerShapedLanguage(core.LanguageCodeRust, syntaxShapes{
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
		skipped:            []string{"use_declaration", "extern_crate_declaration", "attribute_item", "token_tree"},
		constructorMethods: []string{"new", "default"},
	})

	registerShapedLanguage(core.LanguageCodePHP, syntaxShapes{
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
		skipped: []string{"namespace_use_declaration"},
	})

	registerShapedLanguage(core.LanguageCodeRuby, syntaxShapes{
		calls: map[string]callShape{
			"call": {receiver: "receiver", name: "method", arguments: "arguments"},
		},
		assignments: map[string]assignmentShape{
			"assignment": {left: "left", right: "right"},
		},
		scopes: map[string]scopeShape{
			"class":            {name: "name", body: "body", reachable: true},
			"module":           {name: "name", body: "body"},
			"method":           {name: "name", body: "body", reachable: true},
			"singleton_method": {name: "name", body: "body", reachable: true},
		},
		constructorMethods: []string{"new"},
	})
}
