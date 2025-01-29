// Code generated by ent, DO NOT EDIT.

package ent

import (
	"context"
	"errors"
	"fmt"

	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/sqlgraph"
	"entgo.io/ent/schema/field"
	"github.com/safedep/code/ent/codefile"
	"github.com/safedep/code/ent/predicate"
	"github.com/safedep/code/ent/usageevidence"
)

// CodeFileUpdate is the builder for updating CodeFile entities.
type CodeFileUpdate struct {
	config
	hooks    []Hook
	mutation *CodeFileMutation
}

// Where appends a list predicates to the CodeFileUpdate builder.
func (cfu *CodeFileUpdate) Where(ps ...predicate.CodeFile) *CodeFileUpdate {
	cfu.mutation.Where(ps...)
	return cfu
}

// AddUsageEvidenceIDs adds the "usage_evidences" edge to the UsageEvidence entity by IDs.
func (cfu *CodeFileUpdate) AddUsageEvidenceIDs(ids ...int) *CodeFileUpdate {
	cfu.mutation.AddUsageEvidenceIDs(ids...)
	return cfu
}

// AddUsageEvidences adds the "usage_evidences" edges to the UsageEvidence entity.
func (cfu *CodeFileUpdate) AddUsageEvidences(u ...*UsageEvidence) *CodeFileUpdate {
	ids := make([]int, len(u))
	for i := range u {
		ids[i] = u[i].ID
	}
	return cfu.AddUsageEvidenceIDs(ids...)
}

// Mutation returns the CodeFileMutation object of the builder.
func (cfu *CodeFileUpdate) Mutation() *CodeFileMutation {
	return cfu.mutation
}

// ClearUsageEvidences clears all "usage_evidences" edges to the UsageEvidence entity.
func (cfu *CodeFileUpdate) ClearUsageEvidences() *CodeFileUpdate {
	cfu.mutation.ClearUsageEvidences()
	return cfu
}

// RemoveUsageEvidenceIDs removes the "usage_evidences" edge to UsageEvidence entities by IDs.
func (cfu *CodeFileUpdate) RemoveUsageEvidenceIDs(ids ...int) *CodeFileUpdate {
	cfu.mutation.RemoveUsageEvidenceIDs(ids...)
	return cfu
}

// RemoveUsageEvidences removes "usage_evidences" edges to UsageEvidence entities.
func (cfu *CodeFileUpdate) RemoveUsageEvidences(u ...*UsageEvidence) *CodeFileUpdate {
	ids := make([]int, len(u))
	for i := range u {
		ids[i] = u[i].ID
	}
	return cfu.RemoveUsageEvidenceIDs(ids...)
}

// Save executes the query and returns the number of nodes affected by the update operation.
func (cfu *CodeFileUpdate) Save(ctx context.Context) (int, error) {
	return withHooks(ctx, cfu.sqlSave, cfu.mutation, cfu.hooks)
}

// SaveX is like Save, but panics if an error occurs.
func (cfu *CodeFileUpdate) SaveX(ctx context.Context) int {
	affected, err := cfu.Save(ctx)
	if err != nil {
		panic(err)
	}
	return affected
}

// Exec executes the query.
func (cfu *CodeFileUpdate) Exec(ctx context.Context) error {
	_, err := cfu.Save(ctx)
	return err
}

// ExecX is like Exec, but panics if an error occurs.
func (cfu *CodeFileUpdate) ExecX(ctx context.Context) {
	if err := cfu.Exec(ctx); err != nil {
		panic(err)
	}
}

func (cfu *CodeFileUpdate) sqlSave(ctx context.Context) (n int, err error) {
	_spec := sqlgraph.NewUpdateSpec(codefile.Table, codefile.Columns, sqlgraph.NewFieldSpec(codefile.FieldID, field.TypeInt))
	if ps := cfu.mutation.predicates; len(ps) > 0 {
		_spec.Predicate = func(selector *sql.Selector) {
			for i := range ps {
				ps[i](selector)
			}
		}
	}
	if cfu.mutation.UsageEvidencesCleared() {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.O2M,
			Inverse: true,
			Table:   codefile.UsageEvidencesTable,
			Columns: []string{codefile.UsageEvidencesColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(usageevidence.FieldID, field.TypeInt),
			},
		}
		_spec.Edges.Clear = append(_spec.Edges.Clear, edge)
	}
	if nodes := cfu.mutation.RemovedUsageEvidencesIDs(); len(nodes) > 0 && !cfu.mutation.UsageEvidencesCleared() {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.O2M,
			Inverse: true,
			Table:   codefile.UsageEvidencesTable,
			Columns: []string{codefile.UsageEvidencesColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(usageevidence.FieldID, field.TypeInt),
			},
		}
		for _, k := range nodes {
			edge.Target.Nodes = append(edge.Target.Nodes, k)
		}
		_spec.Edges.Clear = append(_spec.Edges.Clear, edge)
	}
	if nodes := cfu.mutation.UsageEvidencesIDs(); len(nodes) > 0 {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.O2M,
			Inverse: true,
			Table:   codefile.UsageEvidencesTable,
			Columns: []string{codefile.UsageEvidencesColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(usageevidence.FieldID, field.TypeInt),
			},
		}
		for _, k := range nodes {
			edge.Target.Nodes = append(edge.Target.Nodes, k)
		}
		_spec.Edges.Add = append(_spec.Edges.Add, edge)
	}
	if n, err = sqlgraph.UpdateNodes(ctx, cfu.driver, _spec); err != nil {
		if _, ok := err.(*sqlgraph.NotFoundError); ok {
			err = &NotFoundError{codefile.Label}
		} else if sqlgraph.IsConstraintError(err) {
			err = &ConstraintError{msg: err.Error(), wrap: err}
		}
		return 0, err
	}
	cfu.mutation.done = true
	return n, nil
}

// CodeFileUpdateOne is the builder for updating a single CodeFile entity.
type CodeFileUpdateOne struct {
	config
	fields   []string
	hooks    []Hook
	mutation *CodeFileMutation
}

// AddUsageEvidenceIDs adds the "usage_evidences" edge to the UsageEvidence entity by IDs.
func (cfuo *CodeFileUpdateOne) AddUsageEvidenceIDs(ids ...int) *CodeFileUpdateOne {
	cfuo.mutation.AddUsageEvidenceIDs(ids...)
	return cfuo
}

// AddUsageEvidences adds the "usage_evidences" edges to the UsageEvidence entity.
func (cfuo *CodeFileUpdateOne) AddUsageEvidences(u ...*UsageEvidence) *CodeFileUpdateOne {
	ids := make([]int, len(u))
	for i := range u {
		ids[i] = u[i].ID
	}
	return cfuo.AddUsageEvidenceIDs(ids...)
}

// Mutation returns the CodeFileMutation object of the builder.
func (cfuo *CodeFileUpdateOne) Mutation() *CodeFileMutation {
	return cfuo.mutation
}

// ClearUsageEvidences clears all "usage_evidences" edges to the UsageEvidence entity.
func (cfuo *CodeFileUpdateOne) ClearUsageEvidences() *CodeFileUpdateOne {
	cfuo.mutation.ClearUsageEvidences()
	return cfuo
}

// RemoveUsageEvidenceIDs removes the "usage_evidences" edge to UsageEvidence entities by IDs.
func (cfuo *CodeFileUpdateOne) RemoveUsageEvidenceIDs(ids ...int) *CodeFileUpdateOne {
	cfuo.mutation.RemoveUsageEvidenceIDs(ids...)
	return cfuo
}

// RemoveUsageEvidences removes "usage_evidences" edges to UsageEvidence entities.
func (cfuo *CodeFileUpdateOne) RemoveUsageEvidences(u ...*UsageEvidence) *CodeFileUpdateOne {
	ids := make([]int, len(u))
	for i := range u {
		ids[i] = u[i].ID
	}
	return cfuo.RemoveUsageEvidenceIDs(ids...)
}

// Where appends a list predicates to the CodeFileUpdate builder.
func (cfuo *CodeFileUpdateOne) Where(ps ...predicate.CodeFile) *CodeFileUpdateOne {
	cfuo.mutation.Where(ps...)
	return cfuo
}

// Select allows selecting one or more fields (columns) of the returned entity.
// The default is selecting all fields defined in the entity schema.
func (cfuo *CodeFileUpdateOne) Select(field string, fields ...string) *CodeFileUpdateOne {
	cfuo.fields = append([]string{field}, fields...)
	return cfuo
}

// Save executes the query and returns the updated CodeFile entity.
func (cfuo *CodeFileUpdateOne) Save(ctx context.Context) (*CodeFile, error) {
	return withHooks(ctx, cfuo.sqlSave, cfuo.mutation, cfuo.hooks)
}

// SaveX is like Save, but panics if an error occurs.
func (cfuo *CodeFileUpdateOne) SaveX(ctx context.Context) *CodeFile {
	node, err := cfuo.Save(ctx)
	if err != nil {
		panic(err)
	}
	return node
}

// Exec executes the query on the entity.
func (cfuo *CodeFileUpdateOne) Exec(ctx context.Context) error {
	_, err := cfuo.Save(ctx)
	return err
}

// ExecX is like Exec, but panics if an error occurs.
func (cfuo *CodeFileUpdateOne) ExecX(ctx context.Context) {
	if err := cfuo.Exec(ctx); err != nil {
		panic(err)
	}
}

func (cfuo *CodeFileUpdateOne) sqlSave(ctx context.Context) (_node *CodeFile, err error) {
	_spec := sqlgraph.NewUpdateSpec(codefile.Table, codefile.Columns, sqlgraph.NewFieldSpec(codefile.FieldID, field.TypeInt))
	id, ok := cfuo.mutation.ID()
	if !ok {
		return nil, &ValidationError{Name: "id", err: errors.New(`ent: missing "CodeFile.id" for update`)}
	}
	_spec.Node.ID.Value = id
	if fields := cfuo.fields; len(fields) > 0 {
		_spec.Node.Columns = make([]string, 0, len(fields))
		_spec.Node.Columns = append(_spec.Node.Columns, codefile.FieldID)
		for _, f := range fields {
			if !codefile.ValidColumn(f) {
				return nil, &ValidationError{Name: f, err: fmt.Errorf("ent: invalid field %q for query", f)}
			}
			if f != codefile.FieldID {
				_spec.Node.Columns = append(_spec.Node.Columns, f)
			}
		}
	}
	if ps := cfuo.mutation.predicates; len(ps) > 0 {
		_spec.Predicate = func(selector *sql.Selector) {
			for i := range ps {
				ps[i](selector)
			}
		}
	}
	if cfuo.mutation.UsageEvidencesCleared() {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.O2M,
			Inverse: true,
			Table:   codefile.UsageEvidencesTable,
			Columns: []string{codefile.UsageEvidencesColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(usageevidence.FieldID, field.TypeInt),
			},
		}
		_spec.Edges.Clear = append(_spec.Edges.Clear, edge)
	}
	if nodes := cfuo.mutation.RemovedUsageEvidencesIDs(); len(nodes) > 0 && !cfuo.mutation.UsageEvidencesCleared() {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.O2M,
			Inverse: true,
			Table:   codefile.UsageEvidencesTable,
			Columns: []string{codefile.UsageEvidencesColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(usageevidence.FieldID, field.TypeInt),
			},
		}
		for _, k := range nodes {
			edge.Target.Nodes = append(edge.Target.Nodes, k)
		}
		_spec.Edges.Clear = append(_spec.Edges.Clear, edge)
	}
	if nodes := cfuo.mutation.UsageEvidencesIDs(); len(nodes) > 0 {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.O2M,
			Inverse: true,
			Table:   codefile.UsageEvidencesTable,
			Columns: []string{codefile.UsageEvidencesColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(usageevidence.FieldID, field.TypeInt),
			},
		}
		for _, k := range nodes {
			edge.Target.Nodes = append(edge.Target.Nodes, k)
		}
		_spec.Edges.Add = append(_spec.Edges.Add, edge)
	}
	_node = &CodeFile{config: cfuo.config}
	_spec.Assign = _node.assignValues
	_spec.ScanValues = _node.scanValues
	if err = sqlgraph.UpdateNode(ctx, cfuo.driver, _spec); err != nil {
		if _, ok := err.(*sqlgraph.NotFoundError); ok {
			err = &NotFoundError{codefile.Label}
		} else if sqlgraph.IsConstraintError(err) {
			err = &ConstraintError{msg: err.Error(), wrap: err}
		}
		return nil, err
	}
	cfuo.mutation.done = true
	return _node, nil
}
