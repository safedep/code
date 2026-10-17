package fs

import (
	"context"
	"fmt"
	iofs "io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/safedep/code/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalFileSystem(t *testing.T) {
	t.Run("NewLocalFileSystem", func(t *testing.T) {
		t.Run("should return a new LocalFileSystem", func(t *testing.T) {
			config := LocalFileSystemConfig{
				AppDirectories:    []string{"./fixtures/fs/app"},
				ImportDirectories: []string{"./fixtures/fs/import"},
			}

			result, err := NewLocalFileSystem(config)

			assert.NoError(t, err)
			assert.NotNil(t, result)
		})
	})

	t.Run("EnumerateApp", func(t *testing.T) {
		t.Run("should enumerate all files in the app directories", func(t *testing.T) {
			config := LocalFileSystemConfig{
				AppDirectories:    []string{"./fixtures/fs/app"},
				ImportDirectories: []string{"./fixtures/fs/import"},
			}

			fs, _ := NewLocalFileSystem(config)

			var files []string
			err := fs.EnumerateApp(context.Background(), func(f core.File) error {
				files = append(files, f.Name())
				return nil
			})

			assert.NoError(t, err)
			assert.ElementsMatch(t, []string{
				"file-1.txt",
				"file-2.txt",
			}, files)
		})
	})

	t.Run("EnumerateImports", func(t *testing.T) {
		t.Run("should enumerate all files in the import directories", func(t *testing.T) {
			config := LocalFileSystemConfig{
				AppDirectories:    []string{"./fixtures/fs/app"},
				ImportDirectories: []string{"./fixtures/fs/import"},
			}

			fs, _ := NewLocalFileSystem(config)

			var files []string
			err := fs.EnumerateImports(context.Background(), func(f core.File) error {
				files = append(files, f.Name())
				return nil
			})

			assert.NoError(t, err)
			assert.ElementsMatch(t, []string{
				"import-1.txt",
				"import-2.txt",
			}, files)
		})
	})

	t.Run("Enumerate", func(t *testing.T) {
		t.Run("should enumerate all files in the app and import directories", func(t *testing.T) {
			config := LocalFileSystemConfig{
				AppDirectories:    []string{"./fixtures/fs/app"},
				ImportDirectories: []string{"./fixtures/fs/import"},
			}

			fs, _ := NewLocalFileSystem(config)

			var files []string
			err := fs.Enumerate(context.Background(), func(f core.File) error {
				files = append(files, f.Name())
				return nil
			})

			assert.NoError(t, err)
			assert.ElementsMatch(t, []string{
				"file-1.txt",
				"file-2.txt",
				"import-1.txt",
				"import-2.txt",
			}, files)
		})
	})

	t.Run("Enumerate", func(t *testing.T) {
		t.Run("should respect exclude patterns", func(t *testing.T) {
			config := LocalFileSystemConfig{
				AppDirectories:    []string{"./fixtures/fs/app"},
				ImportDirectories: []string{"./fixtures/fs/import"},
				ExcludePatterns:   []*regexp.Regexp{regexp.MustCompile(`.*import-2\.txt$`)},
			}

			fs, _ := NewLocalFileSystem(config)

			var files []string
			err := fs.Enumerate(context.Background(), func(f core.File) error {
				files = append(files, f.Name())
				return nil
			})

			assert.NoError(t, err)
			assert.ElementsMatch(t, []string{
				"file-1.txt",
				"file-2.txt",
				"import-1.txt",
			}, files)
		})
	})

	t.Run("Find", func(t *testing.T) {
		t.Run("should find a file by name", func(t *testing.T) {
			config := LocalFileSystemConfig{
				AppDirectories:    []string{"./fixtures/fs/app"},
				ImportDirectories: []string{"./fixtures/fs/import"},
			}

			fs, _ := NewLocalFileSystem(config)
			file, err := fs.Find(context.Background(), "file-1.txt")

			assert.NoError(t, err)
			assert.Equal(t, "file-1.txt", file.Name())
			assert.Equal(t, filepath.Join("fixtures", "fs", "app", "file-1.txt"), file.Path())
			assert.False(t, file.IsImport())
			assert.True(t, file.IsApp())
		})

		t.Run("should return an error if the file is not found", func(t *testing.T) {
			config := LocalFileSystemConfig{
				AppDirectories:    []string{"./fixtures/fs/app"},
				ImportDirectories: []string{"./fixtures/fs/import"},
			}

			fs, _ := NewLocalFileSystem(config)
			file, err := fs.Find(context.Background(), "file-3.txt")

			assert.Error(t, err)
			assert.Nil(t, file)
		})
	})
}

func TestLocalFileSystemSkipsLinksToNoFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "app.js"), []byte("run()"), 0o600))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "packs"), 0o700))
	links := map[string]string{"linked.js": "app.js", "sw.js": "packs/sw.js", "packs-link": "packs", "loop.js": "loop.js"}
	for name, target := range links {
		if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
			t.Skipf("symbolic links are not available: %v", err)
		}
	}

	fs, err := NewLocalFileSystem(LocalFileSystemConfig{AppDirectories: []string{dir}})
	require.NoError(t, err)

	var files []string
	err = fs.EnumerateApp(context.Background(), func(f core.File) error {
		files = append(files, f.Name())
		return nil
	})

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"app.js", "linked.js"}, files)
}

func TestFileNameAndPath(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "src", "pkg", "app.py")
	top := filepath.Join(dir, "main.py")
	require.NoError(t, os.MkdirAll(filepath.Dir(nested), 0o700))
	require.NoError(t, os.WriteFile(nested, []byte("pass\n"), 0o600))
	require.NoError(t, os.WriteFile(top, []byte("pass\n"), 0o600))

	enumerate := func(root string) map[string]string {
		fs, err := NewLocalFileSystem(LocalFileSystemConfig{AppDirectories: []string{root}})
		require.NoError(t, err)
		paths := map[string]string{}
		require.NoError(t, fs.EnumerateApp(context.Background(), func(f core.File) error {
			paths[f.Name()] = f.Path()
			return nil
		}))
		return paths
	}

	cases := []struct {
		name string
		root string
		want map[string]string
	}{
		{
			name: "a directory root names a nested file and a file at the root with /",
			root: dir,
			want: map[string]string{"src/pkg/app.py": nested, "main.py": top},
		},
		{
			name: "a root that is a file names the file by its base name",
			root: nested,
			want: map[string]string{"app.py": nested},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, enumerate(tc.root))
		})
	}

	t.Run("find names the file relative to the root", func(t *testing.T) {
		fs, err := NewLocalFileSystem(LocalFileSystemConfig{AppDirectories: []string{dir}})
		require.NoError(t, err)
		file, err := fs.Find(context.Background(), filepath.Join("src", "pkg", "app.py"))
		require.NoError(t, err)
		assert.Equal(t, "src/pkg/app.py", file.Name())
		assert.Equal(t, nested, file.Path())
	})

	t.Run("a reader file keeps the name it was given, with /", func(t *testing.T) {
		name := filepath.Join("src", "pkg", "app.py")
		file := NewFileFromReader(strings.NewReader("pass\n"), name, false)
		assert.Equal(t, "src/pkg/app.py", file.Name())
		assert.Equal(t, name, file.Path())
	})
}

// fakeEntry is a directory entry of a walk.
type fakeEntry struct {
	os.DirEntry
	dir bool
}

func (e fakeEntry) IsDir() bool { return e.dir }

func TestSkipUnreadable(t *testing.T) {
	errDenied := os.ErrPermission
	cases := []struct {
		name  string
		path  string
		entry os.DirEntry
		want  error
	}{
		{name: "the root fails the walk", path: "root", entry: fakeEntry{dir: true}, want: errDenied},
		{name: "a root that is not there fails the walk", path: "root", entry: nil, want: errDenied},
		{name: "a directory is skipped", path: "root/locked", entry: fakeEntry{dir: true}, want: filepath.SkipDir},
		{name: "a file is skipped", path: "root/locked.py", entry: fakeEntry{dir: false}, want: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := skipUnreadable("root", tc.path, tc.entry, errDenied)
			if tc.want == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

// lockedWalk walks as filepath.WalkDir does, but it cannot read the
// directory locked. A directory with no permissions does that for a user
// that is not root.
func lockedWalk(locked string) func(string, iofs.WalkDirFunc) error {
	return func(root string, fn iofs.WalkDirFunc) error {
		return filepath.WalkDir(root, func(path string, entry iofs.DirEntry, err error) error {
			if path != locked || err != nil {
				return fn(path, entry, err)
			}
			if err := fn(path, entry, nil); err != nil {
				return err
			}
			return fn(path, entry, fmt.Errorf("open %s: %w", path, iofs.ErrPermission))
		})
	}
}

func TestEnumerateGoesOnPastAnUnreadableDirectory(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a/a.py", "b/b.py", "c.py"} {
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
		require.NoError(t, os.WriteFile(path, []byte("import os\n"), 0o600))
	}

	enumerate := func(locked string) ([]string, error) {
		fsys, err := NewLocalFileSystem(LocalFileSystemConfig{AppDirectories: []string{dir}})
		require.NoError(t, err)
		fsys.(*localFileSystem).walkDir = lockedWalk(locked)

		var names []string
		err = fsys.EnumerateApp(context.Background(), func(f core.File) error {
			names = append(names, filepath.ToSlash(f.Name()))
			return nil
		})
		return names, err
	}

	names, err := enumerate(filepath.Join(dir, "b"))
	require.NoError(t, err)
	assert.Equal(t, []string{"a/a.py", "c.py"}, names)

	_, err = enumerate(dir)
	assert.ErrorIs(t, err, iofs.ErrPermission, "the root of the walk must be readable")
}

func TestImportUnits(t *testing.T) {
	dir := t.TempDir()
	write := func(parts ...string) string {
		p := filepath.Join(append([]string{dir}, parts...)...)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o700))
		require.NoError(t, os.WriteFile(p, []byte("x\n"), 0o600))
		return p
	}
	write("index.js")
	write("node_modules", "lodash", "template.js")
	write("node_modules", "lodash", "node_modules", "inner", "index.js")
	write("node_modules", "lodash", "node_modules", "other", "index.js")
	write("node_modules", "other", "index.js")
	write("site-packages", "requests", "api.py")
	write("site-packages", "six.py")

	lodash := filepath.Join(dir, "node_modules", "lodash")
	inner := filepath.Join(lodash, "node_modules", "inner")
	sitePackages := filepath.Join(dir, "site-packages")
	fs, err := NewLocalFileSystem(LocalFileSystemConfig{
		AppDirectories:    []string{dir},
		ImportDirectories: []string{lodash, inner, sitePackages},
		ImportUnits: map[string]string{
			lodash:                                  "npm:lodash@4.17.21",
			inner:                                   "npm:inner@1.0.0",
			filepath.Join(sitePackages, "requests"): "pypi:requests@2.31.0",
		},
		ExcludePatterns: []*regexp.Regexp{regexp.MustCompile(`[\\/]node_modules[\\/]`), regexp.MustCompile(`[\\/]site-packages[\\/]`)},
	})
	require.NoError(t, err)

	names := func(enumerate func(context.Context, func(core.File) error) error) []string {
		var out []string
		require.NoError(t, enumerate(context.Background(), func(f core.File) error {
			out = append(out, f.Name())
			return nil
		}))
		return out
	}
	assert.ElementsMatch(t, []string{"index.js"}, names(fs.EnumerateApp))
	assert.ElementsMatch(t, []string{
		"npm:lodash@4.17.21//template.js",
		"npm:lodash@4.17.21//node_modules/other/index.js",
		"npm:inner@1.0.0//index.js",
		"pypi:requests@2.31.0//requests/api.py",
		"six.py",
	}, names(fs.EnumerateImports), "a pattern for the walk of a project does not drop a named import directory")

	file, err := fs.Find(context.Background(), "pypi:requests@2.31.0//requests/api.py")
	require.NoError(t, err)
	assert.True(t, file.IsImport())
	assert.Equal(t, filepath.Join(sitePackages, "requests", "api.py"), file.Path())
	_, err = fs.Find(context.Background(), "npm:lodash@1.0.0//template.js")
	assert.Error(t, err, "the unit of a name must match")

	for _, tc := range []struct {
		name  string
		units map[string]string
	}{
		{name: "a unit with no version", units: map[string]string{lodash: "npm:lodash"}},
		{name: "a unit outside the import directories", units: map[string]string{filepath.Join(dir, "node_modules", "other"): "npm:other@1.0.0"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewLocalFileSystem(LocalFileSystemConfig{ImportDirectories: []string{lodash}, ImportUnits: tc.units})
			assert.Error(t, err)
		})
	}
}
