package helpers

import (
	"strings"
)

// TrimWithEllipsis trims the string `s` to `maxLength` characters.
// If `centered` is true, it shows the start and end of the string with ellipsis in the middle.
// The ellipsis length is controlled by `dots`.
// If remaining characters after dots are odd, extra character is shown on the prefix side.
func TrimWithEllipsis(s string, maxLength int, centered bool, dots int) string {
	if maxLength <= 0 || dots < 0 {
		return ""
	}

	if len(s) <= maxLength {
		return s
	}

	if dots == 0 || maxLength <= dots {
		return s[:maxLength]
	}

	ellipsis := strings.Repeat(".", dots)

	if !centered {
		trimLen := maxLength - dots
		if trimLen <= 0 {
			return s[:maxLength]
		}
		return s[:trimLen] + ellipsis
	}

	remaining := maxLength - dots
	if remaining <= 0 {
		return s[:maxLength]
	}

	// Extra character to prefix if odd
	leftLen := (remaining + 1) / 2
	rightLen := remaining - leftLen

	return s[:leftLen] + ellipsis + s[len(s)-rightLen:]
}
