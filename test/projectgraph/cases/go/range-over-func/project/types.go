package main

type node struct {
	Attr []string
}

type Seq func(func(string) bool)

type holder struct {
	Each func(func(string) bool)
}
