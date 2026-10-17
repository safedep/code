package sink

import "os"

type File struct{}

func (File) Write(v string) error {
	return os.WriteFile("out", []byte(v), 0o600)
}
