package lang

import (
	"bytes"
	"strings"
)

// componentExtensions are the extensions of the component files of Vue,
// Svelte and Astro. A component file holds its code in script blocks among
// markup.
var componentExtensions = []string{".vue", ".svelte", ".astro"}

// scriptDialect is the language of a script block. A greater dialect parses
// the code of a lesser one.
type scriptDialect int

const (
	dialectJavascript scriptDialect = iota
	dialectTypescript
	dialectTSX
)

// componentSource returns data with each byte outside the script blocks
// replaced by a space, and the dialect of the blocks. The line ends stay, so
// a node of the code reports its line and column in the file. One parse
// reads every block, as a Vue <script> and <script setup>, and the greatest
// dialect of a block parses all of them. Astro reads the code between the
// two --- lines at the top of the file, and every script, as TypeScript.
// A block that does not close runs to the end of the file, as an HTML
// parser reads it.
func componentSource(ext string, data []byte) ([]byte, scriptDialect) {
	source := make([]byte, len(data))
	for i, b := range data {
		if b == '\n' || b == '\r' {
			source[i] = b
		} else {
			source[i] = ' '
		}
	}

	dialect, from := dialectJavascript, 0
	if ext == ".astro" {
		dialect = dialectTypescript
		start, end, next := astroFence(data)
		copy(source[start:end], data[start:end])
		from = next
	}

	lower := asciiLower(data)
	for from < len(data) {
		block, ok := scriptBlock(data, lower, from)
		if !ok {
			break
		}
		if _, external := block.attrs["src"]; !external {
			copy(source[block.start:block.end], data[block.start:block.end])
			dialect = max(dialect, dialectOf(block.attrs["lang"]))
		}
		from = block.next
	}
	return source, dialect
}

func dialectOf(lang string) scriptDialect {
	switch strings.ToLower(lang) {
	case "ts", "typescript":
		return dialectTypescript
	case "tsx":
		return dialectTSX
	default:
		return dialectJavascript
	}
}

// astroFence returns the bounds of the code between the --- line at the top
// of an Astro file and the next --- line, and the offset after the fence. A
// file with no fence gives an empty code at 0.
func astroFence(data []byte) (start, end, next int) {
	open := len(data) - len(bytes.TrimLeft(data, " \t\r\n"))
	line, after := lineAt(data, open)
	if string(line) != "---" {
		return 0, 0, 0
	}
	for i := after; i < len(data); {
		line, lineEnd := lineAt(data, i)
		if string(line) == "---" {
			return after, i, lineEnd
		}
		i = lineEnd
	}
	return after, len(data), len(data)
}

// lineAt returns the line that starts at i, with no trailing space or line
// end, and the offset of the next line.
func lineAt(data []byte, i int) ([]byte, int) {
	end := bytes.IndexByte(data[i:], '\n')
	if end < 0 {
		return bytes.TrimRight(data[i:], " \t\r"), len(data)
	}
	return bytes.TrimRight(data[i:i+end], " \t\r"), i + end + 1
}

// scriptElement is a script element of a file: the bounds of its code, the
// offset after its end tag, and the attributes of its start tag by lower
// case name.
type scriptElement struct {
	start, end, next int
	attrs            map[string]string
}

// scriptBlock finds the first script element at or after from. lower is
// data in ASCII lower case.
func scriptBlock(data, lower []byte, from int) (scriptElement, bool) {
	open := tagAt(lower, "<script", from)
	if open < 0 {
		return scriptElement{}, false
	}
	attrs, start, selfClosing := startTag(data, open+len("<script"))
	if start < 0 {
		return scriptElement{}, false
	}
	block := scriptElement{start: start, end: len(data), next: len(data), attrs: attrs}
	if selfClosing {
		block.end, block.next = start, start
		return block, true
	}
	end := tagAt(lower, "</script", start)
	if end < 0 {
		return block, true
	}
	block.end = end
	if next := bytes.IndexByte(data[end:], '>'); next >= 0 {
		block.next = end + next + 1
	}
	return block, true
}

// tagAt returns the offset of the first tag name at or after from, or -1.
// The name must end, so <scripts> is not <script.
func tagAt(lower []byte, name string, from int) int {
	for from < len(lower) {
		i := bytes.Index(lower[from:], []byte(name))
		if i < 0 {
			return -1
		}
		i += from
		after := i + len(name)
		if after == len(lower) || isSpace(lower[after]) || lower[after] == '/' || lower[after] == '>' {
			return i
		}
		from = after
	}
	return -1
}

// startTag reads the attributes of a start tag from i, after the tag name.
// It returns the attributes, the offset after the tag, or -1 when the tag
// does not end, and whether the tag ends with />.
func startTag(data []byte, i int) (map[string]string, int, bool) {
	attrs := map[string]string{}
	for i < len(data) {
		switch c := data[i]; {
		case c == '>':
			return attrs, i + 1, false
		case c == '/' && i+1 < len(data) && data[i+1] == '>':
			return attrs, i + 2, true
		case isSpace(c) || c == '/':
			i++
		default:
			var name, value string
			name, value, i = attribute(data, i)
			attrs[strings.ToLower(name)] = value
		}
	}
	return attrs, -1, false
}

// attribute reads one attribute from i, as name, name=value, name="value"
// or name='value', and returns the offset after it.
func attribute(data []byte, i int) (name, value string, next int) {
	start := i
	for i < len(data) && !isSpace(data[i]) && data[i] != '=' && data[i] != '>' && data[i] != '/' {
		i++
	}
	name = string(data[start:i])
	for i < len(data) && isSpace(data[i]) {
		i++
	}
	if i == len(data) || data[i] != '=' {
		return name, "", i
	}
	i++
	for i < len(data) && isSpace(data[i]) {
		i++
	}
	if i < len(data) && (data[i] == '"' || data[i] == '\'') {
		quote := data[i]
		end := bytes.IndexByte(data[i+1:], quote)
		if end < 0 {
			return name, string(data[i+1:]), len(data)
		}
		return name, string(data[i+1 : i+1+end]), i + end + 2
	}
	start = i
	for i < len(data) && !isSpace(data[i]) && data[i] != '>' {
		i++
	}
	return name, string(data[start:i]), i
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\f'
}

// asciiLower lowers the ASCII letters only, so each byte keeps its offset.
func asciiLower(data []byte) []byte {
	lower := make([]byte, len(data))
	for i, c := range data {
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		lower[i] = c
	}
	return lower
}
