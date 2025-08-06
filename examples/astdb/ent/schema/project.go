package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Project represents a scanned codebase
type Project struct {
	ent.Schema
}

// Fields of the Project.
func (Project) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").NotEmpty(),
		field.String("root_path").NotEmpty(),
		field.String("git_hash").Optional(),
		field.Time("scanned_at").Default(time.Now),
		field.JSON("metadata", map[string]any{}).Optional(),
	}
}

// Edges of the Project.
func (Project) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("files", File.Type),
	}
}
