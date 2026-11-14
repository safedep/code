package exec

type Cmd struct{}

func Command(name string, arg ...string) *Cmd {
	return &Cmd{}
}

func LookPath(file string) (string, error) {
	return file, nil
}
