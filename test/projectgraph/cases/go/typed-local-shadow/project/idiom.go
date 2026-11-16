package main

import (
	"log"
	"net/url"
	"os"
)

func parse(raw string) string {
	url, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return url.Host
}

type logger struct{}

func (logger) Println() { os.Mkdir("d", 0o755) }

func viaRange(ls []logger) {
	for _, log := range ls {
		log.Println()
	}
}

func viaParam(log logger) {
	log.Println()
}

func keepLog() { log.Println("x") }
