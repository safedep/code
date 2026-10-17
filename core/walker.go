package core

import "context"

type SourceVisitor interface {
	VisitFile(context.Context, File) error
}

// TreeVisitor visits the parse tree of each file of a walk. It owns the tree
// and closes it when it no longer needs the tree.
type TreeVisitor interface {
	VisitTree(context.Context, ParseTree) error
}

// Stage names the step of a walk at which a file failed.
type Stage string

const (
	StageRead  Stage = "read"
	StageParse Stage = "parse"
)

// FileErrorVisitor is an optional interface of a TreeVisitor. A TreeWalker
// calls VisitFileError for a file that it cannot read or parse, and goes on
// with the next file when VisitFileError returns nil. A TreeWalker stops at
// the first such file of a visitor without VisitFileError. The error wraps
// ErrNoLanguage for a file that no language parses.
type FileErrorVisitor interface {
	VisitFileError(ctx context.Context, file File, stage Stage, err error) error
}

type SourceWalker interface {
	Walk(context.Context, ImportAwareFileSystem, SourceVisitor) error
}

type TreeWalker interface {
	Walk(context.Context, ImportAwareFileSystem, TreeVisitor) error
}
