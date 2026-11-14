package langcode

import "slices"

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

// rustPointers are the smart pointers of std. A method call on a pointer
// runs a method of the value that it holds, as s.save() on a Box<dyn Store>.
var rustPointers = []string{"Box", "Rc", "Arc"}

// IsRustPointer reports whether the name is a smart pointer of std.
func IsRustPointer(name string) bool {
	return slices.Contains(rustPointers, name)
}
