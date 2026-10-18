package langcode

// builtins are the names that a file of the language uses with no import or
// definition: functions, types and runtime globals. Each language holds its
// list in its own file.
var builtins = map[Code][]string{
	Python:     pythonBuiltins,
	Javascript: ecmascriptBuiltins,
	Typescript: ecmascriptBuiltins,
	Go:         goBuiltins,
	Java:       javaBuiltins,
	Kotlin:     kotlinBuiltins,
	CSharp:     csharpBuiltins,
	Rust:       rustBuiltins,
	Ruby:       rubyBuiltins,
	PHP:        phpBuiltins,
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

var javaLangSet = func() map[string]bool {
	set := make(map[string]bool, len(javaBuiltins))
	for _, name := range javaBuiltins {
		set[name] = true
	}
	return set
}()

// IsJavaLang reports whether the name is a class of java.lang, which a
// Java or a Kotlin file names with no import.
func IsJavaLang(name string) bool {
	return javaLangSet[name]
}
