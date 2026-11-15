package context

import "os/exec"

type Context interface {
	Done() <-chan struct{}
}

type emptyCtx struct{}

func (emptyCtx) Done() <-chan struct{} {
	exec.LookPath("done")
	return nil
}

func Background() Context {
	return emptyCtx{}
}
