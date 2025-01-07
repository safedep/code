package helpers

// GetFirstNonEmptyString returns the first non-empty string from the given list of strings.
// eg. for GetFirstNonEmptyString("", "", "foo", "bar"), it returns "foo"
func GetFirstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return "" // return empty string if none are non-empty
}
