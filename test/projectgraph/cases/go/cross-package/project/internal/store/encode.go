package store

import "gopkg.in/yaml.v3"

func encode(name string) ([]byte, error) {
	return yaml.Marshal(name)
}

func unused() {
	yaml.Unmarshal(nil, nil)
}
