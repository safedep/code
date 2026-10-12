package ecmascript_test

import (
	"context"
	"fmt"

	"github.com/safedep/code/lang"
	"github.com/safedep/code/lang/ecmascript"
	"github.com/safedep/code/parser"
)

func Example() {
	l, err := lang.NewJavascriptLanguage()
	if err != nil {
		panic(err)
	}
	source := "global['r'] = require\nconst { exec: run } = global.r('child_process')\nrun(atob(c))\n"
	tree, err := parser.ParseBytes(context.Background(), l, []byte(source), parser.WithFileName("logo.svg"))
	if err != nil {
		panic(err)
	}
	f, err := ecmascript.Analyze(context.Background(), tree, ecmascript.Options{})
	if err != nil {
		panic(err)
	}
	for _, c := range f.Calls() {
		for _, v := range f.Eval(c.Callee) {
			if v.IsModuleMember("exec", "child_process") && ecmascript.ClassifyArg(c.Args[0]) == ecmascript.ArgComputed {
				fmt.Printf("line %d runs a computed command\n", c.Node.StartPoint().Row+1)
			}
		}
	}
	// Output: line 3 runs a computed command
}
