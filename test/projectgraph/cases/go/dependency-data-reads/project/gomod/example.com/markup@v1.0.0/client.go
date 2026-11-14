package markup

import (
	"os"
	"os/exec"
)

type Conn interface {
	Close() error
}

type conn struct{}

func (c *conn) Close() error {
	os.Remove("close")
	return nil
}

type Client struct {
	Conn
	name string
}

func Dial() *Client {
	return &Client{Conn: &conn{}}
}

type Base struct{}

func (b *Base) Flush() {
	exec.Command("flush")
}

type Writer struct {
	*Base
	size int
}

func NewWriter() *Writer {
	return &Writer{Base: &Base{}}
}
