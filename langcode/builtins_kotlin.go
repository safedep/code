package langcode

// kotlinBuiltins are the functions and the types of the standard library
// that a Kotlin file uses with no import, and the classes of java.lang.
var kotlinBuiltins = append([]string{
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
}, javaBuiltins...)
