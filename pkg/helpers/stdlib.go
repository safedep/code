package helpers

import "golang.org/x/tools/go/packages"

var GoStdLibs map[string]*packages.Package

func init() {
	cfg := &packages.Config{Mode: packages.NeedName}
	pkgs, err := packages.Load(cfg, "std")
	if err != nil {
		panic(err)
	}

	GoStdLibs = make(map[string]*packages.Package)
	for _, pkg := range pkgs {
		GoStdLibs[pkg.Name] = pkg
	}
}
