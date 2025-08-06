package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// File represents a source code file
type File struct {
	ent.Schema
}

// Fields of the File.
func (File) Fields() []ent.Field {
	return []ent.Field{
		field.String("relative_path").NotEmpty(),
		field.String("absolute_path").NotEmpty(),
		field.Enum("language").Values("go", "python", "java", "javascript", "typescript"),
		field.String("content_hash").NotEmpty(),
		field.Int("size_bytes").NonNegative(),
		field.Int("line_count").NonNegative(),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

// Edges of the File.
func (File) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("project", Project.Type).Ref("files").Unique(),
		edge.To("ast_nodes", ASTNode.Type),
		edge.To("symbols", Symbol.Type),
		edge.To("imports", ImportStatement.Type),
		edge.To("call_sites", CallRelationship.Type).StorageKey(edge.Column("call_site_file_id")),
		edge.To("inheritance_sites", InheritanceRelationship.Type),
		edge.To("symbol_references", SymbolReference.Type),
	}
}

// Indexes of the File.
func (File) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("relative_path", "language"),
		index.Fields("content_hash"),
	}
}
