package distgraph

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImportReaderReadsEachFormOfAnImport(t *testing.T) {
	reader, err := newImportReader()
	require.NoError(t, err)
	source := `from __future__ import annotations
import os, sys.path
import yaml as y
from a.b import c, d as e
from . import sibling
from ..up import thing
import importlib
m = importlib.import_module("yaml.loader")
n = importlib.import_module(name)
o = __import__('six')
p = __import__(f"pkg.{sub}")
try:
    import simplejson
except ImportError:
    simplejson = None
try:
    import ujson
except (ValueError, ModuleNotFoundError) as e:
    pass
try:
    import orjson
except:
    pass
try:
    import nope
except ValueError:
    pass
def f():
    import inner
`
	imports, parsed := reader.read(context.Background(), []byte(source))
	assert.True(t, parsed)
	assert.Equal(t, []importRef{
		{module: "os", line: 2},
		{module: "sys.path", line: 2},
		{module: "yaml", line: 3},
		{module: "a.b", line: 4},
		{module: "importlib", line: 7},
		{module: "yaml.loader", line: 8},
		{line: 9, dynamic: true},
		{module: "six", line: 10},
		{line: 11, dynamic: true},
		{module: "simplejson", line: 13, guarded: true},
		{module: "ujson", line: 17, guarded: true},
		{module: "orjson", line: 21, guarded: true},
		{module: "nope", line: 25},
		{module: "inner", line: 29},
	}, imports, "a relative import and a future import give nothing")
}

func TestImportReaderReportsAFileThatDoesNotParse(t *testing.T) {
	reader, err := newImportReader()
	require.NoError(t, err)
	imports, parsed := reader.read(context.Background(), []byte("import yaml\ndef (:\n"))
	assert.False(t, parsed)
	assert.Equal(t, []importRef{{module: "yaml", line: 1}}, imports)
}
