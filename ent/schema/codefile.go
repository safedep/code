package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// CodeFile holds the schema definition for the CodeFile entity.
type CodeFile struct {
	ent.Schema
}

// Fields of the CodeFile.
func (CodeFile) Fields() []ent.Field {
	return []ent.Field{
		field.
			String("FilePath").
			Unique().
			Immutable().
			StorageKey("FilePath"),
	}
}

func (CodeFile) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("usage_evidences", UsageEvidence.Type).
			Ref("used_in"),
	}
}
