//go:build race

package ecmascript_test

import "time"

// linearLimit is the time limit of an analysis that must stay linear. The
// race detector makes the analysis several times slower, and the CI
// runner shares its cores with other test packages. The quadratic walk
// that the limit guards against took 72 s without the detector, so it
// still fails this limit.
const linearLimit = 60 * time.Second
