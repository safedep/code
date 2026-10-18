package yamlish

import "gopkg.in/yaml.v3"

func decode(text string, out any) error {
	return yaml.Unmarshal([]byte(text), out)
}
