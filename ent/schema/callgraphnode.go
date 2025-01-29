// graphnode.go

package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// CallgraphNode holds the schema definition for the CallgraphNode entity.
type CallgraphNode struct {
	ent.Schema
}

// Fields of the GraphNode.
func (CallgraphNode) Fields() []ent.Field {
	return []ent.Field{
		field.String("namespace").
			Unique().Immutable(),
	}
}

// Edges of the CallGraphNode.
func (CallgraphNode) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("calls_to", CallgraphNode.Type).From("called_by"),
	}
}
