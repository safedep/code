package reach

import (
	"encoding/json"
	"testing"

	"github.com/safedep/code/langcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTargetByKey(t *testing.T) {
	for _, tt := range []struct {
		key  string
		want string
	}{
		{key: "yaml//load"},
		{key: "yaml"},
		{key: "yaml//*"},
		{key: "npm:lodash@4.17.21//template.js//template"},
		{key: "", want: "the key is empty"},
		{key: "//load", want: "an empty segment"},
		{key: "yaml//", want: "an empty segment"},
		{key: "yaml////load", want: "an empty segment"},
		{key: "*", want: "not the last segment"},
		{key: "yaml//*//load", want: "not the last segment"},
	} {
		t.Run(tt.key, func(t *testing.T) {
			target, err := TargetByKey(tt.key)
			if tt.want != "" {
				assert.ErrorContains(t, err, tt.want)
				assert.True(t, target.IsZero())
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.key, target.Key())
			assert.Equal(t, tt.key, target.String())
			assert.False(t, target.IsZero())
		})
	}
}

func TestTargetByImport(t *testing.T) {
	for _, tt := range []struct {
		lang   langcode.Code
		module string
		names  []string
		want   string
	}{
		{lang: langcode.Go, module: "os/exec", names: []string{"Command"}, want: "os//exec//Command"},
		{lang: langcode.Python, module: "yaml", names: []string{"load"}, want: "yaml//load"},
		{lang: langcode.Python, module: "yaml", want: "yaml"},
		{lang: langcode.Python, module: "yaml", names: []string{"*"}, want: "yaml//*"},
		{lang: langcode.Java, module: "java.lang.Thread", names: []string{"sleep"}, want: "java//lang//Thread//sleep"},
		{lang: langcode.Rust, module: "std::process", names: []string{"Command", "new"}, want: "std//process//Command//new"},
		{lang: langcode.PHP, module: `\App\Kernel`, names: []string{"run"}, want: "App//Kernel//run"},
	} {
		t.Run(tt.want, func(t *testing.T) {
			target, err := TargetByImport(tt.lang, tt.module, tt.names...)
			require.NoError(t, err)
			assert.Equal(t, tt.want, target.Key())
		})
	}

	_, err := TargetByImport(langcode.Python, "")
	assert.ErrorContains(t, err, "no module")
	_, err = TargetByImport(langcode.Python, "yaml", "")
	assert.ErrorContains(t, err, "an empty segment")
}

func TestTargetsByKey(t *testing.T) {
	targets, err := TargetsByKey("yaml//load", "os//*")
	require.NoError(t, err)
	assert.Equal(t, []Target{{key: "yaml//load"}, {key: "os//*"}}, targets)

	_, err = TargetsByKey("yaml//load", "")
	assert.ErrorContains(t, err, `target ""`)
}

func TestTargetJSONIsTheKey(t *testing.T) {
	data, err := json.Marshal(Result{Target: targetOf(t, "yaml//load"), Status: Found})
	require.NoError(t, err)
	assert.Contains(t, string(data), `"target":"yaml//load"`)

	var r Result
	require.NoError(t, json.Unmarshal(data, &r))
	assert.Equal(t, "yaml//load", r.Target.Key())
	assert.ErrorContains(t, json.Unmarshal([]byte(`{"target":"//x"}`), &r), "an empty segment")
}

func TestTargetMatches(t *testing.T) {
	for _, tt := range []struct {
		target string
		key    string
		want   bool
	}{
		{target: "yaml//load", key: "yaml//load", want: true},
		{target: "yaml//load", key: "yaml//safe_load"},
		{target: "yaml//*", key: "yaml//load", want: true},
		{target: "yaml//*", key: "yaml", want: false},
		{target: "yaml//*", key: "yamlx//load"},
		{target: "zlib//*", key: "yaml//load"},
	} {
		t.Run(tt.target+" "+tt.key, func(t *testing.T) {
			assert.Equal(t, tt.want, targetOf(t, tt.target).Matches(tt.key))
		})
	}
}

func TestTargetMatchesSorted(t *testing.T) {
	keys := []string{"os//exec", "yaml", "yaml//load", "yaml//safe_load", "yamlx//load"}
	for _, tt := range []struct {
		target string
		keys   []string
		want   bool
	}{
		{target: "yaml//load", keys: keys, want: true},
		{target: "yaml//dump", keys: keys},
		{target: "yaml//*", keys: keys, want: true},
		{target: "yaml//*", keys: []string{"yaml", "yamlx//load"}, want: false},
		{target: "os//*", keys: keys, want: true},
		{target: "zlib//*", keys: keys},
		{target: "yaml//*", keys: nil},
	} {
		t.Run(tt.target, func(t *testing.T) {
			assert.Equal(t, tt.want, targetOf(t, tt.target).MatchesSorted(tt.keys))
		})
	}
}

func TestScopedConcerns(t *testing.T) {
	for _, tt := range []struct {
		name   string
		scoped Scoped
		target string
		want   bool
	}{
		{name: "no scope", scoped: Scoped{}, target: "yaml//load", want: true},
		{name: "unit with the target", scoped: Scoped{Uncertainty: Uncertainty{Unit: "npm:a@1"}, Targets: []string{"fs//rm"}}, target: "fs//rm", want: true},
		{name: "unit without the target", scoped: Scoped{Uncertainty: Uncertainty{Unit: "npm:a@1"}, Targets: []string{"fs//rm"}}, target: "os//exit"},
		{name: "unit and a wildcard", scoped: Scoped{Uncertainty: Uncertainty{Unit: "npm:a@1"}, Targets: []string{"fs//rm"}}, target: "fs//*", want: true},
		{name: "prefix under", scoped: Scoped{Uncertainty: Uncertainty{ImportPrefix: "yaml//"}}, target: "yaml//load", want: true},
		{name: "prefix elsewhere", scoped: Scoped{Uncertainty: Uncertainty{ImportPrefix: "yaml//"}}, target: "os//exit"},
		{name: "prefix inside a wildcard", scoped: Scoped{Uncertainty: Uncertainty{ImportPrefix: "yaml//lo"}}, target: "yaml//*", want: true},
		{name: "prefix of another module", scoped: Scoped{Uncertainty: Uncertainty{ImportPrefix: "yamlx//"}}, target: "yaml//*"},
		{name: "prefix of another module with no separator", scoped: Scoped{Uncertainty: Uncertainty{ImportPrefix: "yamlx"}}, target: "yaml//*"},
		{name: "prefix shorter than the module", scoped: Scoped{Uncertainty: Uncertainty{ImportPrefix: "ya"}}, target: "yaml//*", want: true},
		{name: "method by name", scoped: Scoped{Uncertainty: Uncertainty{Method: "load"}}, target: "yaml//load", want: true},
		{name: "method by reach", scoped: Scoped{Uncertainty: Uncertainty{Method: "run"}, Targets: []string{"os//exit"}}, target: "os//exit", want: true},
		{name: "method elsewhere", scoped: Scoped{Uncertainty: Uncertainty{Method: "run"}, Targets: []string{"os//exit"}}, target: "yaml//load"},
		{name: "method and a wildcard", scoped: Scoped{Uncertainty: Uncertainty{Method: "run"}}, target: "yaml//*", want: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.scoped.Concerns(targetOf(t, tt.target)))
		})
	}
}
