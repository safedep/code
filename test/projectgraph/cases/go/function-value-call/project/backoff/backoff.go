package backoff

import "os"

func Linear(d int) func(int) int {
	return func(attempt int) int {
		os.Getenv("JITTER")
		return attempt * d
	}
}
