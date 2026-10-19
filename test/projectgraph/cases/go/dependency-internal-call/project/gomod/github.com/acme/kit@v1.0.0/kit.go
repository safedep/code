package kit

func Serve() {
	var c conn
	c.readFrame()
}

type conn struct{}

func (c *conn) readFrame() { parseHeader() }

func parseHeader() {}

func Dead() {}

func dead() { Dead() }
