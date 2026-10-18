package lib

import "net/http"

func init() {
	http.HandleFunc("/", nil)
}

func main() {
	http.ListenAndServe(":80", nil)
}
