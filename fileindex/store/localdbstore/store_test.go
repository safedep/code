package localdbstore

import (
	"context"
	"strings"
	"testing"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/fileindex/store"
	"github.com/safedep/code/fileindex/store/storetest"
	"github.com/safedep/dry/localdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newManager(t *testing.T, dir string) localdb.Manager {
	t.Helper()
	mgr := localdb.New(localdb.Config{Dir: dir})
	t.Cleanup(func() { assert.NoError(t, mgr.Close()) })
	return mgr
}

func newStore(t *testing.T, mgr localdb.Manager) *Store {
	t.Helper()
	s, err := New(context.Background(), mgr)
	require.NoError(t, err)
	return s
}

func TestStore(t *testing.T) {
	storetest.Run(t, func(t *testing.T) store.Store {
		return newStore(t, newManager(t, t.TempDir()))
	})
}

func TestTwoManagersOnOneFile(t *testing.T) {
	dir := t.TempDir()
	first := newStore(t, newManager(t, dir))
	second := newStore(t, newManager(t, dir))

	a, b := storetest.Index("src/a.py"), storetest.Index("src/b.py")
	require.NoError(t, first.Put(context.Background(), []*fileindex.Index{a}))
	require.NoError(t, second.Put(context.Background(), []*fileindex.Index{b}))

	want := map[fileindex.Key]*fileindex.Index{a.Key(): a, b.Key(): b}
	for _, s := range []*Store{first, second} {
		found, err := s.Get(context.Background(), []fileindex.Key{a.Key(), b.Key()})
		require.NoError(t, err)
		assert.Equal(t, want, found)
	}
}

func TestModuleRegisteredTwiceOnOneManager(t *testing.T) {
	mgr := newManager(t, t.TempDir())
	first := newStore(t, mgr)
	second := newStore(t, mgr)

	a := storetest.Index("src/a.py")
	require.NoError(t, first.Put(context.Background(), []*fileindex.Index{a}))
	found, err := second.Get(context.Background(), []fileindex.Key{a.Key()})
	require.NoError(t, err)
	assert.Equal(t, a, found[a.Key()])

	module, err := mgr.Store(context.Background(), localdb.Descriptor{Name: moduleName, Migrations: migrations})
	require.NoError(t, err)
	assert.Same(t, module.DB(), first.db)

	version, err := module.SchemaVersion(context.Background())
	require.NoError(t, err)
	assert.Equal(t, len(migrations), version)
}

func TestTablesStartWithModuleName(t *testing.T) {
	s := newStore(t, newManager(t, t.TempDir()))

	rows, err := s.db.QueryContext(context.Background(),
		`SELECT name FROM sqlite_master WHERE sql LIKE '%code_graph%' AND type IN ('table', 'index')`)
	require.NoError(t, err)
	defer func() { assert.NoError(t, rows.Close()) }()

	var names []string
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		names = append(names, name)
	}
	require.NoError(t, rows.Err())

	assert.ElementsMatch(t, []string{"code_graph_file_indexes", "code_graph_file_indexes_read"}, names)
	for _, name := range names {
		assert.True(t, strings.HasPrefix(name, moduleName+"_"), name)
	}
}

func TestGetFailsOnACorruptRow(t *testing.T) {
	s := newStore(t, newManager(t, t.TempDir()))
	a := storetest.Index("src/a.py")
	require.NoError(t, s.Put(context.Background(), []*fileindex.Index{a}))

	_, err := s.db.ExecContext(context.Background(), `UPDATE code_graph_file_indexes SET data = 'not json'`)
	require.NoError(t, err)

	_, err = s.Get(context.Background(), []fileindex.Key{a.Key()})
	assert.ErrorContains(t, err, "decode index of src/a.py")
}

func TestPutRejectsANilIndex(t *testing.T) {
	s := newStore(t, newManager(t, t.TempDir()))
	assert.Error(t, s.Put(context.Background(), []*fileindex.Index{nil}))
}
