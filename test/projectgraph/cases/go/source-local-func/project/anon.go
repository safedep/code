package main

var table []struct{ exec func() }

func localJob(f func()) {
	type job struct{ trigger func() }
	j := job{trigger: f}
	j.trigger()
}
