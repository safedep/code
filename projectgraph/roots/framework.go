package roots

import (
	"slices"
	"strings"

	"github.com/safedep/code/fileindex"
	rules "github.com/safedep/code/fileindex/roots"
	"github.com/safedep/code/internal/pathglob"
)

// matchPath marks the module top level and the exports of each file that a
// path convention names, as a route file of Next.js. The framework loads
// the file and calls what it exports.
func (f *finder) matchPath(r rules.Rule) {
	for _, ix := range f.applying(r) {
		if !pathglob.MatchAny(r.Match.Path, ix.Path) {
			continue
		}
		f.add(r, ix.Path)
		for _, key := range f.Exports(ix.Path) {
			f.add(r, key)
		}
	}
}

// matchDecorator marks each definition with a decorator whose text, as
// written, starts with one of the prefixes of the rule.
func (f *finder) matchDecorator(r rules.Rule) {
	for _, ix := range f.applying(r) {
		for _, def := range ix.Definitions {
			if slices.ContainsFunc(def.Decorators, func(d string) bool {
				return slices.ContainsFunc(r.Match.Decorator, func(prefix string) bool { return strings.HasPrefix(d, prefix) })
			}) {
				f.add(r, def.Key)
			}
		}
	}
}

// matchDecoratorCall marks each definition with a decorator whose key
// matches the call of the rule, as flask//Flask//route for @app.route
// after app = Flask(__name__).
func (f *finder) matchDecoratorCall(r rules.Rule) {
	for _, ix := range f.applying(r) {
		for _, def := range ix.Definitions {
			for _, d := range def.Decorators {
				if key, ok := f.ResolveName(ix.Path, strings.TrimPrefix(d, "@")); ok && f.callMatches(r.Match.Call, key) {
					f.add(r, def.Key)
				}
			}
		}
	}
}

// matchCall marks the definition that an argument of each matching call
// names, as handler in app.get('/x', handler). A function literal that the
// index does not define runs its body in the scope of the call, so that
// scope is the root.
func (f *finder) matchCall(r rules.Rule) {
	m := r.Match.Call
	for _, ix := range f.applying(r) {
		for _, call := range ix.Calls {
			if call.Kind == fileindex.CallImport || !slices.ContainsFunc(f.callKeys(ix, call), func(k string) bool { return f.callMatches(m, k) }) {
				continue
			}
			i := *m.Arg
			if i < 0 {
				i += len(call.Args)
			}
			if i < 0 || i >= len(call.Args) {
				continue
			}
			arg := call.Args[i]
			if len(m.Shape) > 0 && !slices.Contains(m.Shape, arg.Shape) {
				continue
			}
			site := Site{Path: ix.Path, Line: call.Line, Column: call.Column}
			if arg.Shape == fileindex.ArgFunction && len(arg.Targets) == 0 {
				f.addAt(r, call.Caller, site)
				continue
			}
			for _, key := range f.Resolve(ix, call.Caller, arg.Targets) {
				f.addAt(r, key, site)
			}
		}
	}
}

// callKeys returns the keys that a call resolves to.
func (f *finder) callKeys(ix *fileindex.Index, call fileindex.Call) []string {
	targets := call.Targets
	if len(targets) == 0 {
		targets = []string{call.Callee}
	}
	return f.Resolve(ix, call.Caller, targets)
}

// callMatches reports whether a callee key matches the callee and the
// receiver of the rule.
func (f *finder) callMatches(m *rules.CallMatch, key string) bool {
	switch {
	case slices.Contains(m.Callee, key):
	case slices.ContainsFunc(m.CalleeSuffix, func(suffix string) bool { return strings.HasSuffix(key, suffix) }):
	default:
		return false
	}
	return len(m.ReceiverFrom) == 0 || f.receiverFrom(fileindex.Parent(key), m.ReceiverFrom)
}

// receiverFrom reports whether a receiver comes from one of the keys: it is
// one of them or a key under one, as express//Router under express. A
// first-party definition comes from the keys of its decorators, so the
// receiver cli of @cli.command() comes from click//group for a function cli
// that @click.group() decorates.
func (f *finder) receiverFrom(receiver string, from []string) bool {
	under := func(key string) bool {
		return slices.ContainsFunc(from, func(k string) bool { return fileindex.HasKeyPrefix(key, k) })
	}
	if under(receiver) {
		return true
	}
	def, ok := f.defs[receiver]
	if !ok {
		return false
	}
	return slices.ContainsFunc(def.Decorators, func(d string) bool {
		key, ok := f.ResolveName(fileindex.Unit(receiver), strings.TrimPrefix(d, "@"))
		return ok && under(key)
	})
}

// matchExport marks each exported name of the rule in the files of its
// paths, as handler in a serverless handler.js.
func (f *finder) matchExport(r rules.Rule) {
	m := r.Match.Export
	for _, ix := range f.applying(r) {
		if len(m.Path) > 0 && !pathglob.MatchAny(m.Path, ix.Path) {
			continue
		}
		for _, name := range m.Names {
			if key, ok := f.Export(ix.Path, name); ok {
				f.add(r, key)
			}
		}
	}
}
