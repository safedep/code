package indexer

import (
	"context"
	"slices"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fileindex/store"
)

// CachedPlugin is an extra plugin whose output per file the store keeps
// next to the index. A warm run with only cached plugins parses no file:
// the run gives each plugin its stored artifact in place of a tree. The
// run keys an artifact by the fileindex.Key of the file and the Name and
// the Version of the plugin, so a new Version makes every older artifact
// miss.
type CachedPlugin interface {
	core.Plugin

	// Version names the rules of the plugin. Bump it when the artifact of
	// a file changes.
	Version() string

	// Analyze runs the plugin on the tree of a file and returns the
	// artifact to store.
	Analyze(ctx context.Context, file core.File, tree core.ParseTree) ([]byte, error)

	// Restore runs the plugin on the stored artifact of a file, with the
	// effect that Analyze has.
	Restore(ctx context.Context, file core.File, data []byte) error
}

// artifactKey returns the key of the artifact of a cached plugin for a
// file.
func artifactKey(p pending, pl CachedPlugin) store.ArtifactKey {
	return store.ArtifactKey{Key: p.key, Plugin: pl.Name(), Version: pl.Version()}
}

// supports reports whether a plugin runs on the language of a file.
func supports(pl core.Plugin, p pending) bool {
	return slices.Contains(pl.SupportedLanguages(), p.language.Meta().Code)
}

// needsParse reports whether a file with a stored index still needs its
// tree: a plugin of its language is not cached, or an artifact of a cached
// plugin is not stored.
func (r *run) needsParse(p pending, artifacts map[store.ArtifactKey][]byte) bool {
	for _, pl := range r.opts.Plugins {
		if !supports(pl, p) {
			continue
		}
		cached, ok := pl.(CachedPlugin)
		if !ok {
			return true
		}
		if _, ok := artifacts[artifactKey(p, cached)]; !ok {
			return true
		}
	}
	return false
}

// restore runs each cached plugin of the file on its stored artifact. A
// plugin that fails goes to the report, as it does after a parse.
func (r *run) restore(ctx context.Context, p pending, artifacts map[store.ArtifactKey][]byte) {
	for _, pl := range r.opts.Plugins {
		cached, ok := pl.(CachedPlugin)
		if !ok || !supports(pl, p) {
			continue
		}
		if err := cached.Restore(ctx, p.file, artifacts[artifactKey(p, cached)]); err != nil {
			r.result.Report.Failed = append(r.result.Report.Failed, failure(p.file, pl, err))
		}
	}
}
