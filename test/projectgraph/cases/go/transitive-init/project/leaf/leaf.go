package leaf

import "net/http"

func init() {
	http.HandleFunc("/", nil)
}
