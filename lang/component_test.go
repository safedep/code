package lang

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// part is a piece of a component file. A code part stays in the source, and
// componentSource blanks every other part.
type part struct {
	text string
	code bool
}

func markup(text string) part { return part{text: text} }
func code(text string) part   { return part{text: text, code: true} }

func TestComponentSource(t *testing.T) {
	cases := []struct {
		name    string
		ext     string
		parts   []part
		dialect scriptDialect
	}{
		{
			name:  "a file with no script block gives no code",
			ext:   ".vue",
			parts: []part{markup("<template>\n  <div>{{ msg }}</div>\n</template>\n")},
		},
		{
			name: "two blocks give their code at their offsets",
			ext:  ".vue",
			parts: []part{
				markup("<script>"), code("\nexport default { name: 'App' }\n"), markup("</script>\n<template><p/></template>\n<SCRIPT setup>"),
				code("\nimport pad from 'left-pad'\n"), markup("</SCRIPT>\n"),
			},
		},
		{
			name: "lang ts gives TypeScript",
			ext:  ".svelte",
			parts: []part{
				markup(`<script context="module" lang="ts">`), code("\nlet n: number = 1\n"), markup("</script>\n<h1>{n}</h1>\n"),
			},
			dialect: dialectTypescript,
		},
		{
			name: "lang typescript gives TypeScript and a quoted > stays in the tag",
			ext:  ".vue",
			parts: []part{
				markup(`<script data-x="a>b" lang='typescript'>`), code("let n: number = 1"), markup("</script>"),
			},
			dialect: dialectTypescript,
		},
		{
			name: "lang tsx gives TSX over the TypeScript of another block",
			ext:  ".vue",
			parts: []part{
				markup(`<script lang="ts">`), code("let n = 1\n"), markup("</script>\n<script setup lang=tsx>"), code("const e = <a/>\n"), markup("</script>\n"),
			},
			dialect: dialectTSX,
		},
		{
			name: "a block with a src attribute gives no code and no dialect",
			ext:  ".vue",
			parts: []part{
				markup(`<script src="./app.ts" lang="ts">ignored()</script>`), markup("\n<template><p/></template>\n"),
			},
		},
		{
			name: "a block that does not close runs to the end of the file",
			ext:  ".svelte",
			parts: []part{
				markup("<p>hi</p>\n<script>"), code("\nimport pad from 'left-pad'\npad('a', 2)\n"),
			},
		},
		{
			name: "a start tag that does not close gives no code",
			ext:  ".svelte",
			parts: []part{
				markup("<p>hi</p>\n<script lang=\"ts\"\nimport pad from 'left-pad'\n"),
			},
		},
		{
			name: "a rune of several bytes keeps the offset of the code",
			ext:  ".svelte",
			parts: []part{
				markup("<p>café</p><script>"), code("import pad from 'left-pad'"), markup("</script>"),
			},
		},
		{
			name: "a longer tag name is not a script",
			ext:  ".vue",
			parts: []part{
				markup("<scripts>\nnot code\n</scripts>\n"),
			},
		},
		{
			name: "Astro reads the fence and the scripts as TypeScript",
			ext:  ".astro",
			parts: []part{
				markup("\n---\n"), code("import pad from 'left-pad'\nconst title: string = pad('a', 2)\n"), markup("---\n<h1>{title}</h1>\n<script>"),
				code("\nimport confetti from 'canvas-confetti'\n"), markup("</script>\n"),
			},
			dialect: dialectTypescript,
		},
		{
			name: "an Astro fence that does not close runs to the end of the file",
			ext:  ".astro",
			parts: []part{
				markup("---\n"), code("import pad from 'left-pad'\n<script>x()</script>\n"),
			},
			dialect: dialectTypescript,
		},
		{
			name: "an Astro file with no fence reads its scripts",
			ext:  ".astro",
			parts: []part{
				markup("<h1>---</h1>\n<script>"), code("x()"), markup("</script>"),
			},
			dialect: dialectTypescript,
		},
		{
			name: "CRLF line ends stay in the markup",
			ext:  ".vue",
			parts: []part{
				markup("<template>\r\n  <p/>\r\n</template>\r\n<script setup>"), code("\r\nimport pad from 'left-pad'\r\n"), markup("</script>\r\n"),
			},
		},
		{
			name: "an Astro fence with CRLF line ends",
			ext:  ".astro",
			parts: []part{
				markup("---\r\n"), code("import pad from 'left-pad'\r\n"), markup("---\r\n<p/>\r\n"),
			},
			dialect: dialectTypescript,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var data, want strings.Builder
			for _, p := range tc.parts {
				data.WriteString(p.text)
				if p.code {
					want.WriteString(p.text)
				} else {
					want.WriteString(blankExceptLineEnds(p.text))
				}
			}

			source, dialect := componentSource(tc.ext, []byte(data.String()))
			assert.Equal(t, want.String(), string(source))
			assert.Equal(t, tc.dialect, dialect)
		})
	}
}

// blankExceptLineEnds blanks each byte, so a rune of several bytes keeps
// its width in bytes, as the columns of tree-sitter count.
func blankExceptLineEnds(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c != '\n' && c != '\r' {
			b[i] = ' '
		}
	}
	return string(b)
}
