package fileindex

import (
	"fmt"
	"strings"
	"testing"

	"github.com/safedep/code/core"
	"github.com/stretchr/testify/assert"
)

func TestFileIndexReferences(t *testing.T) {
	tests := []struct {
		name   string
		code   core.LanguageCode
		file   string
		source []string
		want   []string
	}{
		{
			name: "python values",
			code: core.LanguageCodePython,
			file: "app.py",
			source: []string{
				"import yaml",
				"from tasks import run",
				"",
				"HANDLERS = {'yaml': yaml.load}",
				"",
				"def process(text, cb=yaml.safe_load):",
				"    local = 1",
				"    run(process, local, key=yaml.dump)",
				"    return [process, print]",
				"",
				"class Handler:",
				"    def handle(self):",
				"        run(self.handle)",
			},
			want: []string{
				"app.py 4 yaml.load yaml//load",
				"app.py//process 6 yaml.safe_load yaml//safe_load",
				"app.py//process 8 process app.py//process",
				"app.py//process 8 yaml.dump yaml//dump",
				"app.py//process 9 process app.py//process",
				"app.py//Handler//handle 13 self.handle app.py//Handler//handle",
			},
		},
		{
			name: "javascript values",
			code: core.LanguageCodeJavascript,
			file: "app.js",
			source: []string{
				"const { load } = require('js-yaml');",
				"const yaml = require('js-yaml');",
				"function parse(text) { return text; }",
				"const handlers = [load, yaml.dump];",
				"const api = { parse, other: yaml.loadAll };",
				"Promise.resolve('a').then(parse || load);",
				"module.exports = { parse, dump: yaml.dump };",
			},
			want: []string{
				"app.js 4 load js-yaml//load",
				"app.js 4 yaml.dump js-yaml//dump",
				"app.js 5 parse app.js//parse",
				"app.js 5 yaml.loadAll js-yaml//loadAll",
				"app.js 6 parse app.js//parse",
				"app.js 6 load js-yaml//load",
				"app.js 7 yaml.dump js-yaml//dump",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := indexSource(t, tt.code, tt.file, strings.Join(tt.source, "\n")+"\n")
			var got []string
			for _, ref := range ix.References {
				got = append(got, fmt.Sprintf("%s %d %s %s", ref.Caller, ref.Line, ref.Text, strings.Join(ref.Targets, ",")))
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFileIndexClassBasesAreNoReferences(t *testing.T) {
	ix := indexSource(t, core.LanguageCodePython, "a.py", "class Base:\n    pass\n\n\nclass Strict(Base):\n    pass\n")
	assert.Empty(t, ix.References)
}
