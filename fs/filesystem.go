package fs

import (
	"context"
	"errors"
	"fmt"
	"io"
	iofs "io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fileindex"
	"github.com/safedep/dry/log"
)

type localFile struct {
	path     string
	name     string
	isImport bool
}

type readerFile struct {
	reader   io.Reader
	name     string
	isImport bool
}

var _ core.File = &localFile{}
var _ core.File = &readerFile{}

// newLocalFile makes the file at path, which a walk of root found. A file
// of a unit has the unit first in its name.
func newLocalFile(root, path, unit string, isImport bool) (*localFile, error) {
	relPath, err := filepath.Rel(root, path)
	if err != nil {
		return nil, fmt.Errorf("error getting relative path: %w", err)
	}
	if relPath == "." {
		relPath = filepath.Base(path)
	}

	return &localFile{
		path:     path,
		name:     fileindex.UnitPath(unit, filepath.ToSlash(relPath)),
		isImport: isImport,
	}, nil
}

func (f *localFile) Name() string {
	return f.name
}

func (f *localFile) Path() string {
	return f.path
}

func (f *localFile) Reader() (io.ReadCloser, error) {
	return os.Open(f.path)
}

func (f *localFile) IsApp() bool {
	return !f.isImport
}

func (f *localFile) IsImport() bool {
	return f.isImport
}

// NewFileFromReader creates a new readerFile for provided io.Reader
// with the given name and isImport flag. readerFile is an implementation of core.File
// which is used to represent a file in the Code analysis framework
//
// This is required when you want to analyze a standalone file eg. an artifact file
func NewFileFromReader(reader io.Reader, name string, isImport bool) *readerFile {
	return &readerFile{
		reader:   reader,
		name:     name,
		isImport: isImport,
	}
}

func (f *readerFile) Name() string {
	return filepath.ToSlash(f.name)
}

// Path returns the name that the caller gave. The reader is open already.
func (f *readerFile) Path() string {
	return f.name
}

func (f *readerFile) Reader() (io.ReadCloser, error) {
	return io.NopCloser(f.reader), nil
}

func (f *readerFile) IsApp() bool {
	return !f.isImport
}

func (f *readerFile) IsImport() bool {
	return f.isImport
}

type LocalFileSystemConfig struct {
	// The directories to find 1st party source files
	AppDirectories []string

	// The directories to find 3rd party source files
	// imported by the application. The walk of an app directory skips
	// each import directory below it, and the walk of an import directory
	// skips each other one below it.
	ImportDirectories []string

	// ImportUnits names the package unit of the code under a directory, as
	// npm:lodash@4.17.21 for node_modules/lodash. A key is an import
	// directory, or a directory below one, as site-packages/requests below
	// the import directory site-packages. The deepest key that holds a file
	// gives its unit. The name of the file is the unit, "//" and the path
	// below its import directory, as npm:lodash@4.17.21//template.js or
	// pypi:requests@2.31.0//requests/api.py. The key of an npm or a Go
	// package is the directory of its package.json or its go.mod, because
	// the link step reads the manifest at the root of the unit.
	ImportUnits map[string]string

	// Regular expressions to exclude files or directories
	// from traversal. A pattern that matches an import directory, or a
	// file directly in it, does not apply to the walk of that directory,
	// because the caller names the directory, as node_modules/lodash under
	// a pattern for node_modules.
	ExcludePatterns []*regexp.Regexp
}

type localFileSystem struct {
	config LocalFileSystemConfig

	// imports holds the clean import directories, and units the clean
	// keys of ImportUnits, the deepest first.
	imports []string
	units   []unitDir

	// walkDir walks a directory tree as filepath.WalkDir does. A test sets
	// a walk that cannot read a directory, because root reads every one.
	walkDir func(root string, fn iofs.WalkDirFunc) error
}

type unitDir struct{ dir, unit string }

func NewLocalFileSystem(config LocalFileSystemConfig) (core.ImportAwareFileSystem, error) {
	fs := &localFileSystem{config: config, walkDir: filepath.WalkDir}
	for _, dir := range config.ImportDirectories {
		fs.imports = append(fs.imports, filepath.Clean(dir))
	}
	for dir, unit := range config.ImportUnits {
		if _, err := fileindex.ParseUnit(unit); err != nil {
			return nil, fmt.Errorf("import unit of %s: %w", dir, err)
		}
		dir = filepath.Clean(dir)
		if !slices.ContainsFunc(fs.imports, func(root string) bool { return within(root, dir) }) {
			return nil, fmt.Errorf("import unit of %s: the directory is not an import directory or below one", dir)
		}
		fs.units = append(fs.units, unitDir{dir: dir, unit: unit})
	}
	slices.SortFunc(fs.units, func(a, b unitDir) int { return strings.Compare(b.dir, a.dir) })
	return fs, nil
}

// within reports whether path is dir or a path below it.
func within(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// unitOf returns the unit of the deepest key of ImportUnits that holds the
// path, or "".
func (fs *localFileSystem) unitOf(path string) string {
	for _, u := range fs.units {
		if within(u.dir, path) {
			return u.unit
		}
	}
	return ""
}

func (fs *localFileSystem) Find(ctx context.Context, name string) (core.File, error) {
	for _, dir := range fs.config.AppDirectories {
		if file, err := fs.findFileInDir(ctx, dir, name, false); err == nil {
			return file, nil
		}
	}

	_, inner := fileindex.SplitUnit(name)
	for _, dir := range fs.config.ImportDirectories {
		if file, err := fs.findFileInDir(ctx, dir, inner, true); err == nil && file.Name() == name {
			return file, nil
		}
	}

	return nil, fmt.Errorf("file not found: %s", name)
}

func (fs *localFileSystem) EnumerateApp(ctx context.Context, callback func(core.File) error) error {
	for _, dir := range fs.config.AppDirectories {
		if err := fs.enumerateDir(ctx, dir, false, callback); err != nil {
			return fmt.Errorf("error enumerating app dir: %s: %w", dir, err)
		}
	}

	return nil
}

func (fs *localFileSystem) EnumerateImports(ctx context.Context, callback func(core.File) error) error {
	for _, dir := range fs.config.ImportDirectories {
		if err := fs.enumerateDir(ctx, dir, true, callback); err != nil {
			return fmt.Errorf("error enumerating import dir: %s: %w", dir, err)
		}
	}

	return nil
}

func (fs *localFileSystem) Enumerate(ctx context.Context, callback func(core.File) error) error {
	err := fs.EnumerateApp(ctx, callback)
	if err != nil {
		return err
	}

	return fs.EnumerateImports(ctx, callback)
}

func (fs *localFileSystem) findFileInDir(ctx context.Context, dir, name string, isImport bool) (core.File, error) {
	fullPath := filepath.Join(dir, name)
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("find cancelled by context: %w", ctx.Err())
	default:
	}

	if st, err := os.Stat(fullPath); err == nil && !st.IsDir() {
		return newLocalFile(dir, fullPath, fs.fileUnit(fullPath, isImport), isImport)
	}

	return nil, fmt.Errorf("file not found: %s", name)
}

func (fs *localFileSystem) enumerateDir(ctx context.Context,
	root string, isImport bool, callback func(core.File) error) error {
	patterns := fs.config.ExcludePatterns
	if isImport {
		patterns = slices.DeleteFunc(slices.Clone(patterns), func(p *regexp.Regexp) bool {
			return p.MatchString(root) || p.MatchString(filepath.Join(root, "x"))
		})
	}
	clean := filepath.Clean(root)
	real, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("error walking %s: %w", root, err)
	}
	return fs.walkDir(root, func(path string, info os.DirEntry, err error) error {
		select {
		case <-ctx.Done():
			return fmt.Errorf("enumeration cancelled by context: %w", ctx.Err())
		default:
		}

		if err != nil {
			return skipUnreadable(root, path, info, err)
		}

		if matchAny(patterns, path) {
			return nil
		}

		if info.IsDir() {
			if p := filepath.Clean(path); p != clean && slices.Contains(fs.imports, p) {
				return filepath.SkipDir
			}
			return nil
		}
		if !isRegularFile(real, path, info) {
			return nil
		}

		file, err := newLocalFile(root, path, fs.fileUnit(path, isImport), isImport)
		if err != nil {
			return err
		}
		return callback(file)
	})
}

// fileUnit returns the unit of an import file, or "" for an app file.
func (fs *localFileSystem) fileUnit(path string, isImport bool) string {
	if !isImport {
		return ""
	}
	return fs.unitOf(filepath.Clean(path))
}

// skipUnreadable skips an entry that the walk cannot read, with a warning.
// One unreadable directory must not stop the walk of a whole project, as
// with a bad link in isRegularFile. The root of the walk must be readable.
func skipUnreadable(root, path string, entry os.DirEntry, err error) error {
	if path == root {
		return fmt.Errorf("error walking %s: %w", path, err)
	}

	log.Warnf("skipped %s: %v", path, err)
	if entry != nil && entry.IsDir() {
		return filepath.SkipDir
	}
	return nil
}

// isRegularFile reports a regular file, or a symbolic link to one under
// the root. A dangling link, as to a build output that is not there, is
// not a file. A link that cannot be read for another reason, such as a
// permission or a loop, is not a file either, but it gets a warning. One
// bad link must not stop the walk of a whole project. A link to a file
// outside the root is not a file of the root, so the walk does not read
// what the root does not hold. The root is its real path, as the links
// resolve to real paths.
func isRegularFile(root, path string, entry os.DirEntry) bool {
	if entry.Type()&os.ModeSymlink == 0 {
		return entry.Type().IsRegular()
	}
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		if !errors.Is(err, iofs.ErrNotExist) {
			log.Warnf("skipped the link %s: %v", path, err)
		}
		return false
	}
	if !within(root, target) {
		log.Debugf("skipped the link %s: %s is outside %s", path, target, root)
		return false
	}
	info, err := os.Stat(target)
	if err != nil {
		log.Warnf("skipped the link %s: %v", path, err)
		return false
	}
	return info.Mode().IsRegular()
}

func matchAny(patterns []*regexp.Regexp, path string) bool {
	return slices.ContainsFunc(patterns, func(p *regexp.Regexp) bool { return p.MatchString(path) })
}
