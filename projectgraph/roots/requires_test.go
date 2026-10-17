package roots

import (
	"testing"
	"testing/fstest"

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
	f := &finder{Project: Project{Root: root, Files: files, JS: modules.NewJavaScript(files, root)}, deps: map[langcode.Code]map[string]bool{}}
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
	assert.False(t, f.requires(rules.Rule{ID: "q", Language: "python", Requires: []string{"quart"}}))
}
