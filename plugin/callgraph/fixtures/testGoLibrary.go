// Package without main function - represents a library package
package testlib

import (
	"fmt"
	"strings"
)

// Exported function - public API
func ProcessData(data string) string {
	result := strings.ToUpper(data)
	fmt.Printf("Processed: %s\n", result)
	return result
}

// Unexported helper function
func logError(msg string) {
	fmt.Println("Error:", msg)
}

// Exported function that calls another function
func ValidateInput(input string) bool {
	if input == "" {
		logError("Empty input")
		return false
	}
	return true
}

// Exported function with multiple calls
func Transform(s string) string {
	if !ValidateInput(s) {
		return ""
	}
	return ProcessData(s)
}
