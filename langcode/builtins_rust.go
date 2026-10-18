package langcode

// rustBuiltins are the macros of std and the names of the prelude, which are in scope in every module.
var rustBuiltins = []string{
	"println", "print", "eprintln", "eprint", "format", "format_args", "write", "writeln",
	"vec", "panic", "assert", "assert_eq", "assert_ne", "debug_assert", "debug_assert_eq",
	"debug_assert_ne", "unreachable", "unimplemented", "todo", "matches", "dbg", "env",
	"option_env", "concat", "stringify", "include", "include_str", "include_bytes", "cfg",
	"line", "column", "file", "module_path", "compile_error", "thread_local",
	"Some", "None", "Ok", "Err", "drop", "Box", "Vec", "String", "Option", "Result",
	"std", "core", "alloc",
}
