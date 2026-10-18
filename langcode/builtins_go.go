package langcode

// goBuiltins are the predeclared functions, constants and types of Go.
var goBuiltins = []string{
	"panic", "recover", "make", "new", "len", "cap", "append", "copy", "delete", "close",
	"complex", "real", "imag", "print", "println", "iota", "true", "false", "nil",
	"min", "max", "clear",
	// A call of a predeclared type is a conversion, as string(b).
	"bool", "byte", "rune", "string", "error", "any", "comparable", "uintptr",
	"int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64",
	"float32", "float64", "complex64", "complex128",
}
