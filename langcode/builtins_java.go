package langcode

// javaBuiltins are the public classes of java.lang, which a Java or a
// Kotlin file names with no import.
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
