package projectgraph

import (
	"cmp"
	"strings"
	"unicode"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
)

// scope holds what one file binds at its top level.
type scope struct {
	ix         *fileindex.Index
	bindings   map[string]fileindex.Import
	imports    []fileindex.Import
	exports    map[string][]fileindex.Export
	stars      []star
	hasExports bool
}

// star is an export * from of JavaScript or a from m import * of Python.
type star struct {
	spec string
	line int
}

func newScope(ix *fileindex.Index) *scope {
	sc := &scope{
		ix:       ix,
		bindings: map[string]fileindex.Import{},
		exports:  map[string][]fileindex.Export{},
	}
	for _, imp := range ix.Imports {
		if imp.Kind != fileindex.ImportStatic && imp.Kind != fileindex.ImportDynamicString {
			continue
		}
		if imp.Wildcard {
			if ix.Language == langcode.Python {
				sc.stars = append(sc.stars, star{spec: imp.Module, line: imp.Line})
			}
			continue
		}
		sc.imports = append(sc.imports, imp)
		if name := bindingName(ix.Language, imp); name != "" {
			sc.bindings[name] = imp
		}
	}
	for _, ex := range ix.Exports {
		sc.hasExports = true
		if ex.Name == "*" && ex.From != "" {
			sc.stars = append(sc.stars, star{spec: ex.From, line: ex.Line})
			continue
		}
		sc.exports[ex.Name] = append(sc.exports[ex.Name], ex)
	}
	return sc
}

// enclosing returns the key of the innermost definition that holds the line,
// or the module key.
func (sc *scope) enclosing(line int) string {
	key, span := sc.ix.Path, -1
	for _, def := range sc.ix.Definitions {
		if def.StartLine > line || def.EndLine < line {
			continue
		}
		if s := def.EndLine - def.StartLine; span < 0 || s <= span {
			key, span = def.Key, s
		}
	}
	return key
}

// importFor returns the import whose namespace is the longest prefix of the
// target, and the segments of the target after that namespace.
func (sc *scope) importFor(target string) (fileindex.Import, []string, bool) {
	var best fileindex.Import
	found := false
	for _, imp := range sc.imports {
		ns := imp.Namespace
		if ns == "" || !HasKeyPrefix(target, ns) || found && len(ns) <= len(best.Namespace) {
			continue
		}
		best, found = imp, true
	}
	if !found {
		return best, nil, false
	}
	var rest []string
	if len(target) > len(best.Namespace) {
		rest = strings.Split(target[len(best.Namespace)+len(Separator):], Separator)
	}
	return best, rest, true
}

func (sc *scope) exported(name string) bool {
	return len(sc.exports[name]) > 0
}

// bindingName returns the name that an import binds in the file. A Python
// import a.b with no alias binds a.
func bindingName(lang langcode.Code, imp fileindex.Import) string {
	switch {
	case imp.Item != "":
		return lastDotted(cmp.Or(imp.Alias, imp.Item))
	case imp.Alias != "" && imp.Alias != imp.Module:
		return imp.Alias
	case lang == langcode.Python:
		return firstDotted(imp.Module)
	}
	return ""
}

// boundModule returns the module that the name of an import holds.
func boundModule(lang langcode.Code, imp fileindex.Import) string {
	if bindsRoot(lang, imp) {
		return firstDotted(imp.Module)
	}
	return imp.Module
}

// bindsRoot reports whether a Python import a.b binds the root package a. A
// bound name of a or no bound name both mean that.
func bindsRoot(lang langcode.Code, imp fileindex.Import) bool {
	return lang == langcode.Python && imp.Item == "" &&
		(imp.Alias == "" || imp.Alias == imp.Module || imp.Alias == firstDotted(imp.Module))
}

type failure struct {
	reason fileindex.UnresolvedReason
	name   string
	// cut marks a failure that a cycle guard or the hop limit caused. The
	// same name can resolve on another way in, so the failure is not kept.
	cut bool
}

func noDefinition(name string) *failure {
	return &failure{reason: fileindex.UnresolvedNoDefinition, name: name}
}

type resolution struct {
	key  string
	kind EdgeKind
	rule string
	fail *failure
}

func (b *builder) linkCalls(ix *fileindex.Index) {
	if isDeclarationFile(ix.Path) {
		return
	}
	sc := b.scopes[ix.Path]
	for _, call := range ix.Calls {
		if call.Kind == fileindex.CallImport {
			continue
		}
		targets := call.Targets
		if len(targets) == 0 {
			if !isIdentifier(call.Callee) {
				continue
			}
			targets = []string{call.Callee}
		}
		from := call.Caller
		if b.g.Nodes[from] == nil {
			from = sc.enclosing(call.Line)
		}
		from = b.runner(from)
		site := Site{Path: ix.Path, Line: call.Line, Column: call.Column}
		for _, t := range targets {
			r := b.resolveTarget(sc, call, t)
			if r.fail != nil {
				b.addUnresolved(UnresolvedSite{
					Caller:   from,
					Site:     site,
					Text:     call.Callee,
					Receiver: call.Receiver,
					Method:   cmp.Or(call.Method, r.fail.name),
					Reason:   r.fail.reason,
				})
				continue
			}
			switch {
			case r.kind == EdgeDirect && isAssignment(sc, call, t):
				r.kind, r.rule = EdgeAssignment, "assignment"
			case r.kind == EdgeDirect && call.Receiver != "" && b.isMethod(r.key):
				r.kind, r.rule = EdgeMember, "member"
			}
			b.addEdge(Edge{From: from, To: r.key, Site: site, Kind: r.kind, Resolver: resolverName(ix.Language, r.rule)})
		}
	}
}

func (b *builder) resolveTarget(sc *scope, call fileindex.Call, target string) resolution {
	ix := sc.ix
	if HasKeyPrefix(target, ix.Path) {
		return b.resolveLocal(target)
	}
	if imp, rest, ok := sc.importFor(target); ok {
		mod, _, found := b.resolveModule(ix, imp.Module)
		if !found {
			return resolution{key: b.external(target), kind: EdgeDirect, rule: "external"}
		}
		var names []string
		switch {
		case imp.Item != "":
			names = append([]string{imp.Item}, rest...)
		case len(rest) == 0 && isECMAScript(ix.Language):
			names = []string{"default"}
		default:
			names = rest
		}
		r := b.walk(mod, names)
		r.rule = cmp.Or(r.rule, "import")
		return r
	}
	if !strings.Contains(target, Separator) {
		if r, ok := b.wildcard(sc, target); ok {
			return r
		}
	}
	return resolution{key: b.external(target), kind: EdgeDirect, rule: "external"}
}

// resolveLocal resolves a key of the file. A key that names no node walks
// from its longest prefix that does, as a method of a base class.
func (b *builder) resolveLocal(target string) resolution {
	if b.g.Nodes[target] != nil {
		return resolution{key: target, kind: EdgeDirect, rule: "local"}
	}
	unit, segs := Unit(target), Segments(target)
	for i := len(segs) - 1; i > 0; i-- {
		if prefix := Join(unit, segs[:i]...); b.g.Nodes[prefix] != nil {
			r := b.walk(prefix, segs[i:])
			r.rule = cmp.Or(r.rule, "local")
			return r
		}
	}
	r := b.walk(unit, segs)
	r.rule = cmp.Or(r.rule, "local")
	return r
}

// walk resolves the names one at a time from a key: a name of a module, a
// method of a class, or a member of an external key.
func (b *builder) walk(key string, names []string) resolution {
	r := resolution{key: key, kind: EdgeDirect}
	for i := 0; i < len(names); {
		name := names[i]
		switch kind := b.kindOf(r.key); {
		case b.unindexed(r.key):
			return r
		case isNamespacePackage(r.key) || kind == NodeModule:
			key, fail := b.resolveName(r.key, name, 0)
			if fail != nil {
				r.fail = fail
				return r
			}
			r.key = key
			i++
		case kind == NodeExport:
			r.key = b.bindings[r.key]
		case kind == NodeClass:
			key, fail := b.method(r.key, name, 0, map[string]bool{})
			if fail != nil {
				r.fail = fail
				return r
			}
			r.key, r.kind, r.rule = key, EdgeMember, "member"
			i++
		case kind == NodeExternal:
			r.key = b.external(Join(r.key, names[i:]...))
			r.rule = cmp.Or(r.rule, "external")
			return r
		case kind == NodeFunction || kind == NodeMethod || kind == NodeConstructor:
			// The name is a local variable or a parameter of the function,
			// and the index does not know its value.
			r.fail = &failure{reason: fileindex.UnresolvedUnknownReceiver, name: name}
			return r
		default:
			r.fail = noDefinition(name)
			return r
		}
	}
	if isNamespacePackage(r.key) {
		r.fail = noDefinition(Last(strings.TrimSuffix(r.key, "/")))
	}
	return r
}

// resolveName returns the key that a name of a module stands for. A name
// that the module re-exports gets a node of kind export in the module, with
// an export edge to what it names.
func (b *builder) resolveName(module, name string, hops int) (string, *failure) {
	k := nameKey{module: module, name: name}
	if r, ok := b.names[k]; ok {
		return r.key, r.fail
	}
	if b.visiting[k] {
		return "", &failure{reason: fileindex.UnresolvedNoDefinition, name: name, cut: true}
	}
	if hops > b.opts.maxExportHops {
		path := strings.TrimSuffix(module, "/")
		b.truncations[Truncation{Kind: TruncationExportHops, Key: Join(path, name), Site: Site{Path: path}}] = true
		return "", &failure{reason: fileindex.UnresolvedLimit, name: name, cut: true}
	}
	b.visiting[k] = true
	key, fail := b.lookupName(module, name, hops)
	delete(b.visiting, k)
	if fail == nil || !fail.cut {
		b.names[k] = resolution{key: key, fail: fail}
	}
	return key, fail
}

func (b *builder) lookupName(module, name string, hops int) (string, *failure) {
	if isNamespacePackage(module) {
		if sub, ok := b.python.Submodule(module, name); ok {
			return sub, nil
		}
		return "", noDefinition(name)
	}
	sc := b.scopes[module]
	if sc == nil {
		return module, nil
	}
	if n := b.g.Nodes[Join(module, name)]; n != nil && n.Kind != NodeExport {
		return n.Key, nil
	}

	miss := noDefinition(name)
	try := func(_ string, fail *failure) bool {
		if fail == nil {
			return true
		}
		miss.cut = miss.cut || fail.cut
		if fail.reason == fileindex.UnresolvedLimit {
			miss.reason = fail.reason
		}
		return false
	}
	for _, ex := range sc.exports[name] {
		if key, fail := b.followExport(sc, ex, hops); try(key, fail) {
			return key, nil
		}
	}
	python := sc.ix.Language == langcode.Python
	if imp, ok := sc.bindings[name]; ok && python && !sc.hasExports {
		if key, fail := b.followBinding(sc, imp, name, hops); try(key, fail) {
			return key, nil
		}
	}
	if !python || !sc.hasExports || sc.exported(name) {
		for _, st := range sc.stars {
			if key, fail := b.followStar(sc, st, name, hops); try(key, fail) {
				return key, nil
			}
		}
	}
	if python {
		if sub, ok := b.python.Submodule(module, name); ok {
			return sub, nil
		}
	}
	return "", miss
}

// followExport resolves an export record of the file. A record with no
// target and no module exports a name that the file imports, and Item holds
// that local name when it differs from Name.
func (b *builder) followExport(sc *scope, ex fileindex.Export, hops int) (string, *failure) {
	switch {
	case ex.Target != "":
		if b.g.Nodes[ex.Target] != nil {
			return ex.Target, nil
		}
		return "", noDefinition(ex.Name)
	case ex.From != "":
		mod, _, ok := b.resolveModule(sc.ix, ex.From)
		var key string
		switch {
		case !ok:
			key = b.external(joinNonEmpty(namespaceOf(sc.ix.Language, ex.From), ex.Item))
		case ex.Item == "":
			key = mod
		default:
			var fail *failure
			if key, fail = b.resolveName(mod, ex.Item, hops+1); fail != nil {
				return "", fail
			}
		}
		return b.bind(sc, ex.Name, ex.Line, key, "reexport"), nil
	}
	imp, ok := sc.bindings[cmp.Or(ex.Item, ex.Name)]
	if !ok {
		return "", noDefinition(ex.Name)
	}
	return b.followBinding(sc, imp, ex.Name, hops)
}

// followBinding resolves a name that the file imports and gives on.
func (b *builder) followBinding(sc *scope, imp fileindex.Import, name string, hops int) (string, *failure) {
	lang := sc.ix.Language
	mod, _, ok := b.resolveModule(sc.ix, boundModule(lang, imp))
	var key string
	switch {
	case !ok:
		key = b.external(boundNamespace(lang, imp))
	case imp.Item != "":
		var fail *failure
		if key, fail = b.resolveName(mod, imp.Item, hops+1); fail != nil {
			return "", fail
		}
	case isECMAScript(lang):
		key = mod
		if def, fail := b.resolveName(mod, "default", hops+1); fail == nil {
			key = def
		}
	default:
		key = mod
	}
	return b.bind(sc, name, imp.Line, key, "reexport"), nil
}

// followStar resolves a name through an export * from or a Python
// from m import *. Python gives on only the names of __all__, or every name
// with no leading underscore.
func (b *builder) followStar(sc *scope, st star, name string, hops int) (string, *failure) {
	mod, _, ok := b.resolveModule(sc.ix, st.spec)
	if !ok || !b.starVisible(mod, name) {
		return "", noDefinition(name)
	}
	key, fail := b.resolveName(mod, name, hops+1)
	if fail != nil {
		return "", fail
	}
	return b.bind(sc, name, st.line, key, "reexport-star"), nil
}

func (b *builder) starVisible(module, name string) bool {
	src := b.scopes[module]
	if src == nil || src.ix.Language != langcode.Python {
		return true
	}
	if src.hasExports {
		return src.exported(name)
	}
	return !strings.HasPrefix(name, "_")
}

// wildcard resolves a bare name through the from m import * of the file.
func (b *builder) wildcard(sc *scope, name string) (resolution, bool) {
	for _, st := range sc.stars {
		if sc.ix.Language != langcode.Python {
			break
		}
		mod, _, ok := b.resolveModule(sc.ix, st.spec)
		if !ok || !b.starVisible(mod, name) {
			continue
		}
		if key, fail := b.resolveName(mod, name, 0); fail == nil {
			return resolution{key: key, kind: EdgeWildcard, rule: "wildcard"}, true
		}
	}
	return resolution{}, false
}

func (b *builder) bind(sc *scope, name string, line int, target, rule string) string {
	if isNamespacePackage(target) {
		return target
	}
	key := Join(sc.ix.Path, name)
	b.addNode(&Node{Key: key, Kind: NodeExport, Path: sc.ix.Path, Line: line, Exported: true, Test: b.test[sc.ix.Path]})
	b.bindings[key] = target
	b.addEdge(Edge{
		From:     key,
		To:       target,
		Site:     Site{Path: sc.ix.Path, Line: line},
		Kind:     EdgeExport,
		Resolver: resolverName(sc.ix.Language, rule),
	})
	return key
}

// method returns the method of a class or of its bases. A method that no
// first-party class defines falls to the first external base.
func (b *builder) method(class, name string, depth int, seen map[string]bool) (string, *failure) {
	if n := b.g.Nodes[Join(class, name)]; n != nil {
		return n.Key, nil
	}
	if seen[class] || depth > b.opts.maxExportHops {
		return "", noDefinition(name)
	}
	seen[class] = true
	rec, ok := b.classes[class]
	if !ok {
		return "", noDefinition(name)
	}
	external := ""
	for _, base := range b.bases(rec) {
		switch {
		case b.kindOf(base) == NodeClass:
			if key, fail := b.method(base, name, depth+1, seen); fail == nil {
				return key, nil
			}
		case external == "" && b.g.IsExternal(base):
			external = Join(base, name)
		}
	}
	if external != "" {
		return b.external(external), nil
	}
	return "", noDefinition(name)
}

// bases returns the keys of the bases of a class that the file scope
// resolves. A builtin base, as object, resolves to nothing.
func (b *builder) bases(rec classRecord) []string {
	sc := b.scopes[rec.ix.Path]
	var out []string
	for _, base := range rec.class.Bases {
		if key, ok := b.resolveScopeName(sc, base); ok {
			out = append(out, key)
		}
	}
	return out
}

// resolveScopeName resolves a dotted name as written in the file, as a base
// class mod.Base.
func (b *builder) resolveScopeName(sc *scope, text string) (string, bool) {
	segs := strings.Split(text, ".")
	var r resolution
	if local := Join(sc.ix.Path, segs[0]); b.g.Nodes[local] != nil {
		r = b.walk(local, segs[1:])
	} else if imp, ok := sc.bindings[segs[0]]; ok {
		lang := sc.ix.Language
		mod, _, found := b.resolveModule(sc.ix, boundModule(lang, imp))
		if !found {
			return Join(boundNamespace(lang, imp), segs[1:]...), true
		}
		names := segs[1:]
		if imp.Item != "" {
			names = append([]string{imp.Item}, names...)
		}
		r = b.walk(mod, names)
	} else {
		return "", false
	}
	if r.fail != nil {
		return "", false
	}
	return b.deref(r.key), true
}

func (b *builder) deref(key string) string {
	for b.kindOf(key) == NodeExport {
		key = b.bindings[key]
	}
	return key
}

func (b *builder) resolveModule(ix *fileindex.Index, spec string) (string, string, bool) {
	if spec == "" {
		return "", "", false
	}
	switch ix.Language {
	case langcode.Python:
		mod, ok := b.python.Resolve(ix.Path, spec)
		via := "absolute"
		if strings.HasPrefix(spec, ".") {
			via = "relative"
		}
		return mod, via, ok
	case langcode.Javascript, langcode.Typescript:
		return b.js.Resolve(ix.Path, spec, ix.Language == langcode.Typescript)
	}
	return "", "", false
}

func (b *builder) kindOf(key string) NodeKind {
	if n := b.g.Nodes[key]; n != nil {
		return n.Kind
	}
	return ""
}

func (b *builder) isModule(key string) bool {
	return b.kindOf(key) == NodeModule
}

func (b *builder) isMethod(key string) bool {
	k := b.kindOf(key)
	return k == NodeMethod || k == NodeConstructor
}

// unindexed reports whether the key is a file that the index step reported
// with no index.
func (b *builder) unindexed(key string) bool {
	status, ok := b.g.Files[key]
	return ok && b.byPath[key] == nil && !status.Complete()
}

func (b *builder) bindsName(module, name string) bool {
	sc := b.scopes[module]
	if sc == nil {
		return false
	}
	_, bound := sc.bindings[name]
	return bound || b.g.Nodes[Join(module, name)] != nil
}

// isAssignment reports whether a call reached its target through an alias
// inside the file: the target has another name than the call writes, and
// the written name is not an import binding.
func isAssignment(sc *scope, call fileindex.Call, target string) bool {
	written := cmp.Or(call.Method, lastDotted(call.Callee))
	if written == "" || Last(target) == written {
		return false
	}
	_, bound := sc.bindings[firstDotted(call.Callee)]
	return !bound
}

// boundNamespace returns the external key of the name that an import binds.
func boundNamespace(lang langcode.Code, imp fileindex.Import) string {
	if bindsRoot(lang, imp) {
		return namespaceOf(lang, firstDotted(imp.Module))
	}
	return imp.Namespace
}

// isDeclarationFile reports whether a TypeScript file only declares types.
func isDeclarationFile(p string) bool {
	return strings.HasSuffix(p, ".d.ts")
}

func isNamespacePackage(key string) bool {
	return strings.HasSuffix(key, "/")
}

func isECMAScript(lang langcode.Code) bool {
	return lang == langcode.Javascript || lang == langcode.Typescript
}

func isIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r != '_' && r != '$' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func firstDotted(s string) string {
	first, _, _ := strings.Cut(s, ".")
	return first
}

func lastDotted(s string) string {
	return s[strings.LastIndex(s, ".")+1:]
}

func joinNonEmpty(unit, segment string) string {
	if segment == "" {
		return unit
	}
	return Join(unit, segment)
}
