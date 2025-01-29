// Code generated by ent, DO NOT EDIT.

package ent

import (
	"context"
	"database/sql/driver"
	"fmt"
	"math"

	"entgo.io/ent"
	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/sqlgraph"
	"entgo.io/ent/schema/field"
	"github.com/safedep/code/ent/codefile"
	"github.com/safedep/code/ent/predicate"
	"github.com/safedep/code/ent/usageevidence"
)

// CodeFileQuery is the builder for querying CodeFile entities.
type CodeFileQuery struct {
	config
	ctx                *QueryContext
	order              []codefile.OrderOption
	inters             []Interceptor
	predicates         []predicate.CodeFile
	withUsageEvidences *UsageEvidenceQuery
	// intermediate query (i.e. traversal path).
	sql  *sql.Selector
	path func(context.Context) (*sql.Selector, error)
}

// Where adds a new predicate for the CodeFileQuery builder.
func (cfq *CodeFileQuery) Where(ps ...predicate.CodeFile) *CodeFileQuery {
	cfq.predicates = append(cfq.predicates, ps...)
	return cfq
}

// Limit the number of records to be returned by this query.
func (cfq *CodeFileQuery) Limit(limit int) *CodeFileQuery {
	cfq.ctx.Limit = &limit
	return cfq
}

// Offset to start from.
func (cfq *CodeFileQuery) Offset(offset int) *CodeFileQuery {
	cfq.ctx.Offset = &offset
	return cfq
}

// Unique configures the query builder to filter duplicate records on query.
// By default, unique is set to true, and can be disabled using this method.
func (cfq *CodeFileQuery) Unique(unique bool) *CodeFileQuery {
	cfq.ctx.Unique = &unique
	return cfq
}

// Order specifies how the records should be ordered.
func (cfq *CodeFileQuery) Order(o ...codefile.OrderOption) *CodeFileQuery {
	cfq.order = append(cfq.order, o...)
	return cfq
}

// QueryUsageEvidences chains the current query on the "usage_evidences" edge.
func (cfq *CodeFileQuery) QueryUsageEvidences() *UsageEvidenceQuery {
	query := (&UsageEvidenceClient{config: cfq.config}).Query()
	query.path = func(ctx context.Context) (fromU *sql.Selector, err error) {
		if err := cfq.prepareQuery(ctx); err != nil {
			return nil, err
		}
		selector := cfq.sqlQuery(ctx)
		if err := selector.Err(); err != nil {
			return nil, err
		}
		step := sqlgraph.NewStep(
			sqlgraph.From(codefile.Table, codefile.FieldID, selector),
			sqlgraph.To(usageevidence.Table, usageevidence.FieldID),
			sqlgraph.Edge(sqlgraph.O2M, true, codefile.UsageEvidencesTable, codefile.UsageEvidencesColumn),
		)
		fromU = sqlgraph.SetNeighbors(cfq.driver.Dialect(), step)
		return fromU, nil
	}
	return query
}

// First returns the first CodeFile entity from the query.
// Returns a *NotFoundError when no CodeFile was found.
func (cfq *CodeFileQuery) First(ctx context.Context) (*CodeFile, error) {
	nodes, err := cfq.Limit(1).All(setContextOp(ctx, cfq.ctx, ent.OpQueryFirst))
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, &NotFoundError{codefile.Label}
	}
	return nodes[0], nil
}

// FirstX is like First, but panics if an error occurs.
func (cfq *CodeFileQuery) FirstX(ctx context.Context) *CodeFile {
	node, err := cfq.First(ctx)
	if err != nil && !IsNotFound(err) {
		panic(err)
	}
	return node
}

// FirstID returns the first CodeFile ID from the query.
// Returns a *NotFoundError when no CodeFile ID was found.
func (cfq *CodeFileQuery) FirstID(ctx context.Context) (id int, err error) {
	var ids []int
	if ids, err = cfq.Limit(1).IDs(setContextOp(ctx, cfq.ctx, ent.OpQueryFirstID)); err != nil {
		return
	}
	if len(ids) == 0 {
		err = &NotFoundError{codefile.Label}
		return
	}
	return ids[0], nil
}

// FirstIDX is like FirstID, but panics if an error occurs.
func (cfq *CodeFileQuery) FirstIDX(ctx context.Context) int {
	id, err := cfq.FirstID(ctx)
	if err != nil && !IsNotFound(err) {
		panic(err)
	}
	return id
}

// Only returns a single CodeFile entity found by the query, ensuring it only returns one.
// Returns a *NotSingularError when more than one CodeFile entity is found.
// Returns a *NotFoundError when no CodeFile entities are found.
func (cfq *CodeFileQuery) Only(ctx context.Context) (*CodeFile, error) {
	nodes, err := cfq.Limit(2).All(setContextOp(ctx, cfq.ctx, ent.OpQueryOnly))
	if err != nil {
		return nil, err
	}
	switch len(nodes) {
	case 1:
		return nodes[0], nil
	case 0:
		return nil, &NotFoundError{codefile.Label}
	default:
		return nil, &NotSingularError{codefile.Label}
	}
}

// OnlyX is like Only, but panics if an error occurs.
func (cfq *CodeFileQuery) OnlyX(ctx context.Context) *CodeFile {
	node, err := cfq.Only(ctx)
	if err != nil {
		panic(err)
	}
	return node
}

// OnlyID is like Only, but returns the only CodeFile ID in the query.
// Returns a *NotSingularError when more than one CodeFile ID is found.
// Returns a *NotFoundError when no entities are found.
func (cfq *CodeFileQuery) OnlyID(ctx context.Context) (id int, err error) {
	var ids []int
	if ids, err = cfq.Limit(2).IDs(setContextOp(ctx, cfq.ctx, ent.OpQueryOnlyID)); err != nil {
		return
	}
	switch len(ids) {
	case 1:
		id = ids[0]
	case 0:
		err = &NotFoundError{codefile.Label}
	default:
		err = &NotSingularError{codefile.Label}
	}
	return
}

// OnlyIDX is like OnlyID, but panics if an error occurs.
func (cfq *CodeFileQuery) OnlyIDX(ctx context.Context) int {
	id, err := cfq.OnlyID(ctx)
	if err != nil {
		panic(err)
	}
	return id
}

// All executes the query and returns a list of CodeFiles.
func (cfq *CodeFileQuery) All(ctx context.Context) ([]*CodeFile, error) {
	ctx = setContextOp(ctx, cfq.ctx, ent.OpQueryAll)
	if err := cfq.prepareQuery(ctx); err != nil {
		return nil, err
	}
	qr := querierAll[[]*CodeFile, *CodeFileQuery]()
	return withInterceptors[[]*CodeFile](ctx, cfq, qr, cfq.inters)
}

// AllX is like All, but panics if an error occurs.
func (cfq *CodeFileQuery) AllX(ctx context.Context) []*CodeFile {
	nodes, err := cfq.All(ctx)
	if err != nil {
		panic(err)
	}
	return nodes
}

// IDs executes the query and returns a list of CodeFile IDs.
func (cfq *CodeFileQuery) IDs(ctx context.Context) (ids []int, err error) {
	if cfq.ctx.Unique == nil && cfq.path != nil {
		cfq.Unique(true)
	}
	ctx = setContextOp(ctx, cfq.ctx, ent.OpQueryIDs)
	if err = cfq.Select(codefile.FieldID).Scan(ctx, &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

// IDsX is like IDs, but panics if an error occurs.
func (cfq *CodeFileQuery) IDsX(ctx context.Context) []int {
	ids, err := cfq.IDs(ctx)
	if err != nil {
		panic(err)
	}
	return ids
}

// Count returns the count of the given query.
func (cfq *CodeFileQuery) Count(ctx context.Context) (int, error) {
	ctx = setContextOp(ctx, cfq.ctx, ent.OpQueryCount)
	if err := cfq.prepareQuery(ctx); err != nil {
		return 0, err
	}
	return withInterceptors[int](ctx, cfq, querierCount[*CodeFileQuery](), cfq.inters)
}

// CountX is like Count, but panics if an error occurs.
func (cfq *CodeFileQuery) CountX(ctx context.Context) int {
	count, err := cfq.Count(ctx)
	if err != nil {
		panic(err)
	}
	return count
}

// Exist returns true if the query has elements in the graph.
func (cfq *CodeFileQuery) Exist(ctx context.Context) (bool, error) {
	ctx = setContextOp(ctx, cfq.ctx, ent.OpQueryExist)
	switch _, err := cfq.FirstID(ctx); {
	case IsNotFound(err):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("ent: check existence: %w", err)
	default:
		return true, nil
	}
}

// ExistX is like Exist, but panics if an error occurs.
func (cfq *CodeFileQuery) ExistX(ctx context.Context) bool {
	exist, err := cfq.Exist(ctx)
	if err != nil {
		panic(err)
	}
	return exist
}

// Clone returns a duplicate of the CodeFileQuery builder, including all associated steps. It can be
// used to prepare common query builders and use them differently after the clone is made.
func (cfq *CodeFileQuery) Clone() *CodeFileQuery {
	if cfq == nil {
		return nil
	}
	return &CodeFileQuery{
		config:             cfq.config,
		ctx:                cfq.ctx.Clone(),
		order:              append([]codefile.OrderOption{}, cfq.order...),
		inters:             append([]Interceptor{}, cfq.inters...),
		predicates:         append([]predicate.CodeFile{}, cfq.predicates...),
		withUsageEvidences: cfq.withUsageEvidences.Clone(),
		// clone intermediate query.
		sql:  cfq.sql.Clone(),
		path: cfq.path,
	}
}

// WithUsageEvidences tells the query-builder to eager-load the nodes that are connected to
// the "usage_evidences" edge. The optional arguments are used to configure the query builder of the edge.
func (cfq *CodeFileQuery) WithUsageEvidences(opts ...func(*UsageEvidenceQuery)) *CodeFileQuery {
	query := (&UsageEvidenceClient{config: cfq.config}).Query()
	for _, opt := range opts {
		opt(query)
	}
	cfq.withUsageEvidences = query
	return cfq
}

// GroupBy is used to group vertices by one or more fields/columns.
// It is often used with aggregate functions, like: count, max, mean, min, sum.
//
// Example:
//
//	var v []struct {
//		FilePath string `json:"FilePath,omitempty"`
//		Count int `json:"count,omitempty"`
//	}
//
//	client.CodeFile.Query().
//		GroupBy(codefile.FieldFilePath).
//		Aggregate(ent.Count()).
//		Scan(ctx, &v)
func (cfq *CodeFileQuery) GroupBy(field string, fields ...string) *CodeFileGroupBy {
	cfq.ctx.Fields = append([]string{field}, fields...)
	grbuild := &CodeFileGroupBy{build: cfq}
	grbuild.flds = &cfq.ctx.Fields
	grbuild.label = codefile.Label
	grbuild.scan = grbuild.Scan
	return grbuild
}

// Select allows the selection one or more fields/columns for the given query,
// instead of selecting all fields in the entity.
//
// Example:
//
//	var v []struct {
//		FilePath string `json:"FilePath,omitempty"`
//	}
//
//	client.CodeFile.Query().
//		Select(codefile.FieldFilePath).
//		Scan(ctx, &v)
func (cfq *CodeFileQuery) Select(fields ...string) *CodeFileSelect {
	cfq.ctx.Fields = append(cfq.ctx.Fields, fields...)
	sbuild := &CodeFileSelect{CodeFileQuery: cfq}
	sbuild.label = codefile.Label
	sbuild.flds, sbuild.scan = &cfq.ctx.Fields, sbuild.Scan
	return sbuild
}

// Aggregate returns a CodeFileSelect configured with the given aggregations.
func (cfq *CodeFileQuery) Aggregate(fns ...AggregateFunc) *CodeFileSelect {
	return cfq.Select().Aggregate(fns...)
}

func (cfq *CodeFileQuery) prepareQuery(ctx context.Context) error {
	for _, inter := range cfq.inters {
		if inter == nil {
			return fmt.Errorf("ent: uninitialized interceptor (forgotten import ent/runtime?)")
		}
		if trv, ok := inter.(Traverser); ok {
			if err := trv.Traverse(ctx, cfq); err != nil {
				return err
			}
		}
	}
	for _, f := range cfq.ctx.Fields {
		if !codefile.ValidColumn(f) {
			return &ValidationError{Name: f, err: fmt.Errorf("ent: invalid field %q for query", f)}
		}
	}
	if cfq.path != nil {
		prev, err := cfq.path(ctx)
		if err != nil {
			return err
		}
		cfq.sql = prev
	}
	return nil
}

func (cfq *CodeFileQuery) sqlAll(ctx context.Context, hooks ...queryHook) ([]*CodeFile, error) {
	var (
		nodes       = []*CodeFile{}
		_spec       = cfq.querySpec()
		loadedTypes = [1]bool{
			cfq.withUsageEvidences != nil,
		}
	)
	_spec.ScanValues = func(columns []string) ([]any, error) {
		return (*CodeFile).scanValues(nil, columns)
	}
	_spec.Assign = func(columns []string, values []any) error {
		node := &CodeFile{config: cfq.config}
		nodes = append(nodes, node)
		node.Edges.loadedTypes = loadedTypes
		return node.assignValues(columns, values)
	}
	for i := range hooks {
		hooks[i](ctx, _spec)
	}
	if err := sqlgraph.QueryNodes(ctx, cfq.driver, _spec); err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nodes, nil
	}
	if query := cfq.withUsageEvidences; query != nil {
		if err := cfq.loadUsageEvidences(ctx, query, nodes,
			func(n *CodeFile) { n.Edges.UsageEvidences = []*UsageEvidence{} },
			func(n *CodeFile, e *UsageEvidence) { n.Edges.UsageEvidences = append(n.Edges.UsageEvidences, e) }); err != nil {
			return nil, err
		}
	}
	return nodes, nil
}

func (cfq *CodeFileQuery) loadUsageEvidences(ctx context.Context, query *UsageEvidenceQuery, nodes []*CodeFile, init func(*CodeFile), assign func(*CodeFile, *UsageEvidence)) error {
	fks := make([]driver.Value, 0, len(nodes))
	nodeids := make(map[int]*CodeFile)
	for i := range nodes {
		fks = append(fks, nodes[i].ID)
		nodeids[nodes[i].ID] = nodes[i]
		if init != nil {
			init(nodes[i])
		}
	}
	query.withFKs = true
	query.Where(predicate.UsageEvidence(func(s *sql.Selector) {
		s.Where(sql.InValues(s.C(codefile.UsageEvidencesColumn), fks...))
	}))
	neighbors, err := query.All(ctx)
	if err != nil {
		return err
	}
	for _, n := range neighbors {
		fk := n.usage_evidence_code_file
		if fk == nil {
			return fmt.Errorf(`foreign-key "usage_evidence_code_file" is nil for node %v`, n.ID)
		}
		node, ok := nodeids[*fk]
		if !ok {
			return fmt.Errorf(`unexpected referenced foreign-key "usage_evidence_code_file" returned %v for node %v`, *fk, n.ID)
		}
		assign(node, n)
	}
	return nil
}

func (cfq *CodeFileQuery) sqlCount(ctx context.Context) (int, error) {
	_spec := cfq.querySpec()
	_spec.Node.Columns = cfq.ctx.Fields
	if len(cfq.ctx.Fields) > 0 {
		_spec.Unique = cfq.ctx.Unique != nil && *cfq.ctx.Unique
	}
	return sqlgraph.CountNodes(ctx, cfq.driver, _spec)
}

func (cfq *CodeFileQuery) querySpec() *sqlgraph.QuerySpec {
	_spec := sqlgraph.NewQuerySpec(codefile.Table, codefile.Columns, sqlgraph.NewFieldSpec(codefile.FieldID, field.TypeInt))
	_spec.From = cfq.sql
	if unique := cfq.ctx.Unique; unique != nil {
		_spec.Unique = *unique
	} else if cfq.path != nil {
		_spec.Unique = true
	}
	if fields := cfq.ctx.Fields; len(fields) > 0 {
		_spec.Node.Columns = make([]string, 0, len(fields))
		_spec.Node.Columns = append(_spec.Node.Columns, codefile.FieldID)
		for i := range fields {
			if fields[i] != codefile.FieldID {
				_spec.Node.Columns = append(_spec.Node.Columns, fields[i])
			}
		}
	}
	if ps := cfq.predicates; len(ps) > 0 {
		_spec.Predicate = func(selector *sql.Selector) {
			for i := range ps {
				ps[i](selector)
			}
		}
	}
	if limit := cfq.ctx.Limit; limit != nil {
		_spec.Limit = *limit
	}
	if offset := cfq.ctx.Offset; offset != nil {
		_spec.Offset = *offset
	}
	if ps := cfq.order; len(ps) > 0 {
		_spec.Order = func(selector *sql.Selector) {
			for i := range ps {
				ps[i](selector)
			}
		}
	}
	return _spec
}

func (cfq *CodeFileQuery) sqlQuery(ctx context.Context) *sql.Selector {
	builder := sql.Dialect(cfq.driver.Dialect())
	t1 := builder.Table(codefile.Table)
	columns := cfq.ctx.Fields
	if len(columns) == 0 {
		columns = codefile.Columns
	}
	selector := builder.Select(t1.Columns(columns...)...).From(t1)
	if cfq.sql != nil {
		selector = cfq.sql
		selector.Select(selector.Columns(columns...)...)
	}
	if cfq.ctx.Unique != nil && *cfq.ctx.Unique {
		selector.Distinct()
	}
	for _, p := range cfq.predicates {
		p(selector)
	}
	for _, p := range cfq.order {
		p(selector)
	}
	if offset := cfq.ctx.Offset; offset != nil {
		// limit is mandatory for offset clause. We start
		// with default value, and override it below if needed.
		selector.Offset(*offset).Limit(math.MaxInt32)
	}
	if limit := cfq.ctx.Limit; limit != nil {
		selector.Limit(*limit)
	}
	return selector
}

// CodeFileGroupBy is the group-by builder for CodeFile entities.
type CodeFileGroupBy struct {
	selector
	build *CodeFileQuery
}

// Aggregate adds the given aggregation functions to the group-by query.
func (cfgb *CodeFileGroupBy) Aggregate(fns ...AggregateFunc) *CodeFileGroupBy {
	cfgb.fns = append(cfgb.fns, fns...)
	return cfgb
}

// Scan applies the selector query and scans the result into the given value.
func (cfgb *CodeFileGroupBy) Scan(ctx context.Context, v any) error {
	ctx = setContextOp(ctx, cfgb.build.ctx, ent.OpQueryGroupBy)
	if err := cfgb.build.prepareQuery(ctx); err != nil {
		return err
	}
	return scanWithInterceptors[*CodeFileQuery, *CodeFileGroupBy](ctx, cfgb.build, cfgb, cfgb.build.inters, v)
}

func (cfgb *CodeFileGroupBy) sqlScan(ctx context.Context, root *CodeFileQuery, v any) error {
	selector := root.sqlQuery(ctx).Select()
	aggregation := make([]string, 0, len(cfgb.fns))
	for _, fn := range cfgb.fns {
		aggregation = append(aggregation, fn(selector))
	}
	if len(selector.SelectedColumns()) == 0 {
		columns := make([]string, 0, len(*cfgb.flds)+len(cfgb.fns))
		for _, f := range *cfgb.flds {
			columns = append(columns, selector.C(f))
		}
		columns = append(columns, aggregation...)
		selector.Select(columns...)
	}
	selector.GroupBy(selector.Columns(*cfgb.flds...)...)
	if err := selector.Err(); err != nil {
		return err
	}
	rows := &sql.Rows{}
	query, args := selector.Query()
	if err := cfgb.build.driver.Query(ctx, query, args, rows); err != nil {
		return err
	}
	defer rows.Close()
	return sql.ScanSlice(rows, v)
}

// CodeFileSelect is the builder for selecting fields of CodeFile entities.
type CodeFileSelect struct {
	*CodeFileQuery
	selector
}

// Aggregate adds the given aggregation functions to the selector query.
func (cfs *CodeFileSelect) Aggregate(fns ...AggregateFunc) *CodeFileSelect {
	cfs.fns = append(cfs.fns, fns...)
	return cfs
}

// Scan applies the selector query and scans the result into the given value.
func (cfs *CodeFileSelect) Scan(ctx context.Context, v any) error {
	ctx = setContextOp(ctx, cfs.ctx, ent.OpQuerySelect)
	if err := cfs.prepareQuery(ctx); err != nil {
		return err
	}
	return scanWithInterceptors[*CodeFileQuery, *CodeFileSelect](ctx, cfs.CodeFileQuery, cfs, cfs.inters, v)
}

func (cfs *CodeFileSelect) sqlScan(ctx context.Context, root *CodeFileQuery, v any) error {
	selector := root.sqlQuery(ctx)
	aggregation := make([]string, 0, len(cfs.fns))
	for _, fn := range cfs.fns {
		aggregation = append(aggregation, fn(selector))
	}
	switch n := len(*cfs.selector.flds); {
	case n == 0 && len(aggregation) > 0:
		selector.Select(aggregation...)
	case n != 0 && len(aggregation) > 0:
		selector.AppendSelect(aggregation...)
	}
	rows := &sql.Rows{}
	query, args := selector.Query()
	if err := cfs.driver.Query(ctx, query, args, rows); err != nil {
		return err
	}
	defer rows.Close()
	return sql.ScanSlice(rows, v)
}
