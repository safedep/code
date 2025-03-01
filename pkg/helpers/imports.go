package helpers

import (
	"strings"

	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
)

func resolveImportContentsGeneric(importNode *ast.ImportNode) (core.ImportContents, error) {
	return core.ImportContents{
		ModuleName:  importNode.ModuleName(),
		ModuleItem:  importNode.ModuleItem(),
		ModuleAlias: importNode.ModuleAlias(),
	}, nil
}

func resolveImportContentsGo(importNode *ast.ImportNode) (core.ImportContents, error) {
	moduleName := strings.Trim(importNode.ModuleName(), `"`)
	moduleItem := strings.Trim(importNode.ModuleItem(), `"`)
	moduleAlias := strings.Trim(importNode.ModuleAlias(), `"`)

	moduleAliasParts := strings.Split(moduleAlias, "/")
	moduleAlias = moduleAliasParts[len(moduleAliasParts)-1]

	return core.ImportContents{
		ModuleName:  moduleName,
		ModuleItem:  moduleItem,
		ModuleAlias: moduleAlias,
	}, nil
}

var importContentResolvers = map[core.LanguageCode]func(importNode *ast.ImportNode) (core.ImportContents, error){
	core.LanguageCodePython:     resolveImportContentsGeneric,
	core.LanguageCodeGo:         resolveImportContentsGo,
	core.LanguageCodeJavascript: resolveImportContentsGeneric,
}

func ResolveImportContents(importNode *ast.ImportNode, language core.Language) (core.ImportContents, error) {
	resolver, ok := importContentResolvers[language.Meta().Code]
	if ok {
		return resolver(importNode)
	}
	return resolveImportContentsGeneric(importNode)
}
