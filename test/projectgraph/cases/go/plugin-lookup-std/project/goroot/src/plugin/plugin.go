package plugin

type Symbol any

type Plugin struct {
	syms map[string]any
}

func Open(path string) (*Plugin, error) {
	return &Plugin{syms: map[string]any{}}, nil
}

func (p *Plugin) Lookup(symName string) (Symbol, error) {
	return p.syms[symName], nil
}
