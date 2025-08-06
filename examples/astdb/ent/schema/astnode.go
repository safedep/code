package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ASTNode represents a node in the Abstract Syntax Tree
type ASTNode struct {
	ent.Schema
}

// Fields of the ASTNode.
func (ASTNode) Fields() []ent.Field {
	return []ent.Field{
		field.Enum("node_type").Values(
			"module", "class", "function", "method", "variable", "import",
			"call", "assignment", "if_statement", "for_loop", "while_loop",
			"try_catch", "expression", "literal", "identifier",
		),
		field.String("name").Optional(),
		field.String("qualified_name").Optional(),
		field.Int("start_line").NonNegative(),
		field.Int("end_line").NonNegative(),
		field.Int("start_column").NonNegative(),
		field.Int("end_column").NonNegative(),
		field.Text("content").Optional(),
		field.String("tree_sitter_type").Optional(),
		field.JSON("metadata", map[string]any{}).Optional(),
		field.Time("created_at").Default(time.Now),
	}
}

// Edges of the ASTNode.
func (ASTNode) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("file", File.Type).Ref("ast_nodes").Unique(),
		edge.To("children", ASTNode.Type),
		edge.From("parent", ASTNode.Type).Ref("children").Unique(),
		edge.To("symbol", Symbol.Type).Unique(),
		edge.To("references", SymbolReference.Type).StorageKey(edge.Column("context_node_id")),
	}
}

// Indexes of the ASTNode.
func (ASTNode) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("node_type", "name"),
		index.Fields("qualified_name"),
		index.Fields("start_line", "end_line"),
	}
}
