package extra

import "os"

var steps = []func(string) error{os.Chdir}

func Run() {
	steps[0]("/")
}
