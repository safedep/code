package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// InheritanceRelationship represents class inheritance
type InheritanceRelationship struct {
	ent.Schema
}

// Fields of the InheritanceRelationship.
func (InheritanceRelationship) Fields() []ent.Field {
	return []ent.Field{
		// Matches ast.RelationshipType from core/ast/inheritance.go
		field.Enum("relationship_type").Values("extends", "implements", "inherits", "mixin"),
		field.Int("line_number").NonNegative(),
		field.Bool("is_direct_inheritance").Default(true), // Direct vs computed ancestry
		field.Int("inheritance_depth").Default(1),         // 1 for direct parent, 2 for grandparent, etc.
		field.String("module_name").Optional(),            // Module/package context
		field.Time("created_at").Default(time.Now),
	}
}

// Edges of the InheritanceRelationship.
func (InheritanceRelationship) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("child", Symbol.Type).Ref("parent_classes").Unique(),
		edge.From("parent", Symbol.Type).Ref("child_classes").Unique(),
		edge.From("file", File.Type).Ref("inheritance_sites").Unique(),
	}
}

// Indexes of the InheritanceRelationship.
func (InheritanceRelationship) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("relationship_type"),
		index.Fields("inheritance_depth"),
	}
}
