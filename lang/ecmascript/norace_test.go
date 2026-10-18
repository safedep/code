//go:build !race

package ecmascript_test

import "time"

// linearLimit is the time limit of an analysis that must stay linear.
const linearLimit = 20 * time.Second
