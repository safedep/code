package main

import "example.com/shop/store"

func main() {
	s := store.New("orders")
	s.Put("a")
}
