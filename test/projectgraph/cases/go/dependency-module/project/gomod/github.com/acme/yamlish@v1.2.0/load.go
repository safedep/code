package yamlish

func Load(text string) error {
	var out map[string]any
	return decode(text, &out)
}

func Dump(v any) string {
	return ""
}
