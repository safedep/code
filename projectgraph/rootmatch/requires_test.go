package rootmatch

import (
	"testing"
	"testing/fstest"

	"github.com/safedep/code/fileindex"
	rules "github.com/safedep/code/fileindex/roots"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/projectgraph/modules"
	"github.com/stretchr/testify/assert"
)

func TestRequires(t *testing.T) {
	root := fstest.MapFS{
		"pyproject.toml": {Data: []byte("[project]\ndependencies = [\"Flask>=2.0\"]\n")},
	}
	files := modules.NewFiles([]string{"app.py", "svc/a.py", "cmd/main.go"})
	indexes := []*fileindex.Index{
		{Path: "svc/a.py", Language: langcode.Python, Imports: []fileindex.Import{{Module: "quart.app"}, {Module: ".local"}}},
		{Path: "cmd/main.go", Language: langcode.Go, Imports: []fileindex.Import{{Module: "github.com/gin-gonic/gin/binding"}}},
	}
	f := &finder{
		Project: Project{Indexes: indexes, Root: root, Files: files, Dependencies: func(lang langcode.Code) ([]string, []modules.Problem) {
			switch lang {
			case langcode.Python:
				return modules.PythonDependencies(root, files.Dirs())
			case langcode.Go:
				return []string{"example.com/other"}, nil
			}
			return nil, nil
		}},
		deps:     map[langcode.Code]map[string]bool{},
		imported: map[langcode.Code]map[string]bool{},
	}
	assert.True(t, f.dependencies(langcode.Python)["flask"], "a name is in its normal form")

	flask := rules.Rule{ID: "f", Language: "python", Requires: []string{"Flask"}}
	next := rules.Rule{ID: "n", Language: "javascript", Requires: []string{"next"}}
	assert.True(t, f.requires(flask), "a name matches in its normal form")
	assert.True(t, f.requires(next), "a project with no package.json gives no evidence")
	assert.True(t, f.requires(rules.Rule{ID: "q", Language: "python", Requires: []string{"quart"}}), "an import shows a package that the manifests miss")
	assert.False(t, f.requires(rules.Rule{ID: "s", Language: "python", Requires: []string{"sanic"}}))
	assert.True(t, f.requires(rules.Rule{ID: "g", Language: "go", Requires: []string{"github.com/gin-gonic/gin"}}), "an import under the module path shows the dependency")
	assert.False(t, f.requires(rules.Rule{ID: "h", Language: "go", Requires: []string{"github.com/other/pkg"}}), "no manifest and no import names the module")
	assert.True(t, f.requires(rules.Rule{ID: "r", Language: "rust", Requires: []string{"serde"}}), "a Cargo.toml declares nothing, so the rule applies")
}

func TestPackageOf(t *testing.T) {
	assert.Equal(t, "flask", packageOf(langcode.Python, "flask.views"))
	assert.Equal(t, "@koa/router", packageOf(langcode.Javascript, "@koa/router/lib"))
	assert.Equal(t, "next", packageOf(langcode.Javascript, "next/router"))
	assert.Empty(t, packageOf(langcode.Javascript, "./util"))
	assert.Equal(t, "serde_json", packageOf(langcode.Rust, "serde_json::Value"))
	assert.Equal(t, "github.com/x/y/z", packageOf(langcode.Go, "github.com/x/y/z"))
	assert.Empty(t, packageOf(langcode.PHP, `App\Kernel`))
	assert.Equal(t, "serde_json", normalizeDependency(langcode.Rust, "serde-json"))
}
