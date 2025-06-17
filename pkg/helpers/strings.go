package helpers

func TrimWithEllipsis(s string, maxLength int, showEnding bool) string {
	if len(s) <= maxLength {
		return s
	}

	if maxLength <= 5 {
		return s[:maxLength] // not enough space for 5 dots and both sides
	}

	dots := "....."
	remaining := maxLength - len(dots)

	if showEnding {
		// split remaining between prefix and suffix
		prefixLength := remaining / 2
		suffixLength := remaining - prefixLength
		return s[:prefixLength] + dots + s[len(s)-suffixLength:]
	}

	// Only show the prefix and dots
	return s[:remaining] + dots
}
