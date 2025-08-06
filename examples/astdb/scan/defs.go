package scan

// Database enum constants for AST nodes, symbols, and relationships
// These constants match the values defined in ENT schemas and are used
// throughout the scanning process to ensure consistency and avoid typos.

// AST Node Types - semantic categorization of AST nodes
const (
	NodeTypeModule      = "module"
	NodeTypeClass       = "class"
	NodeTypeFunction    = "function"
	NodeTypeMethod      = "method"
	NodeTypeVariable    = "variable"
	NodeTypeImport      = "import"
	NodeTypeCall        = "call"
	NodeTypeAssignment  = "assignment"
	NodeTypeIfStatement = "if_statement"
	NodeTypeForLoop     = "for_loop"
	NodeTypeWhileLoop   = "while_loop"
	NodeTypeTryCatch    = "try_catch"
	NodeTypeExpression  = "expression"
	NodeTypeLiteral     = "literal"
	NodeTypeIdentifier  = "identifier"
)

// Symbol Types - types of symbols that can be stored in the database
const (
	SymbolTypeFunction  = "function"
	SymbolTypeClass     = "class"
	SymbolTypeMethod    = "method"
	SymbolTypeVariable  = "variable"
	SymbolTypeModule    = "module"
	SymbolTypeInterface = "interface"
	SymbolTypeEnum      = "enum"
)

// Scope Types - different scopes where symbols can exist
const (
	ScopeTypeGlobal   = "global"
	ScopeTypeClass    = "class"
	ScopeTypeFunction = "function"
	ScopeTypeBlock    = "block"
	ScopeTypeModule   = "module"
)

// Access Modifiers - visibility levels for symbols
const (
	AccessModifierPublic    = "public"
	AccessModifierPrivate   = "private"
	AccessModifierProtected = "protected"
	AccessModifierPackage   = "package"
)

// Import Types - different types of import statements
const (
	ImportTypeDefault   = "default"
	ImportTypeNamed     = "named"
	ImportTypeNamespace = "namespace"
	ImportTypeWildcard  = "wildcard"
)

// Call Types - different types of function/method calls
const (
	CallTypeDirect      = "direct"
	CallTypeMethod      = "method"
	CallTypeConstructor = "constructor"
	CallTypeDynamic     = "dynamic"
	CallTypeAsync       = "async"
)

// Relationship Types - types of inheritance relationships (matches CAF core/ast)
const (
	RelationshipTypeExtends    = "extends"
	RelationshipTypeImplements = "implements"
	RelationshipTypeInherits   = "inherits"
	RelationshipTypeMixin      = "mixin"
)

// Reference Types - different ways symbols can be referenced
const (
	ReferenceTypeRead           = "read"
	ReferenceTypeWrite          = "write"
	ReferenceTypeCall           = "call"
	ReferenceTypeDeclaration    = "declaration"
	ReferenceTypeTypeAnnotation = "type_annotation"
)

// File Languages - supported programming languages
const (
	LanguageGo         = "go"
	LanguagePython     = "python"
	LanguageJava       = "java"
	LanguageJavaScript = "javascript"
	LanguageTypeScript = "typescript"
	LanguageUnknown    = "unknown"
)

// Output Formats - supported output formats for CLI
const (
	OutputFormatText = "text"
	OutputFormatJSON = "json"
)

// Tree-Sitter Node Type Constants - common node types from Tree-Sitter parsers
const (
	// Module/Program level nodes
	TreeSitterModule     = "module"
	TreeSitterSourceFile = "source_file"
	TreeSitterProgram    = "program"

	// Class definition nodes
	TreeSitterClassDefinition  = "class_definition"
	TreeSitterClassDeclaration = "class_declaration"

	// Function definition nodes
	TreeSitterFunctionDefinition  = "function_definition"
	TreeSitterFunctionDeclaration = "function_declaration"
	TreeSitterMethodDefinition    = "method_definition"

	// Variable and assignment nodes
	TreeSitterVariableDeclaration = "variable_declaration"
	TreeSitterAssignment          = "assignment"
	TreeSitterAssignmentStatement = "assignment_statement"

	// Import statement nodes
	TreeSitterImportStatement     = "import_statement"
	TreeSitterImportDeclaration   = "import_declaration"
	TreeSitterFromImport          = "from_import"
	TreeSitterImportFromStatement = "import_from_statement"

	// Call expression nodes
	TreeSitterCallExpression = "call_expression"
	TreeSitterCall           = "call"

	// Control flow nodes
	TreeSitterIfStatement    = "if_statement"
	TreeSitterIf             = "if"
	TreeSitterForStatement   = "for_statement"
	TreeSitterFor            = "for"
	TreeSitterForInStatement = "for_in_statement"
	TreeSitterWhileStatement = "while_statement"
	TreeSitterWhile          = "while"
	TreeSitterTryStatement   = "try_statement"
	TreeSitterTry            = "try"
	TreeSitterExceptClause   = "except_clause"
	TreeSitterCatchClause    = "catch_clause"

	// Expression nodes
	TreeSitterExpression          = "expression"
	TreeSitterExpressionStatement = "expression_statement"

	// Literal nodes
	TreeSitterLiteral       = "literal"
	TreeSitterStringLiteral = "string_literal"
	TreeSitterNumber        = "number"
	TreeSitterInteger       = "integer"
	TreeSitterFloat         = "float"

	// Identifier nodes
	TreeSitterIdentifier = "identifier"
	TreeSitterName       = "name"
	TreeSitterLeft       = "left"
)

// GetSupportedLanguages returns a map of supported languages for validation
func GetSupportedLanguages() map[string]bool {
	return map[string]bool{
		LanguageGo:         true,
		LanguagePython:     true,
		LanguageJava:       true,
		LanguageJavaScript: true,
		LanguageTypeScript: true,
	}
}
