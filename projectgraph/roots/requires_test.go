package roots

import (
	"testing"
	"testing/fstest"

	"github.com/safedep/code/fileindex"
	rules "github.com/safedep/code/fileindex/roots"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/projectgraph/modules"
	"github.com/stretchr/testify/assert"
)

func TestPythonDependencies(t *testing.T) {
	root := fstest.MapFS{
		"pyproject.toml": {Data: []byte(`[project]
name = "app"
dependencies = [
  "Flask>=2.0",
  'requests[socks]',
]
[project.optional-dependencies]
cli = ["Click"]
[tool.poetry.dependencies]
python = "^3.11"
FastAPI = "^0.110"
[tool.poetry.group.dev.dependencies]
pytest = "*"
`)},
		"svc/setup.py":               {Data: []byte(`setup(install_requires=["Django>=4", 'typer'])`)},
		"svc/Pipfile":                {Data: []byte("[packages]\nPyYAML = \"*\"\n[dev-packages]\nblack = \"*\"\n")},
		"svc/requirements-dev.txt":   {Data: []byte("-r base.txt\n-e .\nzope.interface==5 # a comment\n")},
		"svc/requirements/extra.txt": {Data: []byte("python_dateutil\n")},
	}
	files := modules.NewFiles([]string{"app.py", "svc/a.py"})
	indexes := []*fileindex.Index{{Path: "svc/a.py", Language: langcode.Python, Imports: []fileindex.Import{{Module: "quart.app"}, {Module: ".local"}}}}
	f := &finder{
		Project:  Project{Indexes: indexes, Root: root, Files: files, JS: modules.NewJavaScript(files, root)},
		deps:     map[langcode.Code]map[string]bool{},
		imported: map[langcode.Code]map[string]bool{},
	}
	deps := f.dependencies(langcode.Python)
	for _, name := range []string{"flask", "requests", "click", "fastapi", "pytest", "django", "typer", "pyyaml", "black", "zope-interface", "python-dateutil"} {
		assert.True(t, deps[name], name)
	}
	assert.False(t, deps["python"], "the python version of Poetry is no package")
	assert.False(t, deps["base-txt"], "an option line names no package")

	flask := rules.Rule{ID: "f", Language: "python", Requires: []string{"Flask"}}
	next := rules.Rule{ID: "n", Language: "javascript", Requires: []string{"next"}}
	assert.True(t, f.requires(flask), "a name matches in its normal form")
	assert.True(t, f.requires(next), "a project with no package.json gives no evidence")
	assert.True(t, f.requires(rules.Rule{ID: "q", Language: "python", Requires: []string{"quart"}}), "an import shows a package that the manifests miss")
	assert.False(t, f.requires(rules.Rule{ID: "s", Language: "python", Requires: []string{"sanic"}}))
}

func TestPackageOf(t *testing.T) {
	assert.Equal(t, "flask", packageOf(langcode.Python, "flask.views"))
	assert.Equal(t, "@koa/router", packageOf(langcode.Javascript, "@koa/router/lib"))
	assert.Equal(t, "next", packageOf(langcode.Javascript, "next/router"))
	assert.Empty(t, packageOf(langcode.Javascript, "./util"))
}
