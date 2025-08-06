package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// CallRelationship represents function/method calls
type CallRelationship struct {
	ent.Schema
}

// Fields of the CallRelationship.
func (CallRelationship) Fields() []ent.Field {
	return []ent.Field{
		field.Int("call_site_line").NonNegative(),
		field.Int("call_site_column").NonNegative(),
		field.Enum("call_type").Values("direct", "method", "constructor", "dynamic", "async"),
		field.Bool("is_conditional").Default(false),
		field.JSON("arguments", []string{}).Optional(),
		field.Time("created_at").Default(time.Now),
	}
}

// Edges of the CallRelationship.
func (CallRelationship) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("caller", Symbol.Type).Ref("calls_made").Unique(),
		edge.From("callee", Symbol.Type).Ref("calls_received").Unique(),
		edge.From("call_site_file", File.Type).Ref("call_sites").Unique(),
	}
}

// Indexes of the CallRelationship.
func (CallRelationship) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("call_type"),
		index.Fields("call_site_line"),
	}
}
