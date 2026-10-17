package roots

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPyprojectEntries(t *testing.T) {
	data := []byte(`[project]
scripts = { a = "m.a:run", 'b' = 'm.b:run' }
[project.scripts]
"c" = "m.c:main" # a comment
d = "m.d:main [extra]"
[project.entry-points."x.y"]
e = "m.e:E.load"
[[tool.other]]
f = "m.f:no"
`)
	assert.Equal(t, []string{"m.a:run", "m.b:run", "m.c:main", "m.d:main [extra]", "m.e:E.load"},
		pyprojectEntries(data, []string{"project.scripts", "project.entry-points"}))
	assert.Empty(t, pyprojectEntries(data, []string{"tool.poetry.scripts"}))
}

func TestSetupEntries(t *testing.T) {
	data := []byte(`setup(entry_points={'console_scripts': ["a=pkg.a:main", ' b = pkg.b:run ']}, other=["c = no:no"])`)
	assert.Equal(t, []string{"pkg.a:main", "pkg.b:run"}, setupEntries(data))
}

func TestCommands(t *testing.T) {
	assert.Equal(t, [][]string{{"gunicorn", "app:app"}, {"node", "x.js"}},
		procfileCommands([]byte("web: gunicorn app:app\n# skip: me\nworker:node x.js\n")))
	assert.Equal(t, [][]string{{"node", "/srv/a.js"}, {"python", "-m", "app"}},
		dockerfileCommands([]byte("FROM node:20\nCMD [\"node\", \"/srv/a.js\"]\n  entrypoint python -m app\nRUN make\n")))
}
