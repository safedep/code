package main

import "strings"

type node struct {
	Attr []string
}

type Seq func(func(string) bool)

type holder struct {
	Each func(func(string) bool)
}

type List []string

func names() []string { return nil }

func (*node) Children() []*node { return nil }

func stdRange(s string) {
	for range strings.Fields(s) {
	}
}
