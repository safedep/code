package modules

import (
	"testing"
	"testing/fstest"

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
	names, problems := PythonDependencies(root, []string{"", "svc"})
	assert.Empty(t, problems)
	assert.Equal(t, []string{"Flask", "requests", "Click", "FastAPI", "pytest", "Django", "typer", "PyYAML", "black", "zope.interface", "python_dateutil"}, names)

	names, _ = PythonDependencies(nil, []string{""})
	assert.Empty(t, names, "a nil root reads no manifest")
}

func TestCargoDependencies(t *testing.T) {
	root := fstest.MapFS{
		"Cargo.toml":  {Data: []byte("[package]\nname = \"tool\"\n\n[dependencies]\nserde = \"1\"\n\n[dependencies.tokio]\nversion = \"1\"\n\n[dev-dependencies]\ntempfile = \"3\"\n\n[target.'cfg(unix)'.dependencies]\nlibc = \"0.2\"\n")},
		"src/main.rs": {Data: []byte("")},
	}
	c := NewCargo(NewFiles([]string{"src/main.rs"}), root)
	assert.Equal(t, []string{"serde", "tokio", "tempfile", "libc"}, c.Dependencies())
}

func TestComposerDependencies(t *testing.T) {
	root := fstest.MapFS{
		"composer.json": {Data: []byte(`{"require": {"php": ">=8", "monolog/monolog": "^3"}, "require-dev": {"phpunit/phpunit": "^10"}}`)},
		"index.php":     {Data: []byte("")},
	}
	c := NewComposer(NewFiles([]string{"index.php"}), root)
	assert.Equal(t, []string{"monolog/monolog", "php", "phpunit/phpunit"}, c.Dependencies())
}

func TestGolangDependencies(t *testing.T) {
	root := fstest.MapFS{
		"go.mod":  {Data: []byte("module example.com/app\n\ngo 1.22\n\nrequire (\n\tgithub.com/gin-gonic/gin v1.9.0\n\tgolang.org/x/mod v0.1.0 // indirect\n)\n")},
		"main.go": {Data: []byte("")},
	}
	g := NewGolang(NewFiles([]string{"main.go"}), root, []string{""})
	assert.Equal(t, []string{"github.com/gin-gonic/gin", "golang.org/x/mod"}, g.Dependencies())
}

func TestOwnNamespaces(t *testing.T) {
	root := fstest.MapFS{
		"go.mod":                         {Data: []byte("module example.com/app\n\nrequire github.com/x/y v1.0.0\n")},
		"tools/go.mod":                   {Data: []byte("module example.com/app/tools\n")},
		"vendor/github.com/x/y/go.mod":   {Data: []byte("module github.com/x/y\n")},
		"node_modules/pkg/composer.json": {Data: []byte(`{"autoload": {"psr-4": {"Pkg\\": "src/"}}}`)},
		"crates/core/Cargo.toml":         {Data: []byte("[package]\nname = \"app-core\"\n")},
		"php/composer.json":              {Data: []byte(`{"autoload": {"psr-4": {"App\\": "src/", "App\\Tests\\": "tests/"}}}`)},
		".hidden/go.mod":                 {Data: []byte("module hidden\n")},
	}
	names, problems := OwnNamespaces(root)
	assert.Empty(t, problems)
	assert.Equal(t, []string{`App\`, `App\Tests\`, "app_core", "example.com/app", "example.com/app/tools"}, names, "vendored, installed and hidden manifests are skipped")

	names, problems = OwnNamespaces(nil)
	assert.Empty(t, names)
	assert.Empty(t, problems)
}
