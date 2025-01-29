// Code generated by ent, DO NOT EDIT.

package ent

import (
	"context"
	"errors"
	"fmt"
	"log"
	"reflect"

	"github.com/safedep/code/ent/migrate"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/sqlgraph"
	"github.com/safedep/code/ent/callgraphnode"
	"github.com/safedep/code/ent/codefile"
	"github.com/safedep/code/ent/usageevidence"
)

// Client is the client that holds all ent builders.
type Client struct {
	config
	// Schema is the client for creating, migrating and dropping schema.
	Schema *migrate.Schema
	// CallgraphNode is the client for interacting with the CallgraphNode builders.
	CallgraphNode *CallgraphNodeClient
	// CodeFile is the client for interacting with the CodeFile builders.
	CodeFile *CodeFileClient
	// UsageEvidence is the client for interacting with the UsageEvidence builders.
	UsageEvidence *UsageEvidenceClient
}

// NewClient creates a new client configured with the given options.
func NewClient(opts ...Option) *Client {
	client := &Client{config: newConfig(opts...)}
	client.init()
	return client
}

func (c *Client) init() {
	c.Schema = migrate.NewSchema(c.driver)
	c.CallgraphNode = NewCallgraphNodeClient(c.config)
	c.CodeFile = NewCodeFileClient(c.config)
	c.UsageEvidence = NewUsageEvidenceClient(c.config)
}

type (
	// config is the configuration for the client and its builder.
	config struct {
		// driver used for executing database requests.
		driver dialect.Driver
		// debug enable a debug logging.
		debug bool
		// log used for logging on debug mode.
		log func(...any)
		// hooks to execute on mutations.
		hooks *hooks
		// interceptors to execute on queries.
		inters *inters
	}
	// Option function to configure the client.
	Option func(*config)
)

// newConfig creates a new config for the client.
func newConfig(opts ...Option) config {
	cfg := config{log: log.Println, hooks: &hooks{}, inters: &inters{}}
	cfg.options(opts...)
	return cfg
}

// options applies the options on the config object.
func (c *config) options(opts ...Option) {
	for _, opt := range opts {
		opt(c)
	}
	if c.debug {
		c.driver = dialect.Debug(c.driver, c.log)
	}
}

// Debug enables debug logging on the ent.Driver.
func Debug() Option {
	return func(c *config) {
		c.debug = true
	}
}

// Log sets the logging function for debug mode.
func Log(fn func(...any)) Option {
	return func(c *config) {
		c.log = fn
	}
}

// Driver configures the client driver.
func Driver(driver dialect.Driver) Option {
	return func(c *config) {
		c.driver = driver
	}
}

// Open opens a database/sql.DB specified by the driver name and
// the data source name, and returns a new client attached to it.
// Optional parameters can be added for configuring the client.
func Open(driverName, dataSourceName string, options ...Option) (*Client, error) {
	switch driverName {
	case dialect.MySQL, dialect.Postgres, dialect.SQLite:
		drv, err := sql.Open(driverName, dataSourceName)
		if err != nil {
			return nil, err
		}
		return NewClient(append(options, Driver(drv))...), nil
	default:
		return nil, fmt.Errorf("unsupported driver: %q", driverName)
	}
}

// ErrTxStarted is returned when trying to start a new transaction from a transactional client.
var ErrTxStarted = errors.New("ent: cannot start a transaction within a transaction")

// Tx returns a new transactional client. The provided context
// is used until the transaction is committed or rolled back.
func (c *Client) Tx(ctx context.Context) (*Tx, error) {
	if _, ok := c.driver.(*txDriver); ok {
		return nil, ErrTxStarted
	}
	tx, err := newTx(ctx, c.driver)
	if err != nil {
		return nil, fmt.Errorf("ent: starting a transaction: %w", err)
	}
	cfg := c.config
	cfg.driver = tx
	return &Tx{
		ctx:           ctx,
		config:        cfg,
		CallgraphNode: NewCallgraphNodeClient(cfg),
		CodeFile:      NewCodeFileClient(cfg),
		UsageEvidence: NewUsageEvidenceClient(cfg),
	}, nil
}

// BeginTx returns a transactional client with specified options.
func (c *Client) BeginTx(ctx context.Context, opts *sql.TxOptions) (*Tx, error) {
	if _, ok := c.driver.(*txDriver); ok {
		return nil, errors.New("ent: cannot start a transaction within a transaction")
	}
	tx, err := c.driver.(interface {
		BeginTx(context.Context, *sql.TxOptions) (dialect.Tx, error)
	}).BeginTx(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("ent: starting a transaction: %w", err)
	}
	cfg := c.config
	cfg.driver = &txDriver{tx: tx, drv: c.driver}
	return &Tx{
		ctx:           ctx,
		config:        cfg,
		CallgraphNode: NewCallgraphNodeClient(cfg),
		CodeFile:      NewCodeFileClient(cfg),
		UsageEvidence: NewUsageEvidenceClient(cfg),
	}, nil
}

// Debug returns a new debug-client. It's used to get verbose logging on specific operations.
//
//	client.Debug().
//		CallgraphNode.
//		Query().
//		Count(ctx)
func (c *Client) Debug() *Client {
	if c.debug {
		return c
	}
	cfg := c.config
	cfg.driver = dialect.Debug(c.driver, c.log)
	client := &Client{config: cfg}
	client.init()
	return client
}

// Close closes the database connection and prevents new queries from starting.
func (c *Client) Close() error {
	return c.driver.Close()
}

// Use adds the mutation hooks to all the entity clients.
// In order to add hooks to a specific client, call: `client.Node.Use(...)`.
func (c *Client) Use(hooks ...Hook) {
	c.CallgraphNode.Use(hooks...)
	c.CodeFile.Use(hooks...)
	c.UsageEvidence.Use(hooks...)
}

// Intercept adds the query interceptors to all the entity clients.
// In order to add interceptors to a specific client, call: `client.Node.Intercept(...)`.
func (c *Client) Intercept(interceptors ...Interceptor) {
	c.CallgraphNode.Intercept(interceptors...)
	c.CodeFile.Intercept(interceptors...)
	c.UsageEvidence.Intercept(interceptors...)
}

// Mutate implements the ent.Mutator interface.
func (c *Client) Mutate(ctx context.Context, m Mutation) (Value, error) {
	switch m := m.(type) {
	case *CallgraphNodeMutation:
		return c.CallgraphNode.mutate(ctx, m)
	case *CodeFileMutation:
		return c.CodeFile.mutate(ctx, m)
	case *UsageEvidenceMutation:
		return c.UsageEvidence.mutate(ctx, m)
	default:
		return nil, fmt.Errorf("ent: unknown mutation type %T", m)
	}
}

// CallgraphNodeClient is a client for the CallgraphNode schema.
type CallgraphNodeClient struct {
	config
}

// NewCallgraphNodeClient returns a client for the CallgraphNode from the given config.
func NewCallgraphNodeClient(c config) *CallgraphNodeClient {
	return &CallgraphNodeClient{config: c}
}

// Use adds a list of mutation hooks to the hooks stack.
// A call to `Use(f, g, h)` equals to `callgraphnode.Hooks(f(g(h())))`.
func (c *CallgraphNodeClient) Use(hooks ...Hook) {
	c.hooks.CallgraphNode = append(c.hooks.CallgraphNode, hooks...)
}

// Intercept adds a list of query interceptors to the interceptors stack.
// A call to `Intercept(f, g, h)` equals to `callgraphnode.Intercept(f(g(h())))`.
func (c *CallgraphNodeClient) Intercept(interceptors ...Interceptor) {
	c.inters.CallgraphNode = append(c.inters.CallgraphNode, interceptors...)
}

// Create returns a builder for creating a CallgraphNode entity.
func (c *CallgraphNodeClient) Create() *CallgraphNodeCreate {
	mutation := newCallgraphNodeMutation(c.config, OpCreate)
	return &CallgraphNodeCreate{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// CreateBulk returns a builder for creating a bulk of CallgraphNode entities.
func (c *CallgraphNodeClient) CreateBulk(builders ...*CallgraphNodeCreate) *CallgraphNodeCreateBulk {
	return &CallgraphNodeCreateBulk{config: c.config, builders: builders}
}

// MapCreateBulk creates a bulk creation builder from the given slice. For each item in the slice, the function creates
// a builder and applies setFunc on it.
func (c *CallgraphNodeClient) MapCreateBulk(slice any, setFunc func(*CallgraphNodeCreate, int)) *CallgraphNodeCreateBulk {
	rv := reflect.ValueOf(slice)
	if rv.Kind() != reflect.Slice {
		return &CallgraphNodeCreateBulk{err: fmt.Errorf("calling to CallgraphNodeClient.MapCreateBulk with wrong type %T, need slice", slice)}
	}
	builders := make([]*CallgraphNodeCreate, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		builders[i] = c.Create()
		setFunc(builders[i], i)
	}
	return &CallgraphNodeCreateBulk{config: c.config, builders: builders}
}

// Update returns an update builder for CallgraphNode.
func (c *CallgraphNodeClient) Update() *CallgraphNodeUpdate {
	mutation := newCallgraphNodeMutation(c.config, OpUpdate)
	return &CallgraphNodeUpdate{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// UpdateOne returns an update builder for the given entity.
func (c *CallgraphNodeClient) UpdateOne(cn *CallgraphNode) *CallgraphNodeUpdateOne {
	mutation := newCallgraphNodeMutation(c.config, OpUpdateOne, withCallgraphNode(cn))
	return &CallgraphNodeUpdateOne{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// UpdateOneID returns an update builder for the given id.
func (c *CallgraphNodeClient) UpdateOneID(id int) *CallgraphNodeUpdateOne {
	mutation := newCallgraphNodeMutation(c.config, OpUpdateOne, withCallgraphNodeID(id))
	return &CallgraphNodeUpdateOne{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// Delete returns a delete builder for CallgraphNode.
func (c *CallgraphNodeClient) Delete() *CallgraphNodeDelete {
	mutation := newCallgraphNodeMutation(c.config, OpDelete)
	return &CallgraphNodeDelete{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// DeleteOne returns a builder for deleting the given entity.
func (c *CallgraphNodeClient) DeleteOne(cn *CallgraphNode) *CallgraphNodeDeleteOne {
	return c.DeleteOneID(cn.ID)
}

// DeleteOneID returns a builder for deleting the given entity by its id.
func (c *CallgraphNodeClient) DeleteOneID(id int) *CallgraphNodeDeleteOne {
	builder := c.Delete().Where(callgraphnode.ID(id))
	builder.mutation.id = &id
	builder.mutation.op = OpDeleteOne
	return &CallgraphNodeDeleteOne{builder}
}

// Query returns a query builder for CallgraphNode.
func (c *CallgraphNodeClient) Query() *CallgraphNodeQuery {
	return &CallgraphNodeQuery{
		config: c.config,
		ctx:    &QueryContext{Type: TypeCallgraphNode},
		inters: c.Interceptors(),
	}
}

// Get returns a CallgraphNode entity by its id.
func (c *CallgraphNodeClient) Get(ctx context.Context, id int) (*CallgraphNode, error) {
	return c.Query().Where(callgraphnode.ID(id)).Only(ctx)
}

// GetX is like Get, but panics if an error occurs.
func (c *CallgraphNodeClient) GetX(ctx context.Context, id int) *CallgraphNode {
	obj, err := c.Get(ctx, id)
	if err != nil {
		panic(err)
	}
	return obj
}

// QueryCalledBy queries the called_by edge of a CallgraphNode.
func (c *CallgraphNodeClient) QueryCalledBy(cn *CallgraphNode) *CallgraphNodeQuery {
	query := (&CallgraphNodeClient{config: c.config}).Query()
	query.path = func(context.Context) (fromV *sql.Selector, _ error) {
		id := cn.ID
		step := sqlgraph.NewStep(
			sqlgraph.From(callgraphnode.Table, callgraphnode.FieldID, id),
			sqlgraph.To(callgraphnode.Table, callgraphnode.FieldID),
			sqlgraph.Edge(sqlgraph.M2M, true, callgraphnode.CalledByTable, callgraphnode.CalledByPrimaryKey...),
		)
		fromV = sqlgraph.Neighbors(cn.driver.Dialect(), step)
		return fromV, nil
	}
	return query
}

// QueryCallsTo queries the calls_to edge of a CallgraphNode.
func (c *CallgraphNodeClient) QueryCallsTo(cn *CallgraphNode) *CallgraphNodeQuery {
	query := (&CallgraphNodeClient{config: c.config}).Query()
	query.path = func(context.Context) (fromV *sql.Selector, _ error) {
		id := cn.ID
		step := sqlgraph.NewStep(
			sqlgraph.From(callgraphnode.Table, callgraphnode.FieldID, id),
			sqlgraph.To(callgraphnode.Table, callgraphnode.FieldID),
			sqlgraph.Edge(sqlgraph.M2M, false, callgraphnode.CallsToTable, callgraphnode.CallsToPrimaryKey...),
		)
		fromV = sqlgraph.Neighbors(cn.driver.Dialect(), step)
		return fromV, nil
	}
	return query
}

// Hooks returns the client hooks.
func (c *CallgraphNodeClient) Hooks() []Hook {
	return c.hooks.CallgraphNode
}

// Interceptors returns the client interceptors.
func (c *CallgraphNodeClient) Interceptors() []Interceptor {
	return c.inters.CallgraphNode
}

func (c *CallgraphNodeClient) mutate(ctx context.Context, m *CallgraphNodeMutation) (Value, error) {
	switch m.Op() {
	case OpCreate:
		return (&CallgraphNodeCreate{config: c.config, hooks: c.Hooks(), mutation: m}).Save(ctx)
	case OpUpdate:
		return (&CallgraphNodeUpdate{config: c.config, hooks: c.Hooks(), mutation: m}).Save(ctx)
	case OpUpdateOne:
		return (&CallgraphNodeUpdateOne{config: c.config, hooks: c.Hooks(), mutation: m}).Save(ctx)
	case OpDelete, OpDeleteOne:
		return (&CallgraphNodeDelete{config: c.config, hooks: c.Hooks(), mutation: m}).Exec(ctx)
	default:
		return nil, fmt.Errorf("ent: unknown CallgraphNode mutation op: %q", m.Op())
	}
}

// CodeFileClient is a client for the CodeFile schema.
type CodeFileClient struct {
	config
}

// NewCodeFileClient returns a client for the CodeFile from the given config.
func NewCodeFileClient(c config) *CodeFileClient {
	return &CodeFileClient{config: c}
}

// Use adds a list of mutation hooks to the hooks stack.
// A call to `Use(f, g, h)` equals to `codefile.Hooks(f(g(h())))`.
func (c *CodeFileClient) Use(hooks ...Hook) {
	c.hooks.CodeFile = append(c.hooks.CodeFile, hooks...)
}

// Intercept adds a list of query interceptors to the interceptors stack.
// A call to `Intercept(f, g, h)` equals to `codefile.Intercept(f(g(h())))`.
func (c *CodeFileClient) Intercept(interceptors ...Interceptor) {
	c.inters.CodeFile = append(c.inters.CodeFile, interceptors...)
}

// Create returns a builder for creating a CodeFile entity.
func (c *CodeFileClient) Create() *CodeFileCreate {
	mutation := newCodeFileMutation(c.config, OpCreate)
	return &CodeFileCreate{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// CreateBulk returns a builder for creating a bulk of CodeFile entities.
func (c *CodeFileClient) CreateBulk(builders ...*CodeFileCreate) *CodeFileCreateBulk {
	return &CodeFileCreateBulk{config: c.config, builders: builders}
}

// MapCreateBulk creates a bulk creation builder from the given slice. For each item in the slice, the function creates
// a builder and applies setFunc on it.
func (c *CodeFileClient) MapCreateBulk(slice any, setFunc func(*CodeFileCreate, int)) *CodeFileCreateBulk {
	rv := reflect.ValueOf(slice)
	if rv.Kind() != reflect.Slice {
		return &CodeFileCreateBulk{err: fmt.Errorf("calling to CodeFileClient.MapCreateBulk with wrong type %T, need slice", slice)}
	}
	builders := make([]*CodeFileCreate, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		builders[i] = c.Create()
		setFunc(builders[i], i)
	}
	return &CodeFileCreateBulk{config: c.config, builders: builders}
}

// Update returns an update builder for CodeFile.
func (c *CodeFileClient) Update() *CodeFileUpdate {
	mutation := newCodeFileMutation(c.config, OpUpdate)
	return &CodeFileUpdate{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// UpdateOne returns an update builder for the given entity.
func (c *CodeFileClient) UpdateOne(cf *CodeFile) *CodeFileUpdateOne {
	mutation := newCodeFileMutation(c.config, OpUpdateOne, withCodeFile(cf))
	return &CodeFileUpdateOne{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// UpdateOneID returns an update builder for the given id.
func (c *CodeFileClient) UpdateOneID(id int) *CodeFileUpdateOne {
	mutation := newCodeFileMutation(c.config, OpUpdateOne, withCodeFileID(id))
	return &CodeFileUpdateOne{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// Delete returns a delete builder for CodeFile.
func (c *CodeFileClient) Delete() *CodeFileDelete {
	mutation := newCodeFileMutation(c.config, OpDelete)
	return &CodeFileDelete{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// DeleteOne returns a builder for deleting the given entity.
func (c *CodeFileClient) DeleteOne(cf *CodeFile) *CodeFileDeleteOne {
	return c.DeleteOneID(cf.ID)
}

// DeleteOneID returns a builder for deleting the given entity by its id.
func (c *CodeFileClient) DeleteOneID(id int) *CodeFileDeleteOne {
	builder := c.Delete().Where(codefile.ID(id))
	builder.mutation.id = &id
	builder.mutation.op = OpDeleteOne
	return &CodeFileDeleteOne{builder}
}

// Query returns a query builder for CodeFile.
func (c *CodeFileClient) Query() *CodeFileQuery {
	return &CodeFileQuery{
		config: c.config,
		ctx:    &QueryContext{Type: TypeCodeFile},
		inters: c.Interceptors(),
	}
}

// Get returns a CodeFile entity by its id.
func (c *CodeFileClient) Get(ctx context.Context, id int) (*CodeFile, error) {
	return c.Query().Where(codefile.ID(id)).Only(ctx)
}

// GetX is like Get, but panics if an error occurs.
func (c *CodeFileClient) GetX(ctx context.Context, id int) *CodeFile {
	obj, err := c.Get(ctx, id)
	if err != nil {
		panic(err)
	}
	return obj
}

// QueryUsageEvidences queries the usage_evidences edge of a CodeFile.
func (c *CodeFileClient) QueryUsageEvidences(cf *CodeFile) *UsageEvidenceQuery {
	query := (&UsageEvidenceClient{config: c.config}).Query()
	query.path = func(context.Context) (fromV *sql.Selector, _ error) {
		id := cf.ID
		step := sqlgraph.NewStep(
			sqlgraph.From(codefile.Table, codefile.FieldID, id),
			sqlgraph.To(usageevidence.Table, usageevidence.FieldID),
			sqlgraph.Edge(sqlgraph.O2M, true, codefile.UsageEvidencesTable, codefile.UsageEvidencesColumn),
		)
		fromV = sqlgraph.Neighbors(cf.driver.Dialect(), step)
		return fromV, nil
	}
	return query
}

// Hooks returns the client hooks.
func (c *CodeFileClient) Hooks() []Hook {
	return c.hooks.CodeFile
}

// Interceptors returns the client interceptors.
func (c *CodeFileClient) Interceptors() []Interceptor {
	return c.inters.CodeFile
}

func (c *CodeFileClient) mutate(ctx context.Context, m *CodeFileMutation) (Value, error) {
	switch m.Op() {
	case OpCreate:
		return (&CodeFileCreate{config: c.config, hooks: c.Hooks(), mutation: m}).Save(ctx)
	case OpUpdate:
		return (&CodeFileUpdate{config: c.config, hooks: c.Hooks(), mutation: m}).Save(ctx)
	case OpUpdateOne:
		return (&CodeFileUpdateOne{config: c.config, hooks: c.Hooks(), mutation: m}).Save(ctx)
	case OpDelete, OpDeleteOne:
		return (&CodeFileDelete{config: c.config, hooks: c.Hooks(), mutation: m}).Exec(ctx)
	default:
		return nil, fmt.Errorf("ent: unknown CodeFile mutation op: %q", m.Op())
	}
}

// UsageEvidenceClient is a client for the UsageEvidence schema.
type UsageEvidenceClient struct {
	config
}

// NewUsageEvidenceClient returns a client for the UsageEvidence from the given config.
func NewUsageEvidenceClient(c config) *UsageEvidenceClient {
	return &UsageEvidenceClient{config: c}
}

// Use adds a list of mutation hooks to the hooks stack.
// A call to `Use(f, g, h)` equals to `usageevidence.Hooks(f(g(h())))`.
func (c *UsageEvidenceClient) Use(hooks ...Hook) {
	c.hooks.UsageEvidence = append(c.hooks.UsageEvidence, hooks...)
}

// Intercept adds a list of query interceptors to the interceptors stack.
// A call to `Intercept(f, g, h)` equals to `usageevidence.Intercept(f(g(h())))`.
func (c *UsageEvidenceClient) Intercept(interceptors ...Interceptor) {
	c.inters.UsageEvidence = append(c.inters.UsageEvidence, interceptors...)
}

// Create returns a builder for creating a UsageEvidence entity.
func (c *UsageEvidenceClient) Create() *UsageEvidenceCreate {
	mutation := newUsageEvidenceMutation(c.config, OpCreate)
	return &UsageEvidenceCreate{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// CreateBulk returns a builder for creating a bulk of UsageEvidence entities.
func (c *UsageEvidenceClient) CreateBulk(builders ...*UsageEvidenceCreate) *UsageEvidenceCreateBulk {
	return &UsageEvidenceCreateBulk{config: c.config, builders: builders}
}

// MapCreateBulk creates a bulk creation builder from the given slice. For each item in the slice, the function creates
// a builder and applies setFunc on it.
func (c *UsageEvidenceClient) MapCreateBulk(slice any, setFunc func(*UsageEvidenceCreate, int)) *UsageEvidenceCreateBulk {
	rv := reflect.ValueOf(slice)
	if rv.Kind() != reflect.Slice {
		return &UsageEvidenceCreateBulk{err: fmt.Errorf("calling to UsageEvidenceClient.MapCreateBulk with wrong type %T, need slice", slice)}
	}
	builders := make([]*UsageEvidenceCreate, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		builders[i] = c.Create()
		setFunc(builders[i], i)
	}
	return &UsageEvidenceCreateBulk{config: c.config, builders: builders}
}

// Update returns an update builder for UsageEvidence.
func (c *UsageEvidenceClient) Update() *UsageEvidenceUpdate {
	mutation := newUsageEvidenceMutation(c.config, OpUpdate)
	return &UsageEvidenceUpdate{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// UpdateOne returns an update builder for the given entity.
func (c *UsageEvidenceClient) UpdateOne(ue *UsageEvidence) *UsageEvidenceUpdateOne {
	mutation := newUsageEvidenceMutation(c.config, OpUpdateOne, withUsageEvidence(ue))
	return &UsageEvidenceUpdateOne{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// UpdateOneID returns an update builder for the given id.
func (c *UsageEvidenceClient) UpdateOneID(id int) *UsageEvidenceUpdateOne {
	mutation := newUsageEvidenceMutation(c.config, OpUpdateOne, withUsageEvidenceID(id))
	return &UsageEvidenceUpdateOne{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// Delete returns a delete builder for UsageEvidence.
func (c *UsageEvidenceClient) Delete() *UsageEvidenceDelete {
	mutation := newUsageEvidenceMutation(c.config, OpDelete)
	return &UsageEvidenceDelete{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// DeleteOne returns a builder for deleting the given entity.
func (c *UsageEvidenceClient) DeleteOne(ue *UsageEvidence) *UsageEvidenceDeleteOne {
	return c.DeleteOneID(ue.ID)
}

// DeleteOneID returns a builder for deleting the given entity by its id.
func (c *UsageEvidenceClient) DeleteOneID(id int) *UsageEvidenceDeleteOne {
	builder := c.Delete().Where(usageevidence.ID(id))
	builder.mutation.id = &id
	builder.mutation.op = OpDeleteOne
	return &UsageEvidenceDeleteOne{builder}
}

// Query returns a query builder for UsageEvidence.
func (c *UsageEvidenceClient) Query() *UsageEvidenceQuery {
	return &UsageEvidenceQuery{
		config: c.config,
		ctx:    &QueryContext{Type: TypeUsageEvidence},
		inters: c.Interceptors(),
	}
}

// Get returns a UsageEvidence entity by its id.
func (c *UsageEvidenceClient) Get(ctx context.Context, id int) (*UsageEvidence, error) {
	return c.Query().Where(usageevidence.ID(id)).Only(ctx)
}

// GetX is like Get, but panics if an error occurs.
func (c *UsageEvidenceClient) GetX(ctx context.Context, id int) *UsageEvidence {
	obj, err := c.Get(ctx, id)
	if err != nil {
		panic(err)
	}
	return obj
}

// QueryCodeFile queries the code_file edge of a UsageEvidence.
func (c *UsageEvidenceClient) QueryCodeFile(ue *UsageEvidence) *CodeFileQuery {
	query := (&CodeFileClient{config: c.config}).Query()
	query.path = func(context.Context) (fromV *sql.Selector, _ error) {
		id := ue.ID
		step := sqlgraph.NewStep(
			sqlgraph.From(usageevidence.Table, usageevidence.FieldID, id),
			sqlgraph.To(codefile.Table, codefile.FieldID),
			sqlgraph.Edge(sqlgraph.M2O, false, usageevidence.CodeFileTable, usageevidence.CodeFileColumn),
		)
		fromV = sqlgraph.Neighbors(ue.driver.Dialect(), step)
		return fromV, nil
	}
	return query
}

// Hooks returns the client hooks.
func (c *UsageEvidenceClient) Hooks() []Hook {
	return c.hooks.UsageEvidence
}

// Interceptors returns the client interceptors.
func (c *UsageEvidenceClient) Interceptors() []Interceptor {
	return c.inters.UsageEvidence
}

func (c *UsageEvidenceClient) mutate(ctx context.Context, m *UsageEvidenceMutation) (Value, error) {
	switch m.Op() {
	case OpCreate:
		return (&UsageEvidenceCreate{config: c.config, hooks: c.Hooks(), mutation: m}).Save(ctx)
	case OpUpdate:
		return (&UsageEvidenceUpdate{config: c.config, hooks: c.Hooks(), mutation: m}).Save(ctx)
	case OpUpdateOne:
		return (&UsageEvidenceUpdateOne{config: c.config, hooks: c.Hooks(), mutation: m}).Save(ctx)
	case OpDelete, OpDeleteOne:
		return (&UsageEvidenceDelete{config: c.config, hooks: c.Hooks(), mutation: m}).Exec(ctx)
	default:
		return nil, fmt.Errorf("ent: unknown UsageEvidence mutation op: %q", m.Op())
	}
}

// hooks and interceptors per client, for fast access.
type (
	hooks struct {
		CallgraphNode, CodeFile, UsageEvidence []ent.Hook
	}
	inters struct {
		CallgraphNode, CodeFile, UsageEvidence []ent.Interceptor
	}
)
