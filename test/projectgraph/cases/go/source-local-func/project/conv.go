package main

import "unsafe"

type T struct{ n int }

func convs(p unsafe.Pointer, k *uint32) uintptr {
	_ = (*T)(p)
	_ = (*T)(nil)
	return *(*uintptr)(unsafe.Pointer(k))
}

func runTable() {
	for _, t := range table {
		t.exec()
	}
}
