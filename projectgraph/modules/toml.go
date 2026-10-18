package modules

import (
	"regexp"
	"strings"
)

// TOMLPair is one key = value of a TOML file, with the name of its table, as
// project.scripts. Value is the text after the equals sign. A list that
// spans lines is one value.
type TOMLPair struct {
	Table, Key, Value string
}

var (
	tomlTable  = regexp.MustCompile(`^\[\[?\s*([^\]]+?)\s*\]\]?`)
	tomlPair   = regexp.MustCompile(`^("[^"]*"|'[^']*'|[A-Za-z0-9_.-]+)\s*=\s*(.*)$`)
	tomlString = regexp.MustCompile(`^("[^"]*"|'[^']*')`)
	tomlInline = regexp.MustCompile(`("[^"]*"|'[^']*'|[A-Za-z0-9_.-]+)\s*=\s*("[^"]*"|'[^']*')`)
)

// ScanTOML returns the pairs of a TOML file in order. It reads what the
// manifests of a project need: the tables, the pairs, and a list that
// spans lines as one value. It reads no other TOML syntax, so a manifest
// that a reader rejects still gives its pairs.
func ScanTOML(data []byte) []TOMLPair {
	var pairs []TOMLPair
	table := ""
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if last := len(pairs) - 1; last >= 0 && openList(pairs[last].Value) {
			pairs[last].Value += "\n" + line
			continue
		}
		if m := tomlTable.FindStringSubmatch(line); m != nil {
			table = unquoteDotted(m[1])
			continue
		}
		if m := tomlPair.FindStringSubmatch(line); m != nil {
			pairs = append(pairs, TOMLPair{Table: table, Key: unquote(m[1]), Value: strings.TrimSpace(m[2])})
		}
	}
	return pairs
}

// openList reports whether a value starts a list that its line does not
// close.
func openList(value string) bool {
	return strings.HasPrefix(value, "[") && !strings.Contains(value, "]")
}

// TOMLString returns the string that a value holds, when the value is a
// string.
func TOMLString(value string) (string, bool) {
	s := tomlString.FindString(value)
	if s == "" {
		return "", false
	}
	return unquote(s), true
}

// TOMLInlineTable returns the pairs of an inline table, as {a = "x"}, whose
// values are strings. It reports whether the value is an inline table.
func TOMLInlineTable(value string) ([]TOMLPair, bool) {
	if !strings.HasPrefix(value, "{") {
		return nil, false
	}
	var pairs []TOMLPair
	for _, m := range tomlInline.FindAllStringSubmatch(value, -1) {
		pairs = append(pairs, TOMLPair{Key: unquote(m[1]), Value: unquote(m[2])})
	}
	return pairs, true
}

func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

// unquoteDotted removes the quotes of the parts of a table name, as
// project.entry-points."flake8.extension".
func unquoteDotted(s string) string {
	return strings.NewReplacer(`"`, "", `'`, "").Replace(s)
}

// matchStar returns the pattern with one * that matches the text, the one
// with the longest prefix before its *, and the text that the * matches. A
// tie takes the pattern that sorts first.
func matchStar(patterns []string, text string) (string, string, bool) {
	best, bestPrefix, star := "", -1, ""
	for _, pattern := range patterns {
		prefix, suffix, ok := strings.Cut(pattern, "*")
		if !ok || len(text) < len(prefix)+len(suffix) ||
			!strings.HasPrefix(text, prefix) || !strings.HasSuffix(text, suffix) {
			continue
		}
		if len(prefix) < bestPrefix || len(prefix) == bestPrefix && pattern > best {
			continue
		}
		best, bestPrefix, star = pattern, len(prefix), text[len(prefix):len(text)-len(suffix)]
	}
	return best, star, bestPrefix >= 0
}
