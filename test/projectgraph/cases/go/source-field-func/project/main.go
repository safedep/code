package main

import "os"

func main() {
	s := &Svc{cfg: Config{Remove: os.Remove}}
	s.run()
}
