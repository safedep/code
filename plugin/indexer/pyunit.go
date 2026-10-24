package indexer

import (
	"path"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/projectgraph/modules"
)

// pythonOutOfBuild reports whether no program imports a file of a pypi
// unit, by its path. Such a file holds no code that an importer runs, and a
// test file of a package adds test roots and budget to a graph that no
// search of the package needs. modules.PythonTestPath holds the rule.
func pythonOutOfBuild(file core.File) bool {
	unit, inner := fileindex.SplitUnit(file.Name())
	pkg, err := fileindex.ParseUnit(unit)
	if !file.IsImport() || err != nil || pkg.Ecosystem != "pypi" || path.Ext(inner) != ".py" {
		return false
	}
	return modules.PythonTestPath(inner)
}
