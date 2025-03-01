//go:build exclude

package fixtures

import "fmt"
import "flag"
import "github.com/safedep/code/lang"
import "github.com/aws/aws-sdk-go"
import osalias "os"
import codeccorealias "github.com/safedep/code/core"

import _ "embed"
import . "math"

import (
	"bufio"
	cryptoalias "crypto"
	_ "github.com/labstack/echo-contrib/pprof"
	gotreesitteralias "github.com/smacker/go-tree-sitter"
	. "net/http"
	"strings"
)

func main() {
	fmt.Println(lang.GetLanguage("go"))

	f, _ := osalias.Open("test.txt")
	defer f.Close()

	// Using items from wildcard imported math module
	sqrt := Sqrt(25)

	hash := cryptoalias.SHA256

	// Using items from wildcard imported net/http module
	resp, _ := Get("https://example.com")

	// 8. Using a package in a conditional block
	if strings.Contains("hello world", "hello") {
	}
}
