package indexer

import (
	"path"
	"strings"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/projectgraph/modules"
)

// pythonOutOfBuild reports whether no program imports a file of a Python
// unit, by its path. Such a file holds no code that an importer runs, and a
// test file of a package adds test roots and budget to a graph that no
// search of the package needs. modules.PythonTestPath holds the rule of a
// pypi unit. The unit of the standard library, as python:std@3.12.3 for
// the Lib directory of CPython, also leaves out its test suite under test,
// the IDLE, the turtle demos and lib2to3, which no program imports.
func pythonOutOfBuild(file core.File) bool {
	unit, inner := fileindex.SplitUnit(file.Name())
	pkg, err := fileindex.ParseUnit(unit)
	if !file.IsImport() || err != nil || path.Ext(inner) != ".py" {
		return false
	}
	switch pkg.Ecosystem {
	case "pypi":
		return modules.PythonTestPath(inner)
	case "python":
		first, _, _ := strings.Cut(inner, "/")
		return modules.PythonTestPath(inner) || pythonStdlibOutOfBuild[first]
	}
	return false
}

// pythonStdlibOutOfBuild names the directories of the Lib directory of
// CPython that no program imports.
var pythonStdlibOutOfBuild = map[string]bool{"test": true, "idlelib": true, "turtledemo": true, "lib2to3": true, "__phello__": true}
