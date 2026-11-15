package ws

import "io"

type Conn struct{}

func (c *Conn) ReadMessage() ([]byte, error) {
	return io.ReadAll(c.NextReader())
}

func (c *Conn) NextReader() io.Reader {
	return &messageReader{c: c}
}

type messageReader struct{ c *Conn }

func (r *messageReader) Read(p []byte) (int, error) {
	return r.c.advance(p)
}

func (c *Conn) advance(p []byte) (int, error) {
	return len(p), nil
}
