package store

import "gopkg.in/yaml.v3"

func Save(name string) {
	encode(name)
}
