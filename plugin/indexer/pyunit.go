package indexer

import (
	"path"
	"slices"
	"strings"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fileindex"
)

// pythonOutOfBuild reports whether no program imports a file of a pypi
// unit, by its path: a file under a tests directory, a test file by its
// name, a conftest.py, a file under a __pycache__, and a file under the
// .dist-info, .egg-info or .data directory of the wheel. Such a file holds
// no code that an importer runs, and a test file of a package adds test
// roots and budget to a graph that no search of the package needs. A test
// or a testing directory stays, because django.test and numpy.testing are
// modules that programs import.
func pythonOutOfBuild(file core.File) bool {
	unit, inner := fileindex.SplitUnit(file.Name())
	pkg, err := fileindex.ParseUnit(unit)
	if !file.IsImport() || err != nil || pkg.Ecosystem != "pypi" || path.Ext(inner) != ".py" {
		return false
	}
	base := path.Base(inner)
	if base == "conftest.py" || strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test.py") {
		return true
	}
	return slices.ContainsFunc(strings.Split(path.Dir(inner), "/"), func(d string) bool {
		return d == "tests" || d == "__pycache__" || strings.HasSuffix(d, ".dist-info") || strings.HasSuffix(d, ".egg-info") || strings.HasSuffix(d, ".data")
	})
}
