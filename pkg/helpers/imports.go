package helpers

import (
	"path"
	"strconv"
	"strings"
	"unicode"

	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
)

// importContents represents the contents of an import node
// It represents the ready to use content of import nodes which may
// exhibit different forms in different languages
type importContents struct {
	ModuleName  string
	ModuleItem  string
	ModuleAlias string
}

func resolveImportContentsGeneric(importNode *ast.ImportNode) (importContents, error) {
	return importContents{
		ModuleName:  importNode.ModuleName(),
		ModuleItem:  importNode.ModuleItem(),
		ModuleAlias: importNode.ModuleAlias(),
	}, nil
}

func resolveImportContentsGo(importNode *ast.ImportNode) (importContents, error) {
	moduleName := strings.Trim(importNode.ModuleName(), `"`)
	moduleItem := strings.Trim(importNode.ModuleItem(), `"`)
	moduleAlias := strings.Trim(importNode.ModuleAlias(), `"`)

	// An import with no alias has the module as its alias node
	if moduleAlias == moduleName {
		moduleAlias = GoPackageName(moduleName)
	}

	return importContents{
		ModuleName:  moduleName,
		ModuleItem:  moduleItem,
		ModuleAlias: moduleAlias,
	}, nil
}

var importContentResolvers = map[core.LanguageCode]func(importNode *ast.ImportNode) (importContents, error){
	core.LanguageCodePython:     resolveImportContentsGeneric,
	core.LanguageCodeGo:         resolveImportContentsGo,
	core.LanguageCodeJavascript: resolveImportContentsGeneric,
}

func ResolveImportContents(importNode *ast.ImportNode, language core.Language) (importContents, error) {
	resolver, ok := importContentResolvers[language.Meta().Code]
	if ok {
		return resolver(importNode)
	}
	return resolveImportContentsGeneric(importNode)
}

// GoPackageName returns the name that a Go import path brings into scope
// when the import has no alias. It follows goimports: a major version
// element names the element before it, as in github.com/golang-jwt/jwt/v5,
// a go- prefix goes, as in go-openai, and the name ends at the first
// character that an identifier cannot hold, as in yaml.v3 or openai-go.
func GoPackageName(importPath string) string {
	importPath = strings.Trim(importPath, `"`)
	base := path.Base(importPath)
	if len(base) > 1 && base[0] == 'v' {
		if _, err := strconv.Atoi(base[1:]); err == nil {
			if dir := path.Dir(importPath); dir != "." {
				base = path.Base(dir)
			}
		}
	}
	base = strings.TrimPrefix(base, "go-")
	if i := strings.IndexFunc(base, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
	}); i >= 0 {
		base = base[:i]
	}
	return base
}
