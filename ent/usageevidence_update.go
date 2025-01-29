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

// UsageEvidenceUpdate is the builder for updating UsageEvidence entities.
type UsageEvidenceUpdate struct {
	config
	hooks    []Hook
	mutation *UsageEvidenceMutation
}

// Where appends a list predicates to the UsageEvidenceUpdate builder.
func (ueu *UsageEvidenceUpdate) Where(ps ...predicate.UsageEvidence) *UsageEvidenceUpdate {
	ueu.mutation.Where(ps...)
	return ueu
}

// SetPackageHint sets the "PackageHint" field.
func (ueu *UsageEvidenceUpdate) SetPackageHint(s string) *UsageEvidenceUpdate {
	ueu.mutation.SetPackageHint(s)
	return ueu
}

// SetNillablePackageHint sets the "PackageHint" field if the given value is not nil.
func (ueu *UsageEvidenceUpdate) SetNillablePackageHint(s *string) *UsageEvidenceUpdate {
	if s != nil {
		ueu.SetPackageHint(*s)
	}
	return ueu
}

// ClearPackageHint clears the value of the "PackageHint" field.
func (ueu *UsageEvidenceUpdate) ClearPackageHint() *UsageEvidenceUpdate {
	ueu.mutation.ClearPackageHint()
	return ueu
}

// SetModuleName sets the "ModuleName" field.
func (ueu *UsageEvidenceUpdate) SetModuleName(s string) *UsageEvidenceUpdate {
	ueu.mutation.SetModuleName(s)
	return ueu
}

// SetNillableModuleName sets the "ModuleName" field if the given value is not nil.
func (ueu *UsageEvidenceUpdate) SetNillableModuleName(s *string) *UsageEvidenceUpdate {
	if s != nil {
		ueu.SetModuleName(*s)
	}
	return ueu
}

// SetModuleItem sets the "ModuleItem" field.
func (ueu *UsageEvidenceUpdate) SetModuleItem(s string) *UsageEvidenceUpdate {
	ueu.mutation.SetModuleItem(s)
	return ueu
}

// SetNillableModuleItem sets the "ModuleItem" field if the given value is not nil.
func (ueu *UsageEvidenceUpdate) SetNillableModuleItem(s *string) *UsageEvidenceUpdate {
	if s != nil {
		ueu.SetModuleItem(*s)
	}
	return ueu
}

// ClearModuleItem clears the value of the "ModuleItem" field.
func (ueu *UsageEvidenceUpdate) ClearModuleItem() *UsageEvidenceUpdate {
	ueu.mutation.ClearModuleItem()
	return ueu
}

// SetModuleAlias sets the "ModuleAlias" field.
func (ueu *UsageEvidenceUpdate) SetModuleAlias(s string) *UsageEvidenceUpdate {
	ueu.mutation.SetModuleAlias(s)
	return ueu
}

// SetNillableModuleAlias sets the "ModuleAlias" field if the given value is not nil.
func (ueu *UsageEvidenceUpdate) SetNillableModuleAlias(s *string) *UsageEvidenceUpdate {
	if s != nil {
		ueu.SetModuleAlias(*s)
	}
	return ueu
}

// ClearModuleAlias clears the value of the "ModuleAlias" field.
func (ueu *UsageEvidenceUpdate) ClearModuleAlias() *UsageEvidenceUpdate {
	ueu.mutation.ClearModuleAlias()
	return ueu
}

// SetIsWildCardUsage sets the "IsWildCardUsage" field.
func (ueu *UsageEvidenceUpdate) SetIsWildCardUsage(b bool) *UsageEvidenceUpdate {
	ueu.mutation.SetIsWildCardUsage(b)
	return ueu
}

// SetNillableIsWildCardUsage sets the "IsWildCardUsage" field if the given value is not nil.
func (ueu *UsageEvidenceUpdate) SetNillableIsWildCardUsage(b *bool) *UsageEvidenceUpdate {
	if b != nil {
		ueu.SetIsWildCardUsage(*b)
	}
	return ueu
}

// ClearIsWildCardUsage clears the value of the "IsWildCardUsage" field.
func (ueu *UsageEvidenceUpdate) ClearIsWildCardUsage() *UsageEvidenceUpdate {
	ueu.mutation.ClearIsWildCardUsage()
	return ueu
}

// SetIdentifier sets the "Identifier" field.
func (ueu *UsageEvidenceUpdate) SetIdentifier(s string) *UsageEvidenceUpdate {
	ueu.mutation.SetIdentifier(s)
	return ueu
}

// SetNillableIdentifier sets the "Identifier" field if the given value is not nil.
func (ueu *UsageEvidenceUpdate) SetNillableIdentifier(s *string) *UsageEvidenceUpdate {
	if s != nil {
		ueu.SetIdentifier(*s)
	}
	return ueu
}

// ClearIdentifier clears the value of the "Identifier" field.
func (ueu *UsageEvidenceUpdate) ClearIdentifier() *UsageEvidenceUpdate {
	ueu.mutation.ClearIdentifier()
	return ueu
}

// SetUsageFilePath sets the "UsageFilePath" field.
func (ueu *UsageEvidenceUpdate) SetUsageFilePath(s string) *UsageEvidenceUpdate {
	ueu.mutation.SetUsageFilePath(s)
	return ueu
}

// SetNillableUsageFilePath sets the "UsageFilePath" field if the given value is not nil.
func (ueu *UsageEvidenceUpdate) SetNillableUsageFilePath(s *string) *UsageEvidenceUpdate {
	if s != nil {
		ueu.SetUsageFilePath(*s)
	}
	return ueu
}

// SetLine sets the "Line" field.
func (ueu *UsageEvidenceUpdate) SetLine(u uint) *UsageEvidenceUpdate {
	ueu.mutation.ResetLine()
	ueu.mutation.SetLine(u)
	return ueu
}

// SetNillableLine sets the "Line" field if the given value is not nil.
func (ueu *UsageEvidenceUpdate) SetNillableLine(u *uint) *UsageEvidenceUpdate {
	if u != nil {
		ueu.SetLine(*u)
	}
	return ueu
}

// AddLine adds u to the "Line" field.
func (ueu *UsageEvidenceUpdate) AddLine(u int) *UsageEvidenceUpdate {
	ueu.mutation.AddLine(u)
	return ueu
}

// SetUsedInID sets the "used_in" edge to the CodeFile entity by ID.
func (ueu *UsageEvidenceUpdate) SetUsedInID(id int) *UsageEvidenceUpdate {
	ueu.mutation.SetUsedInID(id)
	return ueu
}

// SetUsedIn sets the "used_in" edge to the CodeFile entity.
func (ueu *UsageEvidenceUpdate) SetUsedIn(c *CodeFile) *UsageEvidenceUpdate {
	return ueu.SetUsedInID(c.ID)
}

// Mutation returns the UsageEvidenceMutation object of the builder.
func (ueu *UsageEvidenceUpdate) Mutation() *UsageEvidenceMutation {
	return ueu.mutation
}

// ClearUsedIn clears the "used_in" edge to the CodeFile entity.
func (ueu *UsageEvidenceUpdate) ClearUsedIn() *UsageEvidenceUpdate {
	ueu.mutation.ClearUsedIn()
	return ueu
}

// Save executes the query and returns the number of nodes affected by the update operation.
func (ueu *UsageEvidenceUpdate) Save(ctx context.Context) (int, error) {
	return withHooks(ctx, ueu.sqlSave, ueu.mutation, ueu.hooks)
}

// SaveX is like Save, but panics if an error occurs.
func (ueu *UsageEvidenceUpdate) SaveX(ctx context.Context) int {
	affected, err := ueu.Save(ctx)
	if err != nil {
		panic(err)
	}
	return affected
}

// Exec executes the query.
func (ueu *UsageEvidenceUpdate) Exec(ctx context.Context) error {
	_, err := ueu.Save(ctx)
	return err
}

// ExecX is like Exec, but panics if an error occurs.
func (ueu *UsageEvidenceUpdate) ExecX(ctx context.Context) {
	if err := ueu.Exec(ctx); err != nil {
		panic(err)
	}
}

// check runs all checks and user-defined validators on the builder.
func (ueu *UsageEvidenceUpdate) check() error {
	if ueu.mutation.UsedInCleared() && len(ueu.mutation.UsedInIDs()) > 0 {
		return errors.New(`ent: clearing a required unique edge "UsageEvidence.used_in"`)
	}
	return nil
}

func (ueu *UsageEvidenceUpdate) sqlSave(ctx context.Context) (n int, err error) {
	if err := ueu.check(); err != nil {
		return n, err
	}
	_spec := sqlgraph.NewUpdateSpec(usageevidence.Table, usageevidence.Columns, sqlgraph.NewFieldSpec(usageevidence.FieldID, field.TypeInt))
	if ps := ueu.mutation.predicates; len(ps) > 0 {
		_spec.Predicate = func(selector *sql.Selector) {
			for i := range ps {
				ps[i](selector)
			}
		}
	}
	if value, ok := ueu.mutation.PackageHint(); ok {
		_spec.SetField(usageevidence.FieldPackageHint, field.TypeString, value)
	}
	if ueu.mutation.PackageHintCleared() {
		_spec.ClearField(usageevidence.FieldPackageHint, field.TypeString)
	}
	if value, ok := ueu.mutation.ModuleName(); ok {
		_spec.SetField(usageevidence.FieldModuleName, field.TypeString, value)
	}
	if value, ok := ueu.mutation.ModuleItem(); ok {
		_spec.SetField(usageevidence.FieldModuleItem, field.TypeString, value)
	}
	if ueu.mutation.ModuleItemCleared() {
		_spec.ClearField(usageevidence.FieldModuleItem, field.TypeString)
	}
	if value, ok := ueu.mutation.ModuleAlias(); ok {
		_spec.SetField(usageevidence.FieldModuleAlias, field.TypeString, value)
	}
	if ueu.mutation.ModuleAliasCleared() {
		_spec.ClearField(usageevidence.FieldModuleAlias, field.TypeString)
	}
	if value, ok := ueu.mutation.IsWildCardUsage(); ok {
		_spec.SetField(usageevidence.FieldIsWildCardUsage, field.TypeBool, value)
	}
	if ueu.mutation.IsWildCardUsageCleared() {
		_spec.ClearField(usageevidence.FieldIsWildCardUsage, field.TypeBool)
	}
	if value, ok := ueu.mutation.Identifier(); ok {
		_spec.SetField(usageevidence.FieldIdentifier, field.TypeString, value)
	}
	if ueu.mutation.IdentifierCleared() {
		_spec.ClearField(usageevidence.FieldIdentifier, field.TypeString)
	}
	if value, ok := ueu.mutation.UsageFilePath(); ok {
		_spec.SetField(usageevidence.FieldUsageFilePath, field.TypeString, value)
	}
	if value, ok := ueu.mutation.Line(); ok {
		_spec.SetField(usageevidence.FieldLine, field.TypeUint, value)
	}
	if value, ok := ueu.mutation.AddedLine(); ok {
		_spec.AddField(usageevidence.FieldLine, field.TypeUint, value)
	}
	if ueu.mutation.UsedInCleared() {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.M2O,
			Inverse: false,
			Table:   usageevidence.UsedInTable,
			Columns: []string{usageevidence.UsedInColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(codefile.FieldID, field.TypeInt),
			},
		}
		_spec.Edges.Clear = append(_spec.Edges.Clear, edge)
	}
	if nodes := ueu.mutation.UsedInIDs(); len(nodes) > 0 {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.M2O,
			Inverse: false,
			Table:   usageevidence.UsedInTable,
			Columns: []string{usageevidence.UsedInColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(codefile.FieldID, field.TypeInt),
			},
		}
		for _, k := range nodes {
			edge.Target.Nodes = append(edge.Target.Nodes, k)
		}
		_spec.Edges.Add = append(_spec.Edges.Add, edge)
	}
	if n, err = sqlgraph.UpdateNodes(ctx, ueu.driver, _spec); err != nil {
		if _, ok := err.(*sqlgraph.NotFoundError); ok {
			err = &NotFoundError{usageevidence.Label}
		} else if sqlgraph.IsConstraintError(err) {
			err = &ConstraintError{msg: err.Error(), wrap: err}
		}
		return 0, err
	}
	ueu.mutation.done = true
	return n, nil
}

// UsageEvidenceUpdateOne is the builder for updating a single UsageEvidence entity.
type UsageEvidenceUpdateOne struct {
	config
	fields   []string
	hooks    []Hook
	mutation *UsageEvidenceMutation
}

// SetPackageHint sets the "PackageHint" field.
func (ueuo *UsageEvidenceUpdateOne) SetPackageHint(s string) *UsageEvidenceUpdateOne {
	ueuo.mutation.SetPackageHint(s)
	return ueuo
}

// SetNillablePackageHint sets the "PackageHint" field if the given value is not nil.
func (ueuo *UsageEvidenceUpdateOne) SetNillablePackageHint(s *string) *UsageEvidenceUpdateOne {
	if s != nil {
		ueuo.SetPackageHint(*s)
	}
	return ueuo
}

// ClearPackageHint clears the value of the "PackageHint" field.
func (ueuo *UsageEvidenceUpdateOne) ClearPackageHint() *UsageEvidenceUpdateOne {
	ueuo.mutation.ClearPackageHint()
	return ueuo
}

// SetModuleName sets the "ModuleName" field.
func (ueuo *UsageEvidenceUpdateOne) SetModuleName(s string) *UsageEvidenceUpdateOne {
	ueuo.mutation.SetModuleName(s)
	return ueuo
}

// SetNillableModuleName sets the "ModuleName" field if the given value is not nil.
func (ueuo *UsageEvidenceUpdateOne) SetNillableModuleName(s *string) *UsageEvidenceUpdateOne {
	if s != nil {
		ueuo.SetModuleName(*s)
	}
	return ueuo
}

// SetModuleItem sets the "ModuleItem" field.
func (ueuo *UsageEvidenceUpdateOne) SetModuleItem(s string) *UsageEvidenceUpdateOne {
	ueuo.mutation.SetModuleItem(s)
	return ueuo
}

// SetNillableModuleItem sets the "ModuleItem" field if the given value is not nil.
func (ueuo *UsageEvidenceUpdateOne) SetNillableModuleItem(s *string) *UsageEvidenceUpdateOne {
	if s != nil {
		ueuo.SetModuleItem(*s)
	}
	return ueuo
}

// ClearModuleItem clears the value of the "ModuleItem" field.
func (ueuo *UsageEvidenceUpdateOne) ClearModuleItem() *UsageEvidenceUpdateOne {
	ueuo.mutation.ClearModuleItem()
	return ueuo
}

// SetModuleAlias sets the "ModuleAlias" field.
func (ueuo *UsageEvidenceUpdateOne) SetModuleAlias(s string) *UsageEvidenceUpdateOne {
	ueuo.mutation.SetModuleAlias(s)
	return ueuo
}

// SetNillableModuleAlias sets the "ModuleAlias" field if the given value is not nil.
func (ueuo *UsageEvidenceUpdateOne) SetNillableModuleAlias(s *string) *UsageEvidenceUpdateOne {
	if s != nil {
		ueuo.SetModuleAlias(*s)
	}
	return ueuo
}

// ClearModuleAlias clears the value of the "ModuleAlias" field.
func (ueuo *UsageEvidenceUpdateOne) ClearModuleAlias() *UsageEvidenceUpdateOne {
	ueuo.mutation.ClearModuleAlias()
	return ueuo
}

// SetIsWildCardUsage sets the "IsWildCardUsage" field.
func (ueuo *UsageEvidenceUpdateOne) SetIsWildCardUsage(b bool) *UsageEvidenceUpdateOne {
	ueuo.mutation.SetIsWildCardUsage(b)
	return ueuo
}

// SetNillableIsWildCardUsage sets the "IsWildCardUsage" field if the given value is not nil.
func (ueuo *UsageEvidenceUpdateOne) SetNillableIsWildCardUsage(b *bool) *UsageEvidenceUpdateOne {
	if b != nil {
		ueuo.SetIsWildCardUsage(*b)
	}
	return ueuo
}

// ClearIsWildCardUsage clears the value of the "IsWildCardUsage" field.
func (ueuo *UsageEvidenceUpdateOne) ClearIsWildCardUsage() *UsageEvidenceUpdateOne {
	ueuo.mutation.ClearIsWildCardUsage()
	return ueuo
}

// SetIdentifier sets the "Identifier" field.
func (ueuo *UsageEvidenceUpdateOne) SetIdentifier(s string) *UsageEvidenceUpdateOne {
	ueuo.mutation.SetIdentifier(s)
	return ueuo
}

// SetNillableIdentifier sets the "Identifier" field if the given value is not nil.
func (ueuo *UsageEvidenceUpdateOne) SetNillableIdentifier(s *string) *UsageEvidenceUpdateOne {
	if s != nil {
		ueuo.SetIdentifier(*s)
	}
	return ueuo
}

// ClearIdentifier clears the value of the "Identifier" field.
func (ueuo *UsageEvidenceUpdateOne) ClearIdentifier() *UsageEvidenceUpdateOne {
	ueuo.mutation.ClearIdentifier()
	return ueuo
}

// SetUsageFilePath sets the "UsageFilePath" field.
func (ueuo *UsageEvidenceUpdateOne) SetUsageFilePath(s string) *UsageEvidenceUpdateOne {
	ueuo.mutation.SetUsageFilePath(s)
	return ueuo
}

// SetNillableUsageFilePath sets the "UsageFilePath" field if the given value is not nil.
func (ueuo *UsageEvidenceUpdateOne) SetNillableUsageFilePath(s *string) *UsageEvidenceUpdateOne {
	if s != nil {
		ueuo.SetUsageFilePath(*s)
	}
	return ueuo
}

// SetLine sets the "Line" field.
func (ueuo *UsageEvidenceUpdateOne) SetLine(u uint) *UsageEvidenceUpdateOne {
	ueuo.mutation.ResetLine()
	ueuo.mutation.SetLine(u)
	return ueuo
}

// SetNillableLine sets the "Line" field if the given value is not nil.
func (ueuo *UsageEvidenceUpdateOne) SetNillableLine(u *uint) *UsageEvidenceUpdateOne {
	if u != nil {
		ueuo.SetLine(*u)
	}
	return ueuo
}

// AddLine adds u to the "Line" field.
func (ueuo *UsageEvidenceUpdateOne) AddLine(u int) *UsageEvidenceUpdateOne {
	ueuo.mutation.AddLine(u)
	return ueuo
}

// SetUsedInID sets the "used_in" edge to the CodeFile entity by ID.
func (ueuo *UsageEvidenceUpdateOne) SetUsedInID(id int) *UsageEvidenceUpdateOne {
	ueuo.mutation.SetUsedInID(id)
	return ueuo
}

// SetUsedIn sets the "used_in" edge to the CodeFile entity.
func (ueuo *UsageEvidenceUpdateOne) SetUsedIn(c *CodeFile) *UsageEvidenceUpdateOne {
	return ueuo.SetUsedInID(c.ID)
}

// Mutation returns the UsageEvidenceMutation object of the builder.
func (ueuo *UsageEvidenceUpdateOne) Mutation() *UsageEvidenceMutation {
	return ueuo.mutation
}

// ClearUsedIn clears the "used_in" edge to the CodeFile entity.
func (ueuo *UsageEvidenceUpdateOne) ClearUsedIn() *UsageEvidenceUpdateOne {
	ueuo.mutation.ClearUsedIn()
	return ueuo
}

// Where appends a list predicates to the UsageEvidenceUpdate builder.
func (ueuo *UsageEvidenceUpdateOne) Where(ps ...predicate.UsageEvidence) *UsageEvidenceUpdateOne {
	ueuo.mutation.Where(ps...)
	return ueuo
}

// Select allows selecting one or more fields (columns) of the returned entity.
// The default is selecting all fields defined in the entity schema.
func (ueuo *UsageEvidenceUpdateOne) Select(field string, fields ...string) *UsageEvidenceUpdateOne {
	ueuo.fields = append([]string{field}, fields...)
	return ueuo
}

// Save executes the query and returns the updated UsageEvidence entity.
func (ueuo *UsageEvidenceUpdateOne) Save(ctx context.Context) (*UsageEvidence, error) {
	return withHooks(ctx, ueuo.sqlSave, ueuo.mutation, ueuo.hooks)
}

// SaveX is like Save, but panics if an error occurs.
func (ueuo *UsageEvidenceUpdateOne) SaveX(ctx context.Context) *UsageEvidence {
	node, err := ueuo.Save(ctx)
	if err != nil {
		panic(err)
	}
	return node
}

// Exec executes the query on the entity.
func (ueuo *UsageEvidenceUpdateOne) Exec(ctx context.Context) error {
	_, err := ueuo.Save(ctx)
	return err
}

// ExecX is like Exec, but panics if an error occurs.
func (ueuo *UsageEvidenceUpdateOne) ExecX(ctx context.Context) {
	if err := ueuo.Exec(ctx); err != nil {
		panic(err)
	}
}

// check runs all checks and user-defined validators on the builder.
func (ueuo *UsageEvidenceUpdateOne) check() error {
	if ueuo.mutation.UsedInCleared() && len(ueuo.mutation.UsedInIDs()) > 0 {
		return errors.New(`ent: clearing a required unique edge "UsageEvidence.used_in"`)
	}
	return nil
}

func (ueuo *UsageEvidenceUpdateOne) sqlSave(ctx context.Context) (_node *UsageEvidence, err error) {
	if err := ueuo.check(); err != nil {
		return _node, err
	}
	_spec := sqlgraph.NewUpdateSpec(usageevidence.Table, usageevidence.Columns, sqlgraph.NewFieldSpec(usageevidence.FieldID, field.TypeInt))
	id, ok := ueuo.mutation.ID()
	if !ok {
		return nil, &ValidationError{Name: "id", err: errors.New(`ent: missing "UsageEvidence.id" for update`)}
	}
	_spec.Node.ID.Value = id
	if fields := ueuo.fields; len(fields) > 0 {
		_spec.Node.Columns = make([]string, 0, len(fields))
		_spec.Node.Columns = append(_spec.Node.Columns, usageevidence.FieldID)
		for _, f := range fields {
			if !usageevidence.ValidColumn(f) {
				return nil, &ValidationError{Name: f, err: fmt.Errorf("ent: invalid field %q for query", f)}
			}
			if f != usageevidence.FieldID {
				_spec.Node.Columns = append(_spec.Node.Columns, f)
			}
		}
	}
	if ps := ueuo.mutation.predicates; len(ps) > 0 {
		_spec.Predicate = func(selector *sql.Selector) {
			for i := range ps {
				ps[i](selector)
			}
		}
	}
	if value, ok := ueuo.mutation.PackageHint(); ok {
		_spec.SetField(usageevidence.FieldPackageHint, field.TypeString, value)
	}
	if ueuo.mutation.PackageHintCleared() {
		_spec.ClearField(usageevidence.FieldPackageHint, field.TypeString)
	}
	if value, ok := ueuo.mutation.ModuleName(); ok {
		_spec.SetField(usageevidence.FieldModuleName, field.TypeString, value)
	}
	if value, ok := ueuo.mutation.ModuleItem(); ok {
		_spec.SetField(usageevidence.FieldModuleItem, field.TypeString, value)
	}
	if ueuo.mutation.ModuleItemCleared() {
		_spec.ClearField(usageevidence.FieldModuleItem, field.TypeString)
	}
	if value, ok := ueuo.mutation.ModuleAlias(); ok {
		_spec.SetField(usageevidence.FieldModuleAlias, field.TypeString, value)
	}
	if ueuo.mutation.ModuleAliasCleared() {
		_spec.ClearField(usageevidence.FieldModuleAlias, field.TypeString)
	}
	if value, ok := ueuo.mutation.IsWildCardUsage(); ok {
		_spec.SetField(usageevidence.FieldIsWildCardUsage, field.TypeBool, value)
	}
	if ueuo.mutation.IsWildCardUsageCleared() {
		_spec.ClearField(usageevidence.FieldIsWildCardUsage, field.TypeBool)
	}
	if value, ok := ueuo.mutation.Identifier(); ok {
		_spec.SetField(usageevidence.FieldIdentifier, field.TypeString, value)
	}
	if ueuo.mutation.IdentifierCleared() {
		_spec.ClearField(usageevidence.FieldIdentifier, field.TypeString)
	}
	if value, ok := ueuo.mutation.UsageFilePath(); ok {
		_spec.SetField(usageevidence.FieldUsageFilePath, field.TypeString, value)
	}
	if value, ok := ueuo.mutation.Line(); ok {
		_spec.SetField(usageevidence.FieldLine, field.TypeUint, value)
	}
	if value, ok := ueuo.mutation.AddedLine(); ok {
		_spec.AddField(usageevidence.FieldLine, field.TypeUint, value)
	}
	if ueuo.mutation.UsedInCleared() {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.M2O,
			Inverse: false,
			Table:   usageevidence.UsedInTable,
			Columns: []string{usageevidence.UsedInColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(codefile.FieldID, field.TypeInt),
			},
		}
		_spec.Edges.Clear = append(_spec.Edges.Clear, edge)
	}
	if nodes := ueuo.mutation.UsedInIDs(); len(nodes) > 0 {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.M2O,
			Inverse: false,
			Table:   usageevidence.UsedInTable,
			Columns: []string{usageevidence.UsedInColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(codefile.FieldID, field.TypeInt),
			},
		}
		for _, k := range nodes {
			edge.Target.Nodes = append(edge.Target.Nodes, k)
		}
		_spec.Edges.Add = append(_spec.Edges.Add, edge)
	}
	_node = &UsageEvidence{config: ueuo.config}
	_spec.Assign = _node.assignValues
	_spec.ScanValues = _node.scanValues
	if err = sqlgraph.UpdateNode(ctx, ueuo.driver, _spec); err != nil {
		if _, ok := err.(*sqlgraph.NotFoundError); ok {
			err = &NotFoundError{usageevidence.Label}
		} else if sqlgraph.IsConstraintError(err) {
			err = &ConstraintError{msg: err.Error(), wrap: err}
		}
		return nil, err
	}
	ueuo.mutation.done = true
	return _node, nil
}
