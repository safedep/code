// Package grammar names the version of the tree-sitter grammar of each
// language. It imports no tree-sitter package, so a store key builds with
// no CGO.
package grammar

import "github.com/safedep/code/langcode"

// treeSitter is the module path and the go.mod pin of the grammars. A test
// fails when go.mod pins another version, so a grammar upgrade also changes
// the store key and no index of the old grammar is read.
const treeSitter = "github.com/smacker/go-tree-sitter@v0.0.0-20240827094217-dd81d9e9be82"

var versions = map[langcode.Code]string{
	langcode.Python:     treeSitter,
	langcode.Javascript: treeSitter,
	langcode.Java:       treeSitter,
	langcode.Go:         treeSitter,
	langcode.Typescript: treeSitter,
	langcode.CSharp:     treeSitter,
	langcode.Rust:       treeSitter,
	langcode.PHP:        treeSitter,
	langcode.Ruby:       treeSitter,
	langcode.Kotlin:     treeSitter,
}

// Version returns the module and the version of the grammar of the
// language, as module@version. It returns "" for an unknown language.
func Version(code langcode.Code) string {
	return versions[code]
}
