package ecmascript

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// unescape returns the value of the text between the quotes of a string or
// a template, or the text of an identifier. Code hides a name from a search
// of the text with an escape, as in require('child\x5fprocess') or
// eval(x), and the value is the name that runs.
func unescape(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 == len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch c := s[i]; c {
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'v':
			b.WriteByte('\v')
		case '0':
			b.WriteByte(0)
		case '\n':
		case '\r':
			if i+1 < len(s) && s[i+1] == '\n' {
				i++
			}
		case 'x':
			if r, ok := hexRune(s, i+1, 2); ok {
				b.WriteRune(r)
				i += 2
			} else {
				b.WriteByte(c)
			}
		case 'u':
			r, n := unicodeEscape(s, i+1)
			if n == 0 {
				b.WriteByte(c)
				continue
			}
			b.WriteRune(r)
			i += n
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// unicodeEscape reads the digits of \uHHHH or \u{H...} that start at i. It
// returns the rune and the number of bytes that it read, or zero.
func unicodeEscape(s string, i int) (rune, int) {
	if i < len(s) && s[i] == '{' {
		end := strings.IndexByte(s[i:], '}')
		if end < 2 || end > 7 {
			return 0, 0
		}
		r, ok := hexRune(s, i+1, end-1)
		if !ok {
			return 0, 0
		}
		return r, end + 1
	}
	r, ok := hexRune(s, i, 4)
	if !ok {
		return 0, 0
	}
	return r, 4
}

func hexRune(s string, i, n int) (rune, bool) {
	if i+n > len(s) {
		return 0, false
	}
	v, err := strconv.ParseUint(s[i:i+n], 16, 32)
	if err != nil || !utf8.ValidRune(rune(v)) {
		return 0, false
	}
	return rune(v), true
}
