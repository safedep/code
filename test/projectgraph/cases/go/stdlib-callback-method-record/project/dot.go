package main

import . "net/http"

func viaDotImport() {
	ListenAndServe(":80", handler{})
}
