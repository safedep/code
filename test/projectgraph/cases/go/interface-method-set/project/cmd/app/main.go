package main

import "example.com/jobs/worker"

func main() {
	start(worker.New())
}

func start(r worker.Runner) {
	r.Run("daily")
}
