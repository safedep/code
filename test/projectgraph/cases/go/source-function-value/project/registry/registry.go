package registry

type F func(string) error

var handlers = map[string]F{}
var list []F

func Register(name string, f F) {
	handlers[name] = f
	list = append(list, f)
}

func ByName() { handlers["rm"]("x") }

func First() { list[0]("x") }
