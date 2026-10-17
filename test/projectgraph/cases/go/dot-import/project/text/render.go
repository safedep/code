package text

import "html/template"

func Render(name string) {
	template.New(name)
}
