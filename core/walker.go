package core

import "context"

type SourceVisitor interface {
	VisitFile(File) error
}

type TreeVisitor interface {
	VisitTree(ParseTree) error
}

type SourceWalker interface {
	Walk(context.Context, ImportAwareFileSystem, SourceVisitor) error
}

type TreeWalker interface {
	Walk(context.Context, ImportAwareFileSystem, TreeVisitor) error
}
