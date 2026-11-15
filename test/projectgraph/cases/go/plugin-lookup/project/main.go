package main

import "plugin"

func main() {
	p, err := plugin.Open("hook.so")
	if err != nil {
		return
	}
	sym, err := p.Lookup("Run")
	if err != nil {
		return
	}
	sym.(func())()
}
