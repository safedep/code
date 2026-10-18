// Package langcode names the languages that safedep/code parses. It imports
// no tree-sitter package, so a package that builds with no CGO can use it.
package langcode

// Code identifies a language.
type Code string

const (
	Python     Code = "python"
	Javascript Code = "javascript"
	Java       Code = "java"
	Go         Code = "go"
	Typescript Code = "typescript"
	CSharp     Code = "csharp"
	Rust       Code = "rust"
	PHP        Code = "php"
	Ruby       Code = "ruby"
	Kotlin     Code = "kotlin"
)

// All lists every language code in a fixed order.
func All() []Code {
	return []Code{Python, Javascript, Java, Go, Typescript, CSharp, Rust, PHP, Ruby, Kotlin}
}

// Family joins the languages whose files call each other and share their
// libraries: TypeScript with JavaScript, Kotlin with Java. A language with
// no such partner is its own family.
func (c Code) Family() Code {
	switch c {
	case Typescript:
		return Javascript
	case Kotlin:
		return Java
	}
	return c
}

// IsECMAScript reports whether the language is JavaScript or TypeScript.
func (c Code) IsECMAScript() bool {
	return c.Family() == Javascript
}

// ModuleSeparator is the separator of a module path of the language, as the
// dot of os.path or the :: of std::io.
func (c Code) ModuleSeparator() string {
	switch c {
	case Python, Java, Kotlin, CSharp:
		return "."
	case PHP:
		return `\`
	case Rust:
		return "::"
	}
	return "/"
}
