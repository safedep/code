package main

import (
	"example.com/markup"
	"example.com/markup/netctx"
)

func main() {
	read(&markup.Node{})
	call()
	promoted()
	wait(nil)
}

func read(n *markup.Node) {
	markup.Print(n.Data)
	markup.Print(markup.ErrNoRows)
	markup.Print(markup.KindText)
	markup.Render(n)
}

func call() {
	markup.Handler("x")
	markup.Run("y")
}

func promoted() {
	w := markup.NewWriter()
	w.Flush()
	c := markup.Dial()
	c.Close()
}

func wait(ctx netctx.Context) {
	ctx.Done()
}
