package modules

import (
	"encoding/json"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mapFS(files map[string]string) fstest.MapFS {
	fsys := fstest.MapFS{}
	for name, data := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(data)}
	}
	return fsys
}

func TestJavaScriptResolve(t *testing.T) {
	files := NewFiles([]string{
		"src/main.js",
		"src/util.js",
		"src/lib/index.js",
		"src/types.d.ts",
		"src/app.ts",
		"src/view.tsx",
		"src/pkgdir/start.js",
		"packages/a/index.js",
		"packages/a/sub.js",
		"packages/skip/index.js",
	})
	root := mapFS(map[string]string{
		"package.json":               `{"workspaces": ["packages/*", "!packages/skip"]}`,
		"src/pkgdir/package.json":    `{"main": "start"}`,
		"packages/a/package.json":    `{"name": "a"}`,
		"packages/skip/package.json": `{"name": "skip"}`,
	})
	j := NewJavaScript(files, root)
	require.Empty(t, j.Problems())

	tests := []struct {
		name     string
		importer string
		spec     string
		ts       bool
		want     string
		via      string
	}{
		{name: "a relative path with no extension", importer: "src/main.js", spec: "./util", want: "src/util.js", via: "relative"},
		{name: "a directory index", importer: "src/main.js", spec: "./lib", want: "src/lib/index.js", via: "relative"},
		{name: "a directory with a package.json main", importer: "src/main.js", spec: "./pkgdir", want: "src/pkgdir/start.js", via: "relative"},
		{name: "a parent path", importer: "src/lib/index.js", spec: "../util.js", want: "src/util.js", via: "relative"},
		{name: "a .js specifier to a .ts file", importer: "src/main.ts", spec: "./app.js", ts: true, want: "src/app.ts", via: "relative"},
		{name: "a .js specifier to a .tsx file", importer: "src/main.ts", spec: "./view.js", ts: true, want: "src/view.tsx", via: "relative"},
		{name: "a declaration file", importer: "src/main.ts", spec: "./types", ts: true, want: "src/types.d.ts", via: "relative"},
		{name: "a JavaScript importer reads no .ts file", importer: "src/main.js", spec: "./app"},
		{name: "a workspace package by its index", importer: "src/main.js", spec: "a", want: "packages/a/index.js", via: "index"},
		{name: "a subpath of a workspace package", importer: "src/main.js", spec: "a/sub", want: "packages/a/sub.js", via: "subpath"},
		{name: "a negated workspace is external", importer: "src/main.js", spec: "skip"},
		{name: "an npm package is external", importer: "src/main.js", spec: "lodash"},
		{name: "a path out of the root", importer: "src/main.js", spec: "../../x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, via, ok := j.Resolve(tt.importer, tt.spec, tt.ts, FormImport)
			assert.Equal(t, tt.want != "", ok)
			if tt.want == "" {
				assert.Empty(t, got)
			} else {
				assert.Equal(t, []string{tt.want}, got)
			}
			assert.Equal(t, tt.via, via)
		})
	}
}

func TestJavaScriptResolveByForm(t *testing.T) {
	files := NewFiles([]string{"src/main.js", "packages/kit/cjs.js", "packages/kit/esm.js", "packages/kit/web.js", "packages/kit/deno.js", "packages/only/web.js"})
	root := mapFS(map[string]string{
		"package.json":               `{"workspaces": ["packages/*"]}`,
		"packages/kit/package.json":  `{"name": "kit", "exports": {".": {"node": {"require": "./cjs.js", "import": "./esm.js"}, "default": "./esm.js"}, "./other": {"browser": "./web.js", "deno": "./deno.js"}}}`,
		"packages/only/package.json": `{"name": "only", "exports": {"browser": "./web.js"}}`,
	})
	j := NewJavaScript(files, root)
	require.Empty(t, j.Problems())

	tests := []struct {
		name string
		spec string
		form Form
		want []string
		via  string
	}{
		{name: "a require takes the require condition", spec: "kit", form: FormRequire, want: []string{"packages/kit/cjs.js"}, via: "exports-node"},
		{name: "an import takes the import condition", spec: "kit", form: FormImport, want: []string{"packages/kit/esm.js"}, via: "exports-node"},
		{name: "no form is an import", spec: "kit", want: []string{"packages/kit/esm.js"}, via: "exports-node"},
		{name: "a map with no condition of the form gives every target but browser", spec: "kit/other", form: FormImport, want: []string{"packages/kit/deno.js"}, via: "exports-any"},
		{name: "a map of browser only gives no file", spec: "only", form: FormRequire},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, via, ok := j.Resolve("src/main.js", tt.spec, false, tt.form)
			assert.Equal(t, tt.want != nil, ok)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.via, via)
		})
	}
}

func TestJavaScriptPackagesWithNoWorkspaces(t *testing.T) {
	files := NewFiles([]string{"front/index.js", "back/index.js"})
	root := mapFS(map[string]string{
		"front/package.json": `{"name": "front"}`,
		"back/package.json":  `{"name": "back"}`,
	})
	j := NewJavaScript(files, root)
	require.Len(t, j.Packages(), 2)
	assert.Equal(t, "back", j.Packages()[0].Dir)

	got, _, ok := j.Resolve("front/index.js", "back", false, FormImport)
	assert.True(t, ok)
	assert.Equal(t, []string{"back/index.js"}, got)
}

func TestJavaScriptProblems(t *testing.T) {
	files := NewFiles([]string{"a/index.ts", "b/index.ts"})
	root := mapFS(map[string]string{
		"a/package.json":  `{"name": `,
		"b/tsconfig.json": `{"extends": "./tsconfig.json"}`,
	})
	j := NewJavaScript(files, root)
	require.Len(t, j.Problems(), 2)
	assert.Equal(t, "a/package.json", j.Problems()[0].Path)
	assert.Equal(t, "b/tsconfig.json", j.Problems()[1].Path)
	assert.Contains(t, j.Problems()[1].Text, "extends chain longer than 10")
}

func TestTSConfigPaths(t *testing.T) {
	files := NewFiles([]string{"web/src/app/server.ts", "web/src/lib/a.ts", "web/main.ts", "web/src/exact.ts"})
	root := mapFS(map[string]string{
		"tsconfig.base.json": `{"compilerOptions": {"paths": {"@lib/*": ["web/src/lib/*"], "exact": ["web/src/exact.ts"]}}}`,
		"web/tsconfig.json": `{
			// comments and trailing commas
			"extends": "../tsconfig.base.json",
			"compilerOptions": {"baseUrl": "src", "paths": {"@app/*": ["app/*"], "@app/server": ["app/server"],},},
		}`,
	})
	j := NewJavaScript(files, root)
	require.Empty(t, j.Problems())

	tests := []struct {
		spec string
		want string
		via  string
	}{
		{spec: "@app/server", want: "web/src/app/server.ts", via: "tsconfig-paths"},
		{spec: "lib/a", want: "web/src/lib/a.ts", via: "tsconfig-baseurl"},
		{spec: "@lib/a"},
	}
	for _, tt := range tests {
		got, via, ok := j.Resolve("web/main.ts", tt.spec, true, FormImport)
		assert.Equal(t, tt.want != "", ok, tt.spec)
		if tt.want == "" {
			assert.Empty(t, got, tt.spec)
		} else {
			assert.Equal(t, []string{tt.want}, got, tt.spec)
		}
		assert.Equal(t, tt.via, via, tt.spec)
	}

	got, via, ok := j.Resolve("tools/x.ts", "exact", true, FormImport)
	assert.False(t, ok, "a file outside web reads no tsconfig: %s %s", got, via)
}

func TestExportTarget(t *testing.T) {
	exports := `{
		".": {"types": "./index.d.ts", "node": {"import": "./node.mjs"}, "require": "./index.cjs", "default": "./index.js"},
		"./feature/*": {"import": ["./esm/*.mjs"]},
		"./feature/special": "./special.js",
		"./data/*.json": "./json/*.json"
	}`
	tests := []struct {
		sub  string
		form Form
		want []string
		cond string
	}{
		{sub: ".", form: FormImport, want: []string{"./node.mjs"}, cond: "node"},
		{sub: ".", form: FormRequire, want: []string{"./index.cjs"}, cond: "require"},
		{sub: "./feature/x", form: FormImport, want: []string{"./esm/x.mjs"}, cond: "import"},
		{sub: "./feature/x", form: FormRequire, want: []string{"./esm/x.mjs"}, cond: "any"},
		{sub: "./feature/special", want: []string{"./special.js"}},
		{sub: "./data/a.json", want: []string{"./json/a.json"}},
		{sub: "./other"},
	}
	for _, tt := range tests {
		got, cond, ok := exportTargets(json.RawMessage(exports), tt.sub, tt.form)
		assert.Equal(t, tt.want != nil, ok, tt.sub)
		assert.Equal(t, tt.want, got, tt.sub)
		assert.Equal(t, tt.cond, cond, tt.sub)
	}

	got, cond, ok := exportTargets(json.RawMessage(`{"import": "./a.mjs", "default": "./a.js"}`), ".", FormRequire)
	assert.True(t, ok)
	assert.Equal(t, []string{"./a.js"}, got)
	assert.Equal(t, "default", cond)

	_, _, ok = exportTargets(json.RawMessage(`"./a.js"`), "./b", FormImport)
	assert.False(t, ok)
}

func TestPackageJSONFields(t *testing.T) {
	pj, err := parsePackageJSON("", []byte(`{
		"bin": {"z": "./bin/z.js", "a": "./bin/a.js"},
		"exports": {".": {"import": "./a.mjs", "require": "./a.cjs"}, "./x/*": "./x/*.js"}
	}`))
	require.NoError(t, err)
	assert.Equal(t, []string{"./bin/a.js", "./bin/z.js"}, pj.BinFiles())
	assert.Equal(t, []string{"./a.cjs", "./a.mjs"}, pj.ExportFiles())

	pj, err = parsePackageJSON("", []byte(`{"bin": "cli.js"}`))
	require.NoError(t, err)
	assert.Equal(t, []string{"cli.js"}, pj.BinFiles())
}

func TestStripJSONC(t *testing.T) {
	in := `{
		// a line comment with "quotes"
		"a": "http://x/*not a comment*/", /* a block */
		"b": [1, 2,],
		"c": {"d": "e\"f", },
	}`
	var v map[string]any
	require.NoError(t, json.Unmarshal(StripJSONC([]byte(in)), &v))
	assert.Equal(t, "http://x/*not a comment*/", v["a"])
	assert.Equal(t, []any{1.0, 2.0}, v["b"])
	assert.Equal(t, map[string]any{"d": `e"f`}, v["c"])
}

func TestSplitPackage(t *testing.T) {
	tests := []struct{ spec, name, sub string }{
		{"lodash", "lodash", "."},
		{"lodash/fp/map", "lodash", "./fp/map"},
		{"@acme/core", "@acme/core", "."},
		{"@acme/core/util", "@acme/core", "./util"},
	}
	for _, tt := range tests {
		name, sub := SplitPackage(tt.spec)
		assert.Equal(t, tt.name, name)
		assert.Equal(t, tt.sub, sub)
	}
}
