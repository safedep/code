package roots

import (
	"testing"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultRules(t *testing.T) {
	rules := Default()
	ids := make([]string, 0, len(rules))
	for _, r := range rules {
		ids = append(ids, r.ID)
	}
	assert.Equal(t, []string{
		"go.main", "java.main", "python.main-guard", "python.main-guard-single-quote", "node.package-json",
		"python.project-scripts", "python.setup-py", "procfile", "dockerfile",
		"python.flask.route", "python.fastapi.route", "python.django.urls", "python.django.view", "python.click.command",
		"python.click.subcommand", "python.typer.command", "node.express.route", "node.koa.route",
		"node.commander.action", "node.next.route", "node.serverless.handler", "java.spring.web", "java.spring.component", "test.files",
	}, ids)
}

func TestParseKeys(t *testing.T) {
	rules, err := Parse([]byte(`version: 1
rules:
  - id: a.definition
    language: python
    kind: main
    match: { definition: { name: main, parent: App } }
  - id: a.decorator
    language: python
    kind: framework
    match: { decorator: ["@app.route"] }
  - id: a.annotation
    language: java
    kind: framework
    match: { annotation: ["@RestController"] }
  - id: a.call
    language: python
    kind: framework
    match: { call: { callee: ["django.urls//path"], arg: 1, shape: name } }
  - id: a.export
    language: javascript
    kind: framework
    match: { export: { names: [handler], path: ["**/handler.*"] } }
  - id: a.path
    language: typescript
    kind: framework
    match: { path: ["app/**/route.ts"] }
  - id: a.manifest
    language: javascript
    kind: manifest
    match: { manifest: { file: package.json, fields: [bin] } }
  - id: a.file-contains
    language: python
    kind: main
    match: { file_contains: "if True" }
  - id: a.test-file
    language: any
    kind: test
    match: { test_file: true }
`))
	require.NoError(t, err)
	require.Len(t, rules, 9)
	assert.Equal(t, "App", rules[0].Match.Definition.Parent)
	require.NotNil(t, rules[3].Match.Call.Arg)
	assert.Equal(t, 1, *rules[3].Match.Call.Arg)
	assert.Equal(t, Shapes{fileindex.ArgName}, rules[3].Match.Call.Shape)
	assert.Equal(t, []string{"bin"}, rules[6].Match.Manifest.Fields)
}

func TestParseErrorsNameTheRule(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{name: "an unknown field", yaml: "{id: x.y, language: python, kind: main, match: {file_contains: a}, extra: 1}", want: `root rule "x.y": unknown key "extra"`},
		{name: "a bad type", yaml: "{id: x.y, language: python, kind: [main], match: {file_contains: a}}", want: `root rule "x.y": `},
		{name: "an unknown kind", yaml: "{id: x.y, language: python, kind: route, match: {file_contains: a}}", want: `root rule "x.y": unknown kind "route"`},
		{name: "an unknown language", yaml: "{id: x.y, language: cobol, kind: main, match: {file_contains: a}}", want: `root rule "x.y": unknown language "cobol"`},
		{name: "two match keys", yaml: "{id: x.y, language: any, kind: main, match: {file_contains: a, test_file: true}}", want: `root rule "x.y": the match has 2 keys, want 1`},
		{name: "no match key", yaml: "{id: x.y, language: any, kind: main, match: {}}", want: `the match has 0 keys`},
		{name: "a bad glob", yaml: "{id: x.y, language: any, kind: framework, match: {path: ['a/[']}}", want: `root rule "x.y": glob`},
		{name: "an unknown manifest", yaml: "{id: x.y, language: any, kind: manifest, match: {manifest: {file: Gemfile}}}", want: `unknown file "Gemfile"`},
		{name: "an unknown manifest field", yaml: "{id: x.y, language: any, kind: manifest, match: {manifest: {file: package.json, fields: [scripts]}}}", want: `package.json has no field "scripts"`},
		{name: "an unknown shape", yaml: "{id: x.y, language: any, kind: framework, match: {call: {callee: [a], arg: 0, shape: [name, list]}}}", want: `unknown shape "list"`},
		{name: "a shape with no arg", yaml: "{id: x.y, language: any, kind: framework, match: {call: {callee: [a], shape: name}}}", want: `root rule "x.y": call: a shape needs an arg`},
		{name: "an unknown match key", yaml: "{id: x.y, language: python, kind: framework, match: {decorators: [\"@app.route\"]}}", want: `root rule "x.y": unknown key "decorators"`},
		{name: "an unknown call key", yaml: "{id: x.y, language: python, kind: framework, match: {call: {callee: [a], receiver: [b]}}}", want: `root rule "x.y": unknown key "receiver"`},
		{name: "a call with no callee", yaml: "{id: x.y, language: any, kind: framework, match: {call: {arg: 1}}}", want: `call: no callee`},
		{name: "a definition with no name", yaml: "{id: x.y, language: any, kind: main, match: {definition: {parent: A}}}", want: `definition: no name`},
		{name: "no id", yaml: "{language: any, kind: main, match: {test_file: true}}", want: `root rule #1: no id`},
		{name: "an unknown root", yaml: "{id: x.y, language: python, kind: framework, match: {call: {callee: [a], root: caller}}}", want: `call: unknown root "caller"`},
		{name: "a root receiver with an arg", yaml: "{id: x.y, language: python, kind: framework, match: {call: {callee: [a], arg: 0, root: receiver}}}", want: `call: a root receiver takes no arg`},
		{name: "requires with no manifest", yaml: "{id: x.y, language: go, kind: main, requires: [gin], match: {test_file: true}}", want: `requires: the language "go" has no manifest of dependencies`},
		{name: "requires an empty name", yaml: "{id: x.y, language: python, kind: main, requires: [''], match: {test_file: true}}", want: `requires: an empty package name`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte("version: 1\nrules:\n  - " + tt.yaml + "\n"))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}

	_, err := Parse([]byte("version: 1\nrules:\n  - {id: a, language: any, kind: test, match: {test_file: true}}\n  - {id: a, language: any, kind: test, match: {test_file: true}}\n"))
	assert.ErrorContains(t, err, `root rule "a": the id is not unique`)

	_, err = Parse([]byte("version: 2\nrules: []\n"))
	assert.ErrorContains(t, err, "version 2, want 1")
}

func TestMerge(t *testing.T) {
	base := []Rule{{ID: "a", Kind: Main}, {ID: "b", Kind: Main}}
	extra := []Rule{{ID: "c", Kind: Test}, {ID: "a", Kind: Framework}}
	got := Merge(base, extra)
	assert.Equal(t, []Rule{{ID: "a", Kind: Framework}, {ID: "b", Kind: Main}, {ID: "c", Kind: Test}}, got)
	assert.Equal(t, Main, base[0].Kind, "the base rules stay as they are")
}

func TestApplies(t *testing.T) {
	assert.True(t, Rule{Language: "javascript"}.Applies(langcode.Typescript))
	assert.True(t, Rule{Language: AnyLanguage}.Applies(langcode.Go))
	assert.False(t, Rule{Language: "python"}.Applies(langcode.Javascript))
}
