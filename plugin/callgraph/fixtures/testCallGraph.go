package main

import (
	"fmt"
	f "fmt"
	"net/http"
	"os"
	. "strings"
)

// Simple function
func helper(x int) int {
	fmt.Println("helper called")
	return x + 1
}

// Function with multiple calls
func processFile(filename string) {
	data := []byte("test data")
	os.WriteFile(filename, data, 0o644)
	fmt.Printf("Wrote file: %s\n", filename)
}

// Method with receiver
type MyType struct {
	Name string
}

func (m *MyType) PrintName() {
	fmt.Println(m.Name)
}

func (m *MyType) SetName(name string) {
	m.Name = name
}

// Function calls with aliases
func useAliases() {
	f.Println("Using alias import")

	// Dot import
	result := ToUpper("hello")
	fmt.Println(result)
}

// Nested function calls
func nestedCalls() {
	x := helper(5)
	y := helper(x)
	fmt.Println(y)
}

// HTTP client example
func makeRequest() {
	resp, err := http.Get("https://example.com")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()
}

// Chained method calls
func chainedCalls() {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", "https://example.com", nil)
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()
}

// Variadic function call
func useVariadic() {
	fmt.Println("one", "two", "three")
	fmt.Printf("Number: %d, String: %s\n", 42, "test")
}

// Function with struct initialization
func useStructs() {
	m := &MyType{Name: "test"}
	m.PrintName()
	m.SetName("new name")
}

func main() {
	// Simple calls
	helper(10)
	processFile("test.txt")

	// Method calls
	obj := &MyType{Name: "example"}
	obj.PrintName()
	obj.SetName("updated")

	// Import alias calls
	useAliases()

	// Nested calls
	nestedCalls()

	// HTTP calls
	makeRequest()
	chainedCalls()

	// Variadic
	useVariadic()

	// Structs
	useStructs()

	// Direct stdlib calls
	os.Getenv("HOME")
	fmt.Sprintf("formatted: %v", 123)
}
