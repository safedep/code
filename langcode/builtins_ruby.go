package langcode

// rubyBuiltins are the methods of Kernel and the classes and constants of the core of Ruby.
var rubyBuiltins = []string{
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
}
