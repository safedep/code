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
	Java: {
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
	},
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
