package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ImportStatement represents import/export statements
type ImportStatement struct {
	ent.Schema
}

// Fields of the ImportStatement.
func (ImportStatement) Fields() []ent.Field {
	return []ent.Field{
		field.String("module_name").NotEmpty(),
		field.String("import_alias").Optional(),
		field.Enum("import_type").Values("default", "named", "namespace", "wildcard"),
		field.Int("line_number").NonNegative(),
		field.Bool("is_dynamic").Default(false),
		field.JSON("imported_names", []string{}).Optional(),
		field.Time("created_at").Default(time.Now),
	}
}

// Edges of the ImportStatement.
func (ImportStatement) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("file", File.Type).Ref("imports").Unique(),
		edge.From("imported_symbol", Symbol.Type).Ref("import_references").Unique(),
	}
}

// Indexes of the ImportStatement.
func (ImportStatement) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("module_name"),
		index.Fields("import_type"),
	}
}
