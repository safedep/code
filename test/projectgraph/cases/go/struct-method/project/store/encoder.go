package store

import "gopkg.in/yaml.v3"

type Encoder struct{}

func (e *Encoder) Encode(v any) ([]byte, error) {
	return yaml.Marshal(v)
}

func (e *Encoder) Remove(v any) {
	yaml.Unmarshal(nil, v)
}
