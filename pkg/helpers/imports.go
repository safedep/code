package helpers

import (
	"strings"

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

	moduleAliasParts := strings.Split(moduleAlias, "/")
	moduleAlias = moduleAliasParts[len(moduleAliasParts)-1]

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
