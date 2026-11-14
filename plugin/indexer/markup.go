package indexer

import (
	"bytes"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/plugin/callgraph"
)

// addMarkup adds the records of the markup of a component file. Each
// script element that loads a file is an import of that file, as
// import './logic.js'. The rest of the markup is one record at the top
// level of the file, because the parse does not read it.
func (p *projection) addMarkup() {
	tree, ok := p.tree.(core.MarkupTree)
	if !ok || tree.Markup() == nil {
		return
	}
	markup := tree.Markup()
	for _, script := range markup.Scripts {
		line, _ := p.position(script.Offset)
		p.ix.Imports = append(p.ix.Imports, fileindex.Import{Line: line, Module: script.Src, Kind: fileindex.ImportStatic})
	}
	line, column := p.position(markup.Offset)
	text, _, _ := bytes.Cut(p.source[markup.Offset:], []byte("\n"))
	p.ix.Unresolved = append(p.ix.Unresolved, fileindex.Unresolved{
		Caller: p.cg.FileName,
		Line:   line,
		Column: column,
		Text:   callgraph.ClipText(string(bytes.TrimSpace(text))),
		Reason: fileindex.UnresolvedMarkup,
	})
}

// position returns the line and the column of an offset of the file, from
// 1 as the index counts them.
func (p *projection) position(offset int) (line, column int) {
	before := p.source[:offset]
	return bytes.Count(before, []byte("\n")) + 1, offset - bytes.LastIndexByte(before, '\n')
}
