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

// Additional test cases for more comprehensive testing

// An exported function starting with uppercase
func ExportedFunction() {
	// exported (public)
}

// A private function starting with lowercase
func unexportedFunction() {
	// unexported (package-private)
}

// A function with underscore (should be package-private)
func _underscoreFunction() {
	// unexported (package-private)
}

// Another struct for more method testing
type myPrivateStruct struct {
	data string
}

// An exported method on private struct
func (m *myPrivateStruct) ExportedMethod() string {
	return m.data
}

// An unexported method on private struct
func (m *myPrivateStruct) unexportedMethod() string {
	return m.data
}

// A public struct
type PublicStruct struct {
	Value int
}

// An exported method on public struct
func (p *PublicStruct) PublicMethod() int {
	return p.Value
}

// An unexported method on public struct
func (p *PublicStruct) privateMethod() int {
	return p.Value * 2
}
