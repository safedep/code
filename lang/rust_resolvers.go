package lang

import (
	"fmt"
	"unicode"
	"unicode/utf8"

	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
	sitter "github.com/smacker/go-tree-sitter"
)

type rustResolvers struct{}

var _ core.LanguageResolvers = (*rustResolvers)(nil)

var rustFunctionShapes = map[string]functionShape{
	"function_item": {name: field("name"), parameters: field("parameters"), body: field("body"), functionType: ast.FunctionTypeFunction},
}

// rustLocalPathRoots are path roots that never name an external crate, such
// as the standard crates and the primitive types in usize::MAX.
var rustLocalPathRoots = map[string]bool{
	"crate": true, "self": true, "super": true, "std": true, "core": true, "alloc": true,
	"bool": true, "char": true, "str": true, "f32": true, "f64": true,
	"i8": true, "i16": true, "i32": true, "i64": true, "i128": true, "isize": true,
	"u8": true, "u16": true, "u32": true, "u64": true, "u128": true, "usize": true,
}

// ResolveImports returns the use declarations and the extern crate
// declarations. A crate is also in scope without a use declaration, as in
// serde_json::to_string(&x), so the first path in a file that starts with a
// crate name is an import of that crate too.
func (r *rustResolvers) ResolveImports(tree core.ParseTree) ([]*ast.ImportNode, error) {
	data, err := tree.Data()
	if err != nil {
		return nil, fmt.Errorf("failed to get data from parse tree: %w", err)
	}

	var imports []*ast.ImportNode
	var pathRoots []*sitter.Node
	localNames := map[string]bool{}

	walkNamed(tree.Tree().RootNode(), func(node *sitter.Node) bool {
		switch node.Type() {
		case "use_declaration":
			imports = append(imports, rustUseImports(data, node.ChildByFieldName("argument"), nil, nil)...)
			return false
		case "extern_crate_declaration":
			name := node.ChildByFieldName("name")
			if name != nil {
				imports = append(imports, newModuleImport(data, name, nil, node.ChildByFieldName("alias"), false))
			}
			return false
		case "mod_item":
			if name := node.ChildByFieldName("name"); name != nil {
				localNames[name.Content(*data)] = true
			}
		case "scoped_identifier", "scoped_type_identifier":
			if root := node.ChildByFieldName("path"); root != nil && root.Type() == "identifier" {
				pathRoots = append(pathRoots, root)
			}
		}
		return true
	})

	for _, imp := range imports {
		localNames[rustImportKey(imp)] = true
	}

	for _, root := range pathRoots {
		name := root.Content(*data)
		if localNames[name] || rustLocalPathRoots[name] || !isRustCrateName(name) {
			continue
		}
		localNames[name] = true
		imports = append(imports, newModuleImport(data, root, nil, nil, false))
	}

	return imports, nil
}

// rustUseImports returns the imports of a use tree. The prefix is the path of
// the outer use list and the module of each import. The nested path is the
// path of a use list inside it. An import node has one node for each part, so
// the item of an import in a nested list drops the nested path: for
// a::{b::{c}} the import is c from a, not b::c from a.
func rustUseImports(data *[]byte, tree, prefix, nested *sitter.Node) []*ast.ImportNode {
	if tree == nil {
		return nil
	}

	switch tree.Type() {
	case "use_as_clause":
		return []*ast.ImportNode{rustPathImport(data, tree.ChildByFieldName("path"), prefix, nested, tree.ChildByFieldName("alias"))}
	case "use_wildcard":
		module := prefix
		if module == nil {
			module = firstNamedChildOfType(tree, "identifier", "scoped_identifier", "crate", "self", "super")
		}
		if module == nil {
			return nil
		}
		return []*ast.ImportNode{newModuleImport(data, module, nil, nil, true)}
	case "scoped_use_list":
		if prefix == nil {
			prefix = tree.ChildByFieldName("path")
		} else {
			nested = tree.ChildByFieldName("path")
		}
		return rustUseImports(data, tree.ChildByFieldName("list"), prefix, nested)
	case "use_list":
		var imports []*ast.ImportNode
		for _, child := range namedChildren(tree) {
			imports = append(imports, rustUseImports(data, child, prefix, nested)...)
		}
		return imports
	case "identifier", "scoped_identifier", "self", "crate", "super":
		return []*ast.ImportNode{rustPathImport(data, tree, prefix, nested, nil)}
	}

	return nil
}

// rustPathImport imports the item at path. With a prefix from a use list the
// prefix is the module and the path is the item. A self item imports the
// nested path, or else the prefix, as in std::{io::{self, Read}}. The last
// segment of the path is the name in scope unless an alias renames it.
func rustPathImport(data *[]byte, path, prefix, nested, alias *sitter.Node) *ast.ImportNode {
	module, item := path, (*sitter.Node)(nil)
	if prefix != nil {
		module, item = prefix, path
		if path.Type() == "self" {
			item, path = nested, nested
			if nested == nil {
				path = prefix
			}
		}
	}

	if alias == nil {
		alias = rustLastSegment(path)
	}

	return newModuleImport(data, module, item, alias, false)
}

func rustLastSegment(path *sitter.Node) *sitter.Node {
	if path.Type() == "scoped_identifier" {
		if name := path.ChildByFieldName("name"); name != nil {
			return name
		}
	}
	return path
}

func rustImportKey(imp *ast.ImportNode) string {
	if alias := imp.ModuleAlias(); alias != "" {
		return alias
	}
	return imp.ModuleName()
}

// isRustCrateName tells a crate name, such as serde_json, from a type, such as
// Vec, by the case of its first letter.
func isRustCrateName(name string) bool {
	first, _ := utf8.DecodeRuneInString(name)
	return unicode.IsLower(first)
}

func (r *rustResolvers) ResolveFunctions(tree core.ParseTree) ([]*ast.FunctionDeclarationNode, error) {
	return resolveFunctionsByShape(tree, rustFunctionShapes)
}
