package fs

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/safedep/code/core"
)

type localFile struct {
	path     string
	name     string
	isImport bool
}

func (f *localFile) Name() string {
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

type LocalFileSystemConfig struct {
	// The directories to find 1st party source files
	AppDirectories []string

	// The directories to find 3rd party source files
	// imported by the application
	ImportDirectories []string

	// Regular expressions to exclude files or directories
	// from traversal
	ExcludePatterns []*regexp.Regexp
}

type localFileSystem struct {
	config LocalFileSystemConfig
}

func NewLocalFileSystem(config LocalFileSystemConfig) (core.ImportAwareFileSystem, error) {
	return &localFileSystem{config: config}, nil
}

func (fs *localFileSystem) Find(ctx context.Context, name string) (core.File, error) {
	for _, dir := range fs.config.AppDirectories {
		if file, err := fs.findFileInDir(ctx, dir, name, false); err == nil {
			return file, nil
		}
	}

	for _, dir := range fs.config.ImportDirectories {
		if file, err := fs.findFileInDir(ctx, dir, name, true); err == nil {
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
	if st, err := os.Stat(fullPath); err == nil && !st.IsDir() {
		relPath, err := filepath.Rel(dir, fullPath)
		if err != nil {
			return nil, fmt.Errorf("error getting relative path: %w", err)
		}

		return &localFile{
			path:     fullPath,
			name:     relPath,
			isImport: isImport,
		}, nil
	}

	return nil, fmt.Errorf("file not found: %s", name)
}

func (fs *localFileSystem) enumerateDir(ctx context.Context,
	root string, isImport bool, callback func(core.File) error) error {
	return filepath.WalkDir(root, func(path string, info os.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("error walking %s: %w", path, err)
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("enumeration cancelled by context: %w", ctx.Err())
		default:
		}

		if fs.skipPattern(path) {
			return nil
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("error getting relative path: %w", err)
		}

		file := &localFile{
			path:     path,
			isImport: isImport,
			name:     relPath,
		}
		return callback(file)
	})
}

func (fs *localFileSystem) skipPattern(dir string) bool {
	for _, pattern := range fs.config.ExcludePatterns {
		if pattern.MatchString(dir) {
			return true
		}
	}

	return false
}
