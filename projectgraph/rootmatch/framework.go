package rootmatch

import (
	"slices"
	"strings"

	"github.com/safedep/code/fileindex"
	rules "github.com/safedep/code/fileindex/roots"
	"github.com/safedep/code/internal/pathglob"
)

// matchPath marks the module top level and the exports of each file that a
// path convention names, as a route file of Next.js. The framework loads
// the file and calls what it exports. A path rule of kind main names a
// script, and only its top level runs.
func (f *finder) matchPath(r rules.Rule) {
	for _, ix := range f.applying(r) {
		if !pathglob.MatchAny(r.Match.Path, ix.Path) {
			continue
		}
		f.add(r, ix.Path)
		if r.Kind == rules.Main {
			continue
		}
		for _, key := range f.Exports(ix.Path) {
			f.add(r, key)
		}
	}
}

// matchDecorator marks each definition with a decorator or an annotation
// whose text, as written, is one of the names of the rule, with or without
// arguments. A longer name, as @ScheduledTask for @Scheduled, is another
// decorator. The index holds the annotations of Java as decorators.
func (f *finder) matchDecorator(r rules.Rule) {
	names := append(slices.Clone(r.Match.Decorator), r.Match.Annotation...)
	for _, ix := range f.applying(r) {
		for _, def := range ix.Definitions {
			if slices.ContainsFunc(def.Decorators, func(d string) bool {
				return slices.ContainsFunc(names, func(name string) bool { return decoratorIs(d, name) })
			}) {
				f.add(r, def.Key)
			}
		}
	}
}

// decoratorIs reports whether a decorator as written is the name, or the
// name with arguments. A qualified Java annotation, as
// @org.springframework.scheduling.annotation.Scheduled, or a qualified
// Rust attribute, as @actix_web::get, is the name of its last part too.
func decoratorIs(decorator, name string) bool {
	if rest, ok := strings.CutPrefix(decorator, name); ok && (rest == "" || strings.HasPrefix(rest, "(")) {
		return true
	}
	simple, ok := strings.CutPrefix(name, "@")
	if !ok || strings.Contains(simple, ".") {
		return false
	}
	written, _, _ := strings.Cut(decorator, "(")
	return strings.HasPrefix(written, "@") && (strings.HasSuffix(written, "."+simple) || strings.HasSuffix(written, "::"+simple))
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
// names, as handler in app.get('/x', handler). The index defines a
// function literal of an argument, so the literal is the root. The scope of
// the call is never the root, because it can be dead code.
func (f *finder) matchCall(r rules.Rule) {
	m := r.Match.Call
	for _, ix := range f.applying(r) {
		for i := range ix.Calls {
			call := &ix.Calls[i]
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
			for _, key := range f.Resolve(ix, call.Caller, arg.Targets) {
				f.addAt(r, key, site)
			}
		}
	}
}

// matchReceiver marks the first-party receiver of each matching call, as
// the class MyView of MyView.as_view(). The framework makes an instance of
// the class and calls its methods, which the override edges of the link
// step reach.
func (f *finder) matchReceiver(r rules.Rule) {
	for _, ix := range f.applying(r) {
		for i := range ix.Calls {
			call := &ix.Calls[i]
			if call.Kind == fileindex.CallImport || !slices.ContainsFunc(f.callKeys(ix, call), func(k string) bool { return f.callMatches(r.Match.Call, k) }) {
				continue
			}
			receivers := make([]string, 0, len(call.Targets))
			for _, t := range call.Targets {
				receivers = append(receivers, fileindex.Parent(t))
			}
			site := Site{Path: ix.Path, Line: call.Line, Column: call.Column}
			for _, key := range f.Resolve(ix, call.Caller, receivers) {
				f.addAt(r, key, site)
			}
		}
	}
}

// callKeys returns the keys that a call resolves to. Three call rules read
// every JavaScript call, so the finder resolves each call once.
func (f *finder) callKeys(ix *fileindex.Index, call *fileindex.Call) []string {
	if keys, ok := f.calls[call]; ok {
		return keys
	}
	targets := call.Targets
	if len(targets) == 0 {
		targets = []string{call.Callee}
	}
	keys := f.Resolve(ix, call.Caller, targets)
	f.calls[call] = keys
	return keys
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
