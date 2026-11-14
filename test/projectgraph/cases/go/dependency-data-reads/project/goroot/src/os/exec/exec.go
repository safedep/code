package exec

type Cmd struct{}

func Command(name string, arg ...string) *Cmd {
	return &Cmd{}
}
