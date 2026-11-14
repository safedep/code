package main

type T func()

func main() {
	{
		var helper T
		_ = helper
	}
	fs := []func(){helper}
	fs[0]()
	{
		var cleanup T
		gs := []func(){cleanup}
		gs[0]()
	}
}
