package fs

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
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
				"fixtures/fs/app/file-1.txt",
				"fixtures/fs/app/file-2.txt",
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
				"fixtures/fs/import/import-1.txt",
				"fixtures/fs/import/import-2.txt",
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
				"fixtures/fs/app/file-1.txt",
				"fixtures/fs/app/file-2.txt",
				"fixtures/fs/import/import-1.txt",
				"fixtures/fs/import/import-2.txt",
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
				"fixtures/fs/app/file-1.txt",
				"fixtures/fs/app/file-2.txt",
				"fixtures/fs/import/import-1.txt",
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
			assert.Equal(t, "fixtures/fs/app/file-1.txt", file.Name())
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
		files = append(files, filepath.Base(f.Name()))
		return nil
	})

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"app.js", "linked.js"}, files)
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
