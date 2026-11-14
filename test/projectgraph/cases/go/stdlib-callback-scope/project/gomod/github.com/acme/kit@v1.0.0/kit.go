package kit

func Run() { start() }

func start() {}

func Dial() { handshake() }

func handshake() {}

type client struct{}

func (c *client) Close() { closeConn() }

func closeConn() {}

func (c *client) reset() { resetConn() }

func resetConn() {}
