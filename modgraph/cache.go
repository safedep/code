package modgraph

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/fileindex/store"
)

// cachePlugin is the plugin name of the artifacts of Build.
const cachePlugin = "modgraph"

// errCache marks a failure of the store. It ends Build, because it says
// nothing about the module.
var errCache = errors.New("modgraph: cache")

// cacheEntry is the artifact of one package: the imports of its Go files
// that count. Build stores an entry only after it read each file of the
// package with no error, so an entry never stands for a part of a package.
type cacheEntry struct {
	Imports []importRef `json:"imports"`
}

// cacheKey is the key of a package of an immutable source. Its path is the
// unit and the directory, as "go:golang.org/x/net@v0.30.0//html". The go.sum
// hash, when the caller has it, separates two copies of one version with
// other bytes.
func cacheKey(ref pkgRef) store.ArtifactKey {
	m := ref.mod.src.Origin
	if m.Path == "" {
		m = ref.mod.src.Module
	}
	unit := fileindex.PackageUnit{Ecosystem: "go", Name: m.Path, Version: m.Version}.String()
	return store.ArtifactKey{
		Key:     fileindex.Key{Path: fileindex.UnitPath(unit, ref.dir), ContentHash: ref.mod.src.Sum},
		Plugin:  cachePlugin,
		Version: Version,
	}
}

// cacheable reports whether the imports of a package can come from, and go
// to, the cache.
func (b *builder) cacheable(ref pkgRef) bool {
	if b.in.Store == nil || ref.mod.main || !ref.mod.src.Immutable {
		return false
	}
	origin := ref.mod.src.Origin
	if origin.Path == "" {
		origin = ref.mod.src.Module
	}
	return origin.Version != ""
}

// cached returns the imports of a package from the cache.
func (b *builder) cached(ref pkgRef) ([]importRef, bool, error) {
	if !b.cacheable(ref) {
		return nil, false, nil
	}
	k := cacheKey(ref)
	found, err := b.in.Store.GetArtifacts(b.ctx, []store.ArtifactKey{k})
	if err != nil {
		return nil, false, fmt.Errorf("%w: read %s: %w", errCache, k.Path, err)
	}
	data, ok := found[k]
	if !ok {
		return nil, false, nil
	}
	var e cacheEntry
	if err := json.Unmarshal(data, &e); err != nil {
		// A bad entry is a miss. The read that follows writes a good one.
		return nil, false, nil
	}
	b.stats.Packages++
	b.stats.CachedPackages++
	return e.Imports, true, nil
}

// cache stores the imports of a package that Build read whole.
func (b *builder) cache(ref pkgRef, imps []importRef) error {
	if !b.cacheable(ref) {
		return nil
	}
	data, err := json.Marshal(cacheEntry{Imports: imps})
	if err != nil {
		return fmt.Errorf("encode the cache entry of %s: %w", ref.dir, err)
	}
	if err := b.in.Store.PutArtifacts(b.ctx, []store.Artifact{{Key: cacheKey(ref), Data: data}}); err != nil {
		return fmt.Errorf("%w: write %s: %w", errCache, ref.dir, err)
	}
	return nil
}
