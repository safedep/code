package langcode

// builtins are the names that a file of the language uses with no import or
// definition: functions, types and runtime globals.
var builtins = map[Code][]string{
	Python: {
		"ArithmeticError", "AssertionError", "AttributeError", "BaseException", "BaseExceptionGroup",
		"BlockingIOError", "BrokenPipeError", "BufferError", "BytesWarning", "ChildProcessError",
		"ConnectionAbortedError", "ConnectionError", "ConnectionRefusedError",
		"ConnectionResetError", "DeprecationWarning", "EOFError", "Ellipsis", "EncodingWarning",
		"EnvironmentError", "Exception", "ExceptionGroup", "False", "FileExistsError",
		"FileNotFoundError", "FloatingPointError", "FutureWarning", "GeneratorExit", "IOError",
		"ImportError", "ImportWarning", "IndentationError", "IndexError", "InterruptedError",
		"IsADirectoryError", "KeyError", "KeyboardInterrupt", "LookupError", "MemoryError",
		"ModuleNotFoundError", "NameError", "None", "NotADirectoryError", "NotImplemented",
		"NotImplementedError", "OSError", "OverflowError", "PendingDeprecationWarning",
		"PermissionError", "ProcessLookupError", "RecursionError", "ReferenceError",
		"ResourceWarning", "RuntimeError", "RuntimeWarning", "StopAsyncIteration", "StopIteration",
		"SyntaxError", "SyntaxWarning", "SystemError", "SystemExit", "TabError", "TimeoutError",
		"True", "TypeError", "UnboundLocalError", "UnicodeDecodeError", "UnicodeEncodeError",
		"UnicodeError", "UnicodeTranslateError", "UnicodeWarning", "UserWarning", "ValueError",
		"Warning", "ZeroDivisionError", "__build_class__", "__debug__", "__doc__", "__import__",
		"__loader__", "__name__", "__package__", "__spec__", "abs", "aiter", "all", "anext", "any",
		"ascii", "bin", "bool", "breakpoint", "bytearray", "bytes", "callable", "chr", "classmethod",
		"compile", "complex", "copyright", "credits", "delattr", "dict", "dir", "divmod",
		"enumerate", "eval", "exec", "exit", "filter", "float", "format", "frozenset", "getattr",
		"globals", "hasattr", "hash", "help", "hex", "id", "input", "int", "isinstance",
		"issubclass", "iter", "len", "license", "list", "locals", "map", "max", "memoryview", "min",
		"next", "object", "oct", "open", "ord", "pow", "print", "property", "quit", "range", "repr",
		"reversed", "round", "set", "setattr", "slice", "sorted", "staticmethod", "str", "sum",
		"super", "tuple", "type", "vars", "zip",
	},
	Javascript: ecmascriptBuiltins,
	Typescript: ecmascriptBuiltins,
	Go: {
		"panic", "recover", "make", "new", "len", "cap", "append", "copy", "delete", "close",
		"complex", "real", "imag", "print", "println", "iota", "true", "false", "nil",
		"min", "max", "clear",
		// A call of a predeclared type is a conversion, as string(b).
		"bool", "byte", "rune", "string", "error", "any", "comparable", "uintptr",
		"int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64", "complex64", "complex128",
	},
	Java:   javaBuiltins,
	CSharp: {"nameof"},
	// Rust has the macros of std and the prelude in scope in every module.
	Rust: {
		"println", "print", "eprintln", "eprint", "format", "format_args", "write", "writeln",
		"vec", "panic", "assert", "assert_eq", "assert_ne", "debug_assert", "debug_assert_eq",
		"debug_assert_ne", "unreachable", "unimplemented", "todo", "matches", "dbg", "env",
		"option_env", "concat", "stringify", "include", "include_str", "include_bytes", "cfg",
		"line", "column", "file", "module_path", "compile_error", "thread_local",
		"Some", "None", "Ok", "Err", "drop", "Box", "Vec", "String", "Option", "Result",
		"std", "core", "alloc",
	},
	Ruby: {
		"puts", "print", "p", "pp", "require", "require_relative", "load", "autoload", "raise",
		"fail", "loop", "lambda", "proc", "format", "sprintf", "printf", "gets", "sleep", "exit",
		"exit!", "abort", "at_exit", "rand", "srand", "catch", "throw", "binding", "block_given?",
		"caller", "warn", "system", "exec", "spawn", "fork", "open", "select", "trap", "freeze",
		"attr_accessor", "attr_reader", "attr_writer", "include", "extend", "prepend", "private",
		"public", "protected", "module_function", "private_constant", "private_class_method",
		"public_class_method", "define_method", "alias_method", "Integer", "Float", "String",
		"Array", "Hash", "Rational", "Complex", "File", "Dir", "IO", "Kernel", "Object",
		"BasicObject", "Class", "Module", "Struct", "Comparable", "Enumerable", "Math", "Time",
		"Process", "Thread", "Mutex", "Queue", "Exception", "StandardError", "RuntimeError",
		"ArgumentError", "NameError", "NoMethodError", "TypeError", "KeyError", "IndexError",
		"StopIteration", "NotImplementedError", "IOError", "ScriptError", "LoadError",
		"SystemExit", "Interrupt", "ENV", "ARGV", "STDIN", "STDOUT", "STDERR", "Symbol", "Range",
		"Regexp", "Proc", "Method", "Encoding", "Marshal", "ObjectSpace", "GC", "Signal",
		"Random", "NilClass", "TrueClass", "FalseClass", "Numeric",
	},
	PHP: {
		"strlen", "count", "isset", "unset", "empty", "array_map", "array_filter", "array_merge",
		"array_keys", "array_values", "in_array", "implode", "explode", "sprintf", "printf", "vsprintf",
		"str_replace", "substr", "strpos", "stripos", "strrpos", "trim", "rtrim", "ltrim",
		"strtolower", "strtoupper", "ucfirst", "lcfirst", "ucwords", "json_encode", "json_decode",
		"var_dump", "var_export", "print_r", "is_array", "is_string", "is_null", "is_int",
		"is_bool", "is_float", "is_numeric", "is_object", "is_callable", "intval", "floatval",
		"strval", "boolval", "file_get_contents", "file_put_contents", "fopen", "fclose", "fwrite",
		"fread", "fgets", "feof", "preg_match", "preg_match_all", "preg_replace",
		"preg_replace_callback", "preg_split", "preg_quote", "array_key_exists", "array_push",
		"array_pop", "array_shift", "array_unshift", "array_slice", "array_splice", "sort", "rsort",
		"usort", "uasort", "uksort", "ksort", "krsort", "asort", "arsort", "date", "time",
		"microtime", "strtotime", "mktime", "header", "define", "defined", "constant",
		"function_exists", "class_exists", "method_exists", "property_exists", "call_user_func",
		"call_user_func_array", "func_get_args", "compact", "extract", "range", "min", "max", "abs",
		"round", "floor", "ceil", "rand", "mt_rand", "random_int", "random_bytes", "uniqid", "md5",
		"sha1", "hash", "hash_hmac", "crc32", "base64_encode", "base64_decode", "urlencode",
		"urldecode", "rawurlencode", "http_build_query", "parse_str", "parse_url",
		"htmlspecialchars", "htmlentities", "html_entity_decode", "strip_tags", "nl2br",
		"number_format", "array_combine", "array_flip", "array_unique", "array_reverse",
		"array_search", "array_sum", "array_walk", "array_reduce", "array_column", "array_fill",
		"array_diff", "array_intersect", "array_key_first", "array_key_last", "iterator_to_array",
		"getenv", "putenv", "ini_set", "ini_get", "error_reporting", "set_error_handler",
		"set_exception_handler", "trigger_error", "error_log", "sleep", "usleep", "exit", "die",
		"serialize", "unserialize", "get_class", "get_parent_class", "get_object_vars",
		"spl_autoload_register", "spl_object_hash", "register_shutdown_function", "ob_start",
		"ob_get_clean", "session_start", "setcookie", "exec", "shell_exec", "system", "passthru",
		"proc_open", "popen", "curl_init", "curl_exec", "curl_setopt", "curl_close", "mail",
		"dirname", "basename", "pathinfo", "realpath", "file_exists", "is_file", "is_dir",
		"is_readable", "is_writable", "mkdir", "rmdir", "unlink", "rename", "copy", "glob",
		"scandir", "file", "str_contains", "str_starts_with", "str_ends_with", "str_repeat",
		"str_pad", "str_split", "wordwrap", "mb_strlen", "mb_substr", "mb_strtolower",
		"mb_strtoupper", "iconv", "chr", "ord", "dechex", "hexdec", "bin2hex", "hex2bin",
		"password_hash", "password_verify", "openssl_encrypt", "openssl_decrypt", "array_is_list",
		"Exception", "ErrorException", "Error", "TypeError", "ValueError", "Throwable",
		"RuntimeException", "LogicException", "InvalidArgumentException", "DomainException",
		"LengthException", "OutOfRangeException", "OutOfBoundsException", "RangeException",
		"OverflowException", "UnderflowException", "UnexpectedValueException", "JsonException",
		"DateTime", "DateTimeImmutable", "DateTimeZone", "DateInterval", "PDO", "PDOStatement",
		"PDOException", "ArrayObject", "ArrayIterator", "SplObjectStorage", "SplStack",
		"SplQueue", "stdClass", "Closure", "Generator", "ReflectionClass", "ReflectionMethod",
		"ReflectionProperty", "ReflectionFunction",
	},
	Kotlin: append([]string{
		"println", "print", "readLine", "readln", "listOf", "listOfNotNull", "mutableListOf",
		"arrayListOf", "emptyList", "mapOf", "mutableMapOf", "hashMapOf", "linkedMapOf",
		"emptyMap", "setOf", "mutableSetOf", "hashSetOf", "linkedSetOf", "emptySet",
		"arrayOf", "arrayOfNulls", "emptyArray", "intArrayOf", "longArrayOf", "doubleArrayOf",
		"byteArrayOf", "charArrayOf", "booleanArrayOf", "sequenceOf", "emptySequence",
		"generateSequence", "buildList", "buildMap", "buildSet", "buildString", "lazy",
		"require", "requireNotNull", "check", "checkNotNull", "error", "TODO", "assert",
		"repeat", "run", "let", "also", "apply", "with", "takeIf", "takeUnless",
		"runCatching", "synchronized", "maxOf", "minOf", "to", "Pair", "Triple", "Result",
		"Any", "Unit", "Nothing", "Int", "Long", "Short", "Byte", "Double", "Float",
		"Char", "Boolean", "Array", "List", "MutableList", "Map", "MutableMap", "Set",
		"MutableSet", "Sequence", "Regex", "Lazy", "IllegalStateException",
		"NoSuchElementException", "KotlinNullPointerException",
	}, javaBuiltins...),
}

var javaBuiltins = []string{
	"Object", "Class", "Enum", "String", "StringBuilder", "StringBuffer", "System", "Runtime",
	"Thread", "ThreadGroup", "Throwable", "Exception", "Error", "Math", "StrictMath", "Number",
	"Void", "Package", "Process", "ProcessBuilder", "SecurityManager", "Boolean", "Byte",
	"Character", "Short", "Integer", "Long", "Float", "Double",
	"Runnable", "Iterable", "Comparable", "AutoCloseable", "CharSequence", "Cloneable",
	"Override", "Deprecated", "SuppressWarnings", "FunctionalInterface", "SafeVarargs",
	"RuntimeException", "IllegalArgumentException", "IllegalStateException",
	"NullPointerException", "UnsupportedOperationException", "IndexOutOfBoundsException",
	"ArithmeticException", "ClassCastException", "InterruptedException",
	"NumberFormatException", "CloneNotSupportedException", "ReflectiveOperationException",
	"ClassNotFoundException", "SecurityException", "AssertionError", "OutOfMemoryError",
	"StackOverflowError", "ThreadLocal", "InheritableThreadLocal", "Record", "StackWalker",
}

var ecmascriptBuiltins = []string{
	"console", "parseInt", "parseFloat", "isNaN", "isFinite", "setTimeout", "setInterval",
	"clearTimeout", "clearInterval", "eval", "encodeURI", "decodeURI", "encodeURIComponent",
	"decodeURIComponent", "Number", "String", "Boolean", "Array", "Object", "Function", "Date",
	"Math", "RegExp", "JSON", "Promise", "Symbol", "BigInt", "Map", "Set", "WeakMap", "WeakSet",
	"Reflect", "Proxy", "Intl", "undefined", "NaN", "Infinity", "process", "global",
	"globalThis", "window", "document", "navigator", "Buffer", "module", "exports", "require",
	"URL", "fetch", "Error", "TypeError", "AbortController", "TextEncoder", "TextDecoder",
	"crypto",
}

// Builtins returns the builtin names of the language, or nil for a language
// with no list.
func Builtins(c Code) []string {
	return builtins[c]
}

var builtinSets = func() map[Code]map[string]bool {
	sets := make(map[Code]map[string]bool, len(builtins))
	for code, names := range builtins {
		set := make(map[string]bool, len(names))
		for _, name := range names {
			set[name] = true
		}
		sets[code] = set
	}
	return sets
}()

// IsBuiltin reports whether the name is a builtin of the language.
func IsBuiltin(c Code, name string) bool {
	return builtinSets[c][name]
}
