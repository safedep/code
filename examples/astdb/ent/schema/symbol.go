package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Symbol represents a symbol (function, class, variable) in the code
type Symbol struct {
	ent.Schema
}

// Fields of the Symbol.
func (Symbol) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").NotEmpty(),
		field.String("qualified_name").NotEmpty(),
		field.Enum("symbol_type").Values("function", "class", "method", "variable", "module", "interface", "enum"),
		field.Enum("scope_type").Values("global", "class", "function", "block", "module"),
		field.Enum("access_modifier").Values("public", "private", "protected", "package").Optional(),
		field.Bool("is_static").Default(false),
		field.Bool("is_abstract").Default(false),
		field.Bool("is_async").Default(false),
		field.Int("line_number").NonNegative(),
		field.Int("column_number").NonNegative(),
		field.JSON("metadata", map[string]any{}).Optional(),
		field.Time("created_at").Default(time.Now),
	}
}

// Edges of the Symbol.
func (Symbol) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("file", File.Type).Ref("symbols").Unique(),
		edge.From("ast_node", ASTNode.Type).Ref("symbol").Unique(),
		edge.To("calls_made", CallRelationship.Type).StorageKey(edge.Column("caller_id")),
		edge.To("calls_received", CallRelationship.Type).StorageKey(edge.Column("callee_id")),
		edge.To("references", SymbolReference.Type),
		edge.To("child_classes", InheritanceRelationship.Type).StorageKey(edge.Column("parent_id")),
		edge.To("parent_classes", InheritanceRelationship.Type).StorageKey(edge.Column("child_id")),
		edge.To("import_references", ImportStatement.Type).StorageKey(edge.Column("imported_symbol_id")),
	}
}

// Indexes of the Symbol.
func (Symbol) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("qualified_name"),
		index.Fields("symbol_type", "scope_type"),
		index.Fields("name").StorageKey("idx_symbol_name"),
	}
}
