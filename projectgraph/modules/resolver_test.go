package modules

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
)

func TestResolvers(t *testing.T) {
	files := NewFiles([]string{"app/main.py", "app/util.py", "web/src/index.ts", "web/src/util.ts"})
	root := fstest.MapFS{}
	tests := []struct {
		name       string
		r          Resolver
		importer   string
		spec       string
		module     string
		via        string
		found      bool
		firstParty bool
	}{
		{name: "a Python relative module", r: NewPython(files, root, nil), importer: "app/main.py", spec: ".util", module: "app/util.py", via: "relative", found: true, firstParty: true},
		{name: "a Python absolute module", r: NewPython(files, root, nil), importer: "app/main.py", spec: "app.util", module: "app/util.py", via: "absolute", found: true, firstParty: true},
		{name: "a Python dependency", r: NewPython(files, root, nil), importer: "app/main.py", spec: "yaml", via: "absolute"},
		{name: "a TypeScript relative module", r: NewJavaScript(files, root).For(true), importer: "web/src/index.ts", spec: "./util", module: "web/src/util.ts", via: "relative", found: true, firstParty: true},
		{name: "a TypeScript alias that names a directory of the project", r: NewJavaScript(files, root).For(true), importer: "web/src/index.ts", spec: "src/util", firstParty: true},
		{name: "a JavaScript dependency", r: NewJavaScript(files, root).For(false), importer: "web/src/index.ts", spec: "lodash"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module, via, found := tt.r.Module(tt.importer, tt.spec)
			assert.Equal(t, tt.module, module)
			assert.Equal(t, tt.via, via)
			assert.Equal(t, tt.found, found)
			assert.Equal(t, tt.firstParty, tt.r.FirstParty(tt.spec))
		})
	}
}
