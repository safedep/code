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

// IsECMAScript reports whether the language is JavaScript or TypeScript.
func (c Code) IsECMAScript() bool {
	return c == Javascript || c == Typescript
}
