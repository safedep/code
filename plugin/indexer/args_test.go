package indexer

import (
	"strings"
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fileindex"
	"github.com/stretchr/testify/assert"
)

func argsOf(ix *fileindex.Index, callee string) []fileindex.Arg {
	for _, call := range ix.Calls {
		if call.Callee == callee {
			return call.Args
		}
	}
	return nil
}

func TestFileIndexCallArguments(t *testing.T) {
	js := strings.Join([]string{
		"const express = require('express');",
		"const app = express();",
		"function handler(req, res) {}",
		"const h = { list: function () {} };",
		"app.get('/x', handler);",
		"app.post(`/y/${id}`, (req, res) => {}, h.list, 1, f(), ...rest);",
		"new Thing('a');",
	}, "\n") + "\n"
	ix := indexSource(t, core.LanguageCodeJavascript, "app.js", js)
	assert.Equal(t, []fileindex.Arg{{Shape: fileindex.ArgString}, {Shape: fileindex.ArgName, Targets: []string{"app.js//handler"}}}, argsOf(ix, "app.get"))
	assert.Equal(t, []fileindex.Arg{
		{Shape: fileindex.ArgTemplate},
		{Shape: fileindex.ArgFunction, Targets: []string{"app.js//<anonymous:6>"}},
		{Shape: fileindex.ArgMember, Targets: []string{"app.js//h//list"}},
		{Shape: fileindex.ArgLiteral},
		{Shape: fileindex.ArgComputed},
		{Shape: fileindex.ArgComputed},
	}, argsOf(ix, "app.post"))
	assert.Equal(t, []fileindex.Arg{{Shape: fileindex.ArgString}}, argsOf(ix, "Thing"))

	py := strings.Join([]string{
		"from django.urls import path",
		"from . import views",
		"urlpatterns = [path('', views.home, name='home'), path(f'{x}/', lambda r: r, None)]",
	}, "\n") + "\n"
	ix = indexSource(t, core.LanguageCodePython, "urls.py", py)
	var calls [][]fileindex.Arg
	for _, call := range ix.Calls {
		calls = append(calls, call.Args)
	}
	assert.Equal(t, [][]fileindex.Arg{
		{{Shape: fileindex.ArgString}, {Shape: fileindex.ArgMember, Targets: []string{"////views//home"}}},
		{{Shape: fileindex.ArgTemplate}, {Shape: fileindex.ArgFunction}, {Shape: fileindex.ArgLiteral}},
	}, calls, "a keyword argument has no position")
}
