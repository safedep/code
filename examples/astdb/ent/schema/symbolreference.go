package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// SymbolReference represents usage of symbols
type SymbolReference struct {
	ent.Schema
}

// Fields of the SymbolReference.
func (SymbolReference) Fields() []ent.Field {
	return []ent.Field{
		field.Int("line_number").NonNegative(),
		field.Int("column_number").NonNegative(),
		field.Enum("reference_type").Values("read", "write", "call", "declaration", "type_annotation"),
		field.String("context").Optional(),
		field.Time("created_at").Default(time.Now),
	}
}

// Edges of the SymbolReference.
func (SymbolReference) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("symbol", Symbol.Type).Ref("references").Unique(),
		edge.From("file", File.Type).Ref("symbol_references").Unique(),
		edge.From("context_node", ASTNode.Type).Ref("references").Unique(),
	}
}

// Indexes of the SymbolReference.
func (SymbolReference) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("reference_type"),
		index.Fields("line_number"),
	}
}
