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
	ix       *fileindex.Index
	bindings map[string]fileindex.Import
	// types holds the names that a TypeScript import type binds. Only a
	// declared type or a base class reads them.
	types   map[string]fileindex.Import
	imports []fileindex.Import
	exports map[string][]fileindex.Export
	stars   []star
	// dots are the Go dot imports, which put the names of a package in
	// the scope of the file.
	dots       []fileindex.Import
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
		types:    map[string]fileindex.Import{},
		exports:  map[string][]fileindex.Export{},
	}
	for _, imp := range ix.Imports {
		if imp.Kind == fileindex.ImportTypeOnly {
			if name := bindingName(ix.Language, imp); name != "" {
				sc.types[name] = imp
			}
			continue
		}
		if imp.Kind != fileindex.ImportStatic && imp.Kind != fileindex.ImportDynamicString {
			continue
		}
		if imp.Wildcard && ix.Language == langcode.Python {
			sc.stars = append(sc.stars, star{spec: imp.Module, line: imp.Line})
			continue
		}
		if imp.Wildcard && ix.Language == langcode.Go {
			sc.dots = append(sc.dots, imp)
		}
		// A JavaScript import for its side effect binds no name. Its
		// namespace still names the module of a call on require('x').
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
		if ns == "" || !fileindex.HasKeyPrefix(target, ns) || found && len(ns) <= len(best.Namespace) {
			continue
		}
		best, found = imp, true
	}
	if !found {
		return best, nil, false
	}
	var rest []string
	if len(target) > len(best.Namespace) {
		rest = strings.Split(target[len(best.Namespace)+len(fileindex.Separator):], fileindex.Separator)
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

// importedModule returns the module that the name of an import holds, and
// the names that lead from that module to the bound value. A Python import
// a.b binds the package a. An import of an item binds the item.
func importedModule(lang langcode.Code, imp fileindex.Import) (string, []string) {
	switch {
	case bindsRoot(lang, imp):
		return firstDotted(imp.Module), nil
	case imp.Item != "":
		return imp.Module, []string{imp.Item}
	}
	return imp.Module, nil
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
	// unbound marks a target that no definition and no import of the file
	// binds, as print or console//log.
	unbound bool
	// target is the per-file target that the resolution came from.
	target string
	// class is the class or interface whose method the walk looked up
	// last. virtual marks a receiver that can hold an instance of any
	// subclass of it: the instance keyword or a declared type. declared
	// marks a receiver that a declared type narrowed.
	class    string
	virtual  bool
	declared bool
}

func (b *builder) linkCalls(ix *fileindex.Index) {
	if isDeclarationFile(ix.Path) {
		return
	}
	sc := b.scopes[ix.Path]
	unknown := map[[2]int]bool{}
	for _, u := range ix.Unresolved {
		if u.Reason == fileindex.UnresolvedUnknownReceiver {
			unknown[[2]int{u.Line, u.Column}] = true
		}
	}
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
		caller := call.Caller
		if b.g.Nodes[caller] == nil {
			caller = sc.enclosing(call.Line)
		}
		from := b.runner(caller)
		site := Site{Path: ix.Path, Line: call.Line, Column: call.Column}
		for _, r := range b.resolveTargets(sc, caller, targets) {
			b.linkCHA(call, r, from, site)
			// A declared type gives the class of the receiver, so the
			// link step owns the record of the site and no name edge
			// goes from it.
			if r.declared && (r.fail == nil || r.fail.name == call.Method) {
				b.narrowed[site] = true
			}
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
			if r.unbound {
				// A call on an unknown receiver, as plugin.run(), has the
				// fallback target plugin//run, which names no module. The
				// record of the index holds the call, so it makes no edge.
				if unknown[[2]int{call.Line, call.Column}] {
					continue
				}
				// A name that no builtin binds is a parameter or a local
				// variable, as cb in def run(cb): cb(x). Its value is not
				// known, so the call is a record and not an external.
				if !langcode.IsBuiltin(ix.Language, fileindex.Unit(r.key)) {
					b.addUnresolved(UnresolvedSite{
						Caller:   from,
						Site:     site,
						Text:     call.Callee,
						Receiver: call.Receiver,
						Method:   cmp.Or(call.Method, fileindex.Last(r.key)),
						Reason:   fileindex.UnresolvedUnknownReceiver,
					})
					continue
				}
				b.external(r.key)
			}
			switch {
			case r.kind == EdgeDirect && isAssignment(sc, call, r.target):
				r.kind, r.rule = EdgeAssignment, "assignment"
			case r.kind == EdgeDirect && call.Receiver != "" && b.isMethod(r.key):
				r.kind, r.rule = EdgeMember, "member"
			}
			if r.kind == EdgeMember && r.declared {
				r.rule = "declared-type"
			}
			b.addEdge(Edge{From: from, To: r.key, Site: site, Kind: r.kind, Resolver: resolverName(ix.Language, r.rule)})
		}
	}
}

// linkCHA narrows the site of a method call whose receiver can hold an
// instance of a subclass of a known class: the instance keyword or a
// declared type. It adds a cha edge to the method of each subclass that the
// call can run besides the resolution, also when the class itself has no
// such method, as an abstract method. A narrowed site gets no field or name
// edge, and its record comes from the link step.
//
// A receiver that a call of the class makes, as p = Parser(), holds an
// instance of that class only, so it gets no cha edge.
func (b *builder) linkCHA(call fileindex.Call, r resolution, from string, site Site) {
	lang := b.byPath[site.Path].Language
	class, virtual := r.class, r.virtual
	if r.fail == nil && class == "" && isInstanceKeyword(lang, call.Receiver) && b.isMethod(r.key) {
		if parent := fileindex.Parent(r.key); b.isType(parent) {
			class, virtual = parent, true
		}
	}
	switch {
	case !virtual || class == "" || call.Method == "" || !b.dispatches(lang, class):
		return
	case r.fail != nil && r.fail.name != call.Method:
		return
	case r.fail == nil && fileindex.Last(r.key) != call.Method:
		return
	}
	b.narrowed[site] = true
	found := ""
	if r.fail == nil {
		found = r.key
	}
	for _, key := range b.overrides(class, call, found) {
		b.addEdge(Edge{From: from, To: key, Site: site, Kind: EdgeCHA, Resolver: resolverName(lang, "cha")})
	}
}

// linkReferences adds a reference edge from the definition that holds each
// reference to each target that resolves. A reference that does not resolve
// is no call, so it makes no record. The call that gets the value has its
// own record.
func (b *builder) linkReferences(ix *fileindex.Index) {
	if isDeclarationFile(ix.Path) {
		return
	}
	sc := b.scopes[ix.Path]
	for _, ref := range ix.References {
		from := b.runner(ref.Caller)
		site := Site{Path: ix.Path, Line: ref.Line, Column: ref.Column}
		for _, r := range b.resolveTargets(sc, ref.Caller, ref.Targets) {
			// A Go value of a named type is no function, so passing it
			// on calls nothing.
			if r.fail != nil || r.unbound || ix.Language == langcode.Go && b.isType(r.key) {
				continue
			}
			b.addEdge(Edge{From: from, To: r.key, Site: site, Kind: EdgeReference, Resolver: resolverName(ix.Language, "reference")})
		}
	}
}

// resolveTargets resolves the per-file targets of a site. Each resolution
// keeps the target that it came from. A name that two files of a unit
// define resolves to both.
func (b *builder) resolveTargets(sc *scope, caller string, targets []string) []resolution {
	var out []resolution
	for _, t := range targets {
		for _, r := range b.resolveTarget(sc, caller, t) {
			r.target = t
			out = append(out, r)
			if r.fail != nil {
				continue
			}
			for _, twin := range b.twins(r.key) {
				r.key = twin
				out = append(out, r)
			}
		}
	}
	return out
}

// resolveTarget resolves one per-file target of a site in the caller. A
// bare name can resolve through more than one wildcard import, so the
// result is a list.
func (b *builder) resolveTarget(sc *scope, caller, target string) []resolution {
	ix := sc.ix
	if target == superKeyword && ix.Language.IsECMAScript() {
		return []resolution{b.resolveSuper(caller, nil)}
	}
	if rest, ok := strings.CutPrefix(target, superKeyword+fileindex.Separator); ok {
		return []resolution{b.resolveSuper(caller, strings.Split(rest, fileindex.Separator))}
	}
	if fileindex.HasKeyPrefix(target, ix.Path) {
		return []resolution{b.resolveLocal(sc, caller, target)}
	}
	if name, rest, ok := strings.Cut(target, fileindex.Separator); ok {
		if class, ok := b.typedName(sc, caller, name); ok {
			return []resolution{b.walkTyped(class, strings.Split(rest, fileindex.Separator))}
		}
	}
	if imp, rest, ok := sc.importFor(target); ok {
		module, lead := importedModule(ix.Language, imp)
		mod, _, found := b.resolveModule(ix, module)
		if !found {
			return []resolution{{key: b.external(target), kind: EdgeDirect, rule: "external"}}
		}
		names := append(lead, rest...)
		if len(names) == 0 && ix.Language.IsECMAScript() {
			names = []string{"default"}
		}
		r := b.walk(mod, names)
		r.rule = cmp.Or(r.rule, "import")
		return []resolution{r}
	}
	if rs := b.resolveScope(sc, target); len(rs) > 0 {
		return rs
	}
	return []resolution{{key: target, kind: EdgeDirect, rule: "external", unbound: true}}
}

// resolveScope resolves a target whose first name the file does not bind:
// a name of another file of the unit, then a name of a wildcard import.
func (b *builder) resolveScope(sc *scope, target string) []resolution {
	first, rest := splitKey(target)
	var out []resolution
	for _, key := range b.unitNames(sc.ix.Path, first) {
		r := b.walk(key, rest)
		r.rule = cmp.Or(r.rule, "package")
		out = append(out, r)
	}
	if len(out) > 0 {
		return out
	}
	if len(rest) == 0 {
		if rs := b.wildcard(sc, first); len(rs) > 0 {
			return rs
		}
	}
	for _, key := range b.dotImported(sc, first) {
		r := b.walk(key, rest)
		if r.kind == EdgeDirect {
			r.kind = EdgeWildcard
		}
		r.rule = cmp.Or(r.rule, "dot-import")
		out = append(out, r)
	}
	return out
}

// dotImported returns the definitions of the name in the packages that the
// file imports with a Go dot import.
func (b *builder) dotImported(sc *scope, name string) []string {
	var keys []string
	for _, imp := range sc.dots {
		if mod, _, ok := b.resolveModule(sc.ix, imp.Module); ok {
			if u := b.units[mod]; u != nil {
				keys = append(keys, u.names[name]...)
			}
		}
	}
	return keys
}

// walkTyped resolves the names on a receiver of a declared type.
func (b *builder) walkTyped(class string, names []string) resolution {
	r := b.walk(class, names)
	r.virtual, r.declared = true, true
	return r
}

// splitKey returns the first segment of a key and the segments after it.
func splitKey(key string) (string, []string) {
	first, rest, ok := strings.Cut(key, fileindex.Separator)
	if !ok {
		return first, nil
	}
	return first, strings.Split(rest, fileindex.Separator)
}

// superKeyword is the receiver of a call of a method of the base class, as
// super().load() in Python and super.load() in JavaScript. A bare call of it
// in JavaScript calls the constructor of the base class.
const superKeyword = "super"

// resolveSuper resolves the names on super in a method of a class through
// the bases of the class. No names is the call of the base class itself,
// which runs its constructor. A builtin base, or no base in Python, gives
// the external key of the builtin, as Exception//__init__ or
// object//__init__.
func (b *builder) resolveSuper(caller string, names []string) resolution {
	class, ok := b.enclosingClass(caller)
	if !ok {
		return resolution{fail: noDefinition(superKeyword)}
	}
	rec := b.classes[class]
	lang := rec.ix.Language
	missing := false
	for _, text := range rec.class.Bases {
		base, ok := b.resolveScopeName(b.scopes[rec.ix.Path], text)
		switch {
		case !ok && langcode.IsBuiltin(lang, text):
			base = text
		case !ok:
			missing = true
			continue
		}
		switch {
		case b.kindOf(base) != NodeClass:
			return resolution{key: b.external(fileindex.Join(base, names...)), kind: EdgeDirect, rule: "super"}
		case len(names) == 0:
			return resolution{key: base, kind: EdgeDirect, rule: "super"}
		}
		if key, fail := b.method(base, names[0]); fail == nil {
			r := b.walk(key, names[1:])
			if r.kind == EdgeDirect && b.isMethod(r.key) {
				r.kind = EdgeMember
			}
			r.rule = cmp.Or(r.rule, "super")
			return r
		}
	}
	if missing || lang != langcode.Python {
		return resolution{fail: noDefinition(cmp.Or(strings.Join(names, fileindex.Separator), superKeyword))}
	}
	return resolution{key: b.external(fileindex.Join("object", names...)), kind: EdgeDirect, rule: "super"}
}

// enclosingClass returns the innermost class that holds the key.
func (b *builder) enclosingClass(key string) (string, bool) {
	for segs := fileindex.Segments(key); len(segs) > 0; segs = segs[:len(segs)-1] {
		if k := fileindex.Join(fileindex.Unit(key), segs...); b.kindOf(k) == NodeClass {
			return k, true
		}
	}
	return "", false
}

// resolveLocal resolves a key of the file. A key that names no node walks
// from its longest prefix that does, as a method of a base class. A
// variable walks on to its value. A name under a function that holds the
// caller is a parameter or a local variable of the function, and its
// declared type gives its class.
func (b *builder) resolveLocal(sc *scope, caller, target string) resolution {
	if b.g.Nodes[target] != nil {
		r := b.walk(target, nil)
		r.rule = cmp.Or(r.rule, "local")
		return r
	}
	unit, segs := fileindex.Unit(target), fileindex.Segments(target)
	for i := len(segs) - 1; i > 0; i-- {
		prefix := fileindex.Join(unit, segs[:i]...)
		if b.g.Nodes[prefix] == nil {
			continue
		}
		r := b.walk(prefix, segs[i:])
		if fileindex.HasKeyPrefix(caller, prefix) && b.isFunction(prefix) {
			r = resolution{fail: &failure{reason: fileindex.UnresolvedUnknownReceiver, name: segs[i]}}
			if class, ok := b.typedName(sc, prefix, segs[i]); ok {
				r = b.walkTyped(class, segs[i+1:])
			}
		}
		r.rule = cmp.Or(r.rule, "local")
		return r
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
		case b.units[r.key] != nil:
			keys := b.units[r.key].names[name]
			if len(keys) == 0 {
				r.fail = noDefinition(name)
				return r
			}
			r.key = keys[0]
			i++
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
		case kind == NodeVariable:
			// A function that a property of the object of the variable
			// holds is a definition under the variable, as api//go.
			if n := b.g.Nodes[fileindex.Join(r.key, name)]; n != nil {
				r.key = n.Key
				i++
				continue
			}
			value, ok := b.variableValue(r.key)
			if !ok {
				r.fail = &failure{reason: fileindex.UnresolvedUnknownReceiver, name: name}
				return r
			}
			r.key = value
		case kind == NodeClass || kind == NodeInterface:
			if isInstanceKeyword(b.byPath[fileindex.Unit(r.key)].Language, name) {
				r.virtual = true
				i++
				continue
			}
			r.class = r.key
			key, fail := b.method(r.key, name)
			if fail != nil {
				if field, ok := b.fieldType(r.key, name); ok && i+1 < len(names) {
					r.key, r.class, r.virtual, r.declared = field, "", true, true
					i++
					continue
				}
				r.fail = fail
				return r
			}
			r.key, r.kind, r.rule = key, EdgeMember, "member"
			i++
		case kind == NodeExternal || kind == "" && b.g.IsExternal(r.key):
			r.key = b.external(fileindex.Join(r.key, names[i:]...))
			r.rule = cmp.Or(r.rule, "external")
			return r
		case kind == NodeFunction || kind == NodeMethod || kind == NodeConstructor:
			// A Go call of the function gives a value of its first result,
			// as New().Run(). Else the name is a local variable or a
			// parameter of the function, and the index does not know its
			// value.
			if class, ok := b.resultType(r.key); ok {
				r.key, r.virtual, r.declared = class, true, true
				continue
			}
			r.fail = &failure{reason: fileindex.UnresolvedUnknownReceiver, name: name}
			return r
		default:
			r.fail = noDefinition(name)
			return r
		}
	}
	switch {
	case isNamespacePackage(r.key):
		r.fail = noDefinition(fileindex.Last(strings.TrimSuffix(r.key, "/")))
	case b.units[r.key] != nil:
		r.fail = noDefinition(fileindex.Last(r.key))
	}
	if b.kindOf(r.key) == NodeVariable {
		value, ok := b.variableValue(r.key)
		if !ok {
			r.fail = &failure{reason: fileindex.UnresolvedUnknownReceiver, name: fileindex.Last(r.key)}
			return r
		}
		r.key = value
	}
	return r
}

// variableValue returns the one key that a module variable holds. A
// variable with no value, or with more than one, has no known value. A
// variable that holds a variable of another file walks on to that value.
func (b *builder) variableValue(key string) (string, bool) {
	if b.reading[key] {
		return "", false
	}
	b.reading[key] = true
	defer delete(b.reading, key)

	sc := b.scopes[fileindex.Unit(key)]
	value := ""
	var values []string
	if def := b.defs[key]; def != nil && def.Kind == fileindex.DefVariable {
		values = def.Values
	}
	for _, r := range b.resolveTargets(sc, fileindex.Unit(key), values) {
		if r.fail != nil || r.unbound && !langcode.IsBuiltin(sc.ix.Language, fileindex.Unit(r.key)) {
			return "", false
		}
		k := b.deref(r.key)
		if value != "" && k != value {
			return "", false
		}
		value = k
	}
	return value, value != ""
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
		b.truncations[Truncation{Kind: TruncationExportHops, Key: fileindex.Join(path, name), Site: Site{Path: path}}] = true
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
	if n := b.g.Nodes[fileindex.Join(module, name)]; n != nil && n.Kind != NodeExport {
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
			key = b.external(joinNonEmpty(fileindex.Namespace(sc.ix.Language, ex.From), ex.Item))
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
	module, lead := importedModule(lang, imp)
	mod, _, ok := b.resolveModule(sc.ix, module)
	var key string
	switch {
	case !ok:
		key = b.external(boundNamespace(lang, imp))
	case len(lead) > 0:
		var fail *failure
		if key, fail = b.resolveName(mod, lead[0], hops+1); fail != nil {
			return "", fail
		}
	case lang.IsECMAScript():
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

// wildcard resolves a bare name through the from m import * of the file. A
// first-party module that defines the name wins. Else each external module
// of a star can define the name, so each one gets a wildcard edge.
func (b *builder) wildcard(sc *scope, name string) []resolution {
	if sc.ix.Language != langcode.Python {
		return nil
	}
	var external []resolution
	for _, st := range sc.stars {
		mod, _, ok := b.resolveModule(sc.ix, st.spec)
		if !ok {
			key := b.external(fileindex.Join(fileindex.Namespace(sc.ix.Language, st.spec), name))
			external = append(external, resolution{key: key, kind: EdgeWildcard, rule: "wildcard-external"})
			continue
		}
		if !b.starVisible(mod, name) {
			continue
		}
		if key, fail := b.resolveName(mod, name, 0); fail == nil {
			return []resolution{{key: key, kind: EdgeWildcard, rule: "wildcard"}}
		}
	}
	return external
}

func (b *builder) bind(sc *scope, name string, line int, target, rule string) string {
	if isNamespacePackage(target) {
		return target
	}
	key := fileindex.Join(sc.ix.Path, name)
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
	imp, ok := sc.bindings[segs[0]]
	if !ok {
		imp, ok = sc.types[segs[0]]
	}
	local := fileindex.Join(sc.ix.Path, segs[0])
	switch scoped := append(b.unitNames(sc.ix.Path, segs[0]), b.dotImported(sc, segs[0])...); {
	case b.g.Nodes[local] != nil:
		r = b.walk(local, segs[1:])
	case ok:
		lang := sc.ix.Language
		module, lead := importedModule(lang, imp)
		mod, _, found := b.resolveModule(sc.ix, module)
		if !found {
			return fileindex.Join(boundNamespace(lang, imp), segs[1:]...), true
		}
		r = b.walk(mod, append(lead, segs[1:]...))
	case len(scoped) > 0:
		r = b.walk(scoped[0], segs[1:])
	default:
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
	case langcode.Go:
		if dir, ok := b.golang.Resolve(spec); ok {
			return b.goPackages[dir].key, "module", true
		}
	}
	return "", "", false
}

// mayBeFirstParty reports whether a module that resolves to no file can be
// a file of the project. A search path root, an alias or a build step can
// resolve a relative module, a module whose first segment names a file or a
// directory of the project, or any module of a language with no resolver.
// A Go import path under the path of a module of the project names a
// package of the project.
func (b *builder) mayBeFirstParty(lang langcode.Code, spec string) bool {
	switch {
	case lang == langcode.Go:
		return b.golang.FirstParty(spec)
	case lang != langcode.Python && !lang.IsECMAScript():
		return true
	case strings.HasPrefix(spec, "."):
		return true
	case lang == langcode.Python:
		return b.files.HasName(firstDotted(spec))
	}
	first, _, _ := strings.Cut(spec, "/")
	return b.files.HasName(first)
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

// isFunction reports whether the key names a function, a method or a
// constructor, whose body is a scope.
func (b *builder) isFunction(key string) bool {
	return b.kindOf(key) == NodeFunction || b.isMethod(key)
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
	return bound || b.g.Nodes[fileindex.Join(module, name)] != nil
}

// isAssignment reports whether a call reached its target through an alias
// inside the file: the target has another name than the call writes, and
// the written name is not an import binding.
func isAssignment(sc *scope, call fileindex.Call, target string) bool {
	written := cmp.Or(call.Method, lastDotted(call.Callee))
	if written == "" || fileindex.Last(target) == written {
		return false
	}
	_, bound := sc.bindings[firstDotted(call.Callee)]
	return !bound
}

// boundNamespace returns the external key of the name that an import binds.
func boundNamespace(lang langcode.Code, imp fileindex.Import) string {
	if bindsRoot(lang, imp) {
		return fileindex.Namespace(lang, firstDotted(imp.Module))
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
	return fileindex.Join(unit, segment)
}
