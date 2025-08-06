//go:build exclude

package fixtures

import "fmt"

// A simple function
func simpleFunction() {
	fmt.Println("hello")
}

// A function with parameters and return value
func functionWithArgs(a int, b string) string {
	return fmt.Sprintf("%s: %d", b, a)
}

// A struct for methods
type MyStruct struct {
	val int
}

// A method on MyStruct
func (s *MyStruct) MyMethod(p int) int {
	return s.val + p
}

// A private function (package-level)
func privateFunction() {
	// not exported
}
