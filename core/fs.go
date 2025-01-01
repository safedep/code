package core

import (
	"context"
	"io"
)

type File interface {
	// The name of the file in the file system.
	// This is the path relative to the root of the file system
	// source defined by the file system.
	Name() string

	// Open the file for reading.
	Reader() (io.ReadCloser, error)

	// Flag to identify if the file is an application source file
	IsApp() bool

	// Flag to identify if the file is an import source file
	IsImport() bool
}

type FileSystem interface {
	// Enumerate the contents of the file system
	Enumerate(context.Context, func(File) error) error

	// Find a file by name in the file system
	// The name is a relative path to the root of the file system
	Find(context.Context, string) (File, error)
}

// ImportAwareFileSystem is a contract for implementing file systems
// that maintain a distinction between application source files and
// imported source files by the application. This is a first class concept
// in our system because we need the ability to distinguish between the two.
type ImportAwareFileSystem interface {
	// Base file system
	FileSystem

	// Enumerate application source files in the file system
	EnumerateApp(context.Context, func(File) error) error

	// Enumerate import source files in the file system
	EnumerateImports(context.Context, func(File) error) error
}
