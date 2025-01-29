package core

import "context"

type SourceVisitor interface {
	VisitFile([]Language, File) error
}

type TreeVisitor interface {
	VisitTree([]Language, ParseTree) error
}

type SourceWalker interface {
	Walk(context.Context, ImportAwareFileSystem, SourceVisitor) error
}

type TreeWalker interface {
	Walk(context.Context, ImportAwareFileSystem, TreeVisitor) error
}
