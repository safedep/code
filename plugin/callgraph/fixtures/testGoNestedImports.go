// Test file with nested package imports (net/http, encoding/json style)
package main

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"io/ioutil"
)

// Function using net/http
func makeHTTPRequest() {
	http.Get("https://api.example.com/data")
}

// Function using encoding/json
func parseJSON(data []byte) map[string]interface{} {
	var result map[string]interface{}
	json.Unmarshal(data, &result)
	return result
}

// Function using path/filepath
func processPath(dir string, file string) string {
	fullPath := filepath.Join(dir, file)
	return fullPath
}

// Function using io/ioutil
func readConfig(filename string) []byte {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil
	}
	return data
}

// Function that combines multiple nested imports
func fetchAndParse() {
	http.Get("https://api.example.com/config")

	data := []byte("test")
	ioutil.ReadAll(nil)
	config := parseJSON(data)

	filepath.Join("/etc", "app.conf")
	json.Marshal(config)
	http.NewRequest("POST", "/api", nil)
}

func main() {
	makeHTTPRequest()
	data := readConfig("config.json")
	result := parseJSON(data)
	processPath("/tmp", "test.txt")
	fetchAndParse()

	// Direct calls with nested imports
	http.Head("https://example.com")
	json.Valid(result)
	filepath.Abs("/tmp")
}
