package modules

import (
	"cmp"
	"encoding/json"
	"errors"
	"io/fs"
	"path"
	"slices"
	"strings"
)

// Composer holds the autoload maps of the composer.json files of a project:
// the PSR-4 prefixes of the namespaces and their directories, and the bin
// scripts. The link step reads the namespaces of the files first, so the
// map names the file of a class that has no index, and tells which
// namespaces are first-party.
type Composer struct {
	prefixes []psr4
	bins     []string
	problems problems
}

type psr4 struct {
	prefix string
	dirs   []string
}

type composerJSON struct {
	Autoload    composerAutoload `json:"autoload"`
	AutoloadDev composerAutoload `json:"autoload-dev"`
	Bin         json.RawMessage  `json:"bin"`
}

type composerAutoload struct {
	PSR4 map[string]json.RawMessage `json:"psr-4"`
}

// NewComposer reads the composer.json of each directory of the file set.
func NewComposer(files *Files, root fs.FS) *Composer {
	c := &Composer{}
	for _, d := range files.Dirs() {
		if root == nil {
			break
		}
		name := path.Join(d, "composer.json")
		data, err := fs.ReadFile(root, fsName(name))
		switch {
		case errors.Is(err, fs.ErrNotExist):
			continue
		case err != nil:
			c.problems.add(name, err)
			continue
		}
		var manifest composerJSON
		if err := json.Unmarshal(data, &manifest); err != nil {
			c.problems.add(name, err)
			continue
		}
		c.add(d, manifest.Autoload.PSR4)
		c.add(d, manifest.AutoloadDev.PSR4)
		for _, bin := range stringOrList(manifest.Bin) {
			if p, ok := Join(d, bin); ok {
				c.bins = append(c.bins, p)
			}
		}
	}
	// The longest prefix holds a name, so it comes first.
	slices.SortStableFunc(c.prefixes, func(a, b psr4) int { return cmp.Compare(len(b.prefix), len(a.prefix)) })
	return c
}

func (c *Composer) add(dir string, prefixes map[string]json.RawMessage) {
	for _, prefix := range sortedKeys(keysOf(prefixes)) {
		var dirs []string
		for _, d := range stringOrList(prefixes[prefix]) {
			if p, ok := Join(dir, d); ok {
				dirs = append(dirs, p)
			}
		}
		c.prefixes = append(c.prefixes, psr4{prefix: strings.TrimPrefix(prefix, `\`), dirs: dirs})
	}
}

// Problems returns the composer.json files that the reader could not read.
func (c *Composer) Problems() []Problem { return c.problems }

// Bins returns the scripts that the bin fields name.
func (c *Composer) Bins() []string { return c.bins }

// Files returns the files that the PSR-4 map gives to a qualified class
// name, as app/Http/Kernel.php for App\Http\Kernel under App\ => app/.
func (c *Composer) Files(class string) []string {
	class = strings.TrimPrefix(class, `\`)
	var out []string
	for _, p := range c.prefixes {
		rest, ok := strings.CutPrefix(class, p.prefix)
		if !ok || rest == "" {
			continue
		}
		for _, d := range p.dirs {
			if file, ok := Join(d, strings.ReplaceAll(rest, `\`, "/")+".php"); ok {
				out = append(out, file)
			}
		}
	}
	return out
}

// FirstParty reports whether a qualified name starts with a prefix of the
// map.
func (c *Composer) FirstParty(name string) bool {
	name = strings.TrimPrefix(name, `\`)
	return slices.ContainsFunc(c.prefixes, func(p psr4) bool {
		return p.prefix != "" && (strings.HasPrefix(name, p.prefix) || name+`\` == p.prefix)
	})
}

// stringOrList reads a JSON string or a list of strings.
func stringOrList(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var one string
	if json.Unmarshal(raw, &one) == nil {
		return []string{one}
	}
	var many []string
	if json.Unmarshal(raw, &many) == nil {
		return many
	}
	return nil
}

func keysOf[V any](m map[string]V) map[string]bool {
	set := make(map[string]bool, len(m))
	for k := range m {
		set[k] = true
	}
	return set
}
