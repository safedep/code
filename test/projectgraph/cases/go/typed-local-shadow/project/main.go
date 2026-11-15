package main

import (
	"os"
	"os/exec"
)

type local struct{}

func (local) run() { os.Getwd() }

type remote struct{}

func (remote) run() { exec.LookPath("ls") }

func (remote) stop() { os.Remove("x") }

func main() {
	x := local{}
	x.run()
	{
		x := remote{}
		x.run()
		f := x.stop
		f()
	}
	pick(opts{})
	named()
}

type opts struct{ r *remote }

func pick(o opts) {
	r := o.r
	if r == nil {
		r = &remote{}
	}
	r.run()
}

func named() (l *local) {
	l = &local{}
	l.run()
	return
}

func (x local) viaLit(r remote) {
	go func(x remote) {
		x.run()
	}(r)
}

type runner interface{ run() }

type optsIface struct{ r runner }

func pickIface(o optsIface) {
	r := o.r
	if r == nil {
		r = &local{}
	}
	r.run()
}

func siblings() {
	{
		x := local{}
		x.run()
	}
	{
		x := remote{}
		x.run()
	}
}

func shadowedParam(o opts, rs []runner) {
	{
		o := struct{ r runner }{rs[0]}
		x := o.r
		x = &remote{}
		x.run()
	}
}
