package kit

import "github.com/acme/kit/internal/wire"

func Serve() {
	wire.Encode()
	var b wire.Buffer
	b.Flush()
}
