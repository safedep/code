package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// UsageEvidence holds the schema definition for the UsageEvidence entity.
type UsageEvidence struct {
	ent.Schema
}

// Fields of the UsageEvidence.
func (UsageEvidence) Fields() []ent.Field {
	return []ent.Field{
		field.String("PackageHint").Optional().Nillable(),
		field.String("ModuleName"),
		field.String("ModuleItem").Optional().Nillable(),
		field.String("ModuleAlias").Optional().Nillable(),
		field.Bool("IsWildCardUsage").Optional().Default(false),
		field.String("Identifier").Optional().Nillable(),
		field.String("UsageFilePath"),
		field.Uint("Line"),
	}
}

// Edges of the UsageEvidence.
func (UsageEvidence) Edges() []ent.Edge {
	return []ent.Edge{
		edge.
			To("used_in", CodeFile.Type).
			Unique().
			Required(),
	}
}
