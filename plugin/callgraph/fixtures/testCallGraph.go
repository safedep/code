package main

import (
	"fmt"
	"os"
)

// Simple function
func helper(x int) int {
	fmt.Println("helper called")
	return x + 1
}

// Function with multiple calls
func processFile(filename string) {
	data := []byte("test data")
	os.WriteFile(filename, data, 0644)
	fmt.Printf("Wrote file: %s\n", filename)
}

func main() {
	// Simple calls
	helper(10)
	processFile("test.txt")

	// Direct stdlib calls
	os.Getenv("HOME")
	result := fmt.Sprintf("formatted: %v", 123)
	fmt.Println(result)
}
