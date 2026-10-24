package projectgraph

import (
	"cmp"
	"slices"
	"strings"
	"unicode"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/projectgraph/modules"
)

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
	rule rule
	fail *failure
	// unbound marks a target that no definition and no import of the file
	// binds, as print or console//log.
	unbound bool
	// dynamic marks a resolution to code that computes the value, as the
	// __getattr__ of a module. The value is not known, so the site also
	// gets a computed-name record.
	dynamic bool
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
		resolutions := b.resolveTargets(sc, caller, targets)
		// A literal, as None, is no function. A name that holds only a
		// literal has a value that the index does not know at the call,
		// as a field that another method sets.
		literals := slices.DeleteFunc(slices.Clone(resolutions), func(r resolution) bool { return !r.unbound || !sc.profile.literals[r.key] })
		if len(literals) == len(resolutions) && len(literals) > 0 {
			b.addUnresolved(UnresolvedSite{Caller: from, Site: site, Text: call.Callee, Receiver: call.Receiver, Method: cmp.Or(call.Method, lastWritten(call.Callee)), Reason: fileindex.UnresolvedUnknownReceiver})
			continue
		}
		for _, r := range resolutions {
			if r.unbound && sc.profile.literals[r.key] {
				continue
			}
			if r.unbound && sc.profile.thisCalls {
				r = b.thisCall(ix, caller, call, r)
			}
			overrides := b.linkCHA(call, r, from, site)
			// A declared type gives the class of the receiver, so the
			// link step owns the record of the site and no name edge
			// goes from it.
			if r.declared && (r.fail == nil || r.fail.name == call.Method) && (r.fail == nil || !b.forwards(r.class)) {
				b.narrow(site, call.Callee)
			}
			if r.fail != nil {
				reason := r.fail.reason
				// A value of an interface or a trait that no type of the
				// graph implements holds a type of code elsewhere.
				if reason == fileindex.UnresolvedNoDefinition && overrides == 0 && r.declared && sc.profile.abstractReceiver != "" && b.kindOf(r.class) == NodeInterface {
					reason = sc.profile.abstractReceiver
				}
				b.addUnresolved(UnresolvedSite{
					Caller:   from,
					Site:     site,
					Text:     call.Callee,
					Receiver: call.Receiver,
					Method:   cmp.Or(call.Method, r.fail.name),
					Reason:   reason,
				})
				continue
			}
			if r.dynamic {
				b.addUnresolved(UnresolvedSite{Caller: from, Site: site, Text: call.Callee, Receiver: call.Receiver, Reason: fileindex.UnresolvedComputedName})
			}
			if r.unbound && !b.unboundEdge(ix, call, from, site, r, unknown[[2]int{call.Line, call.Column}]) {
				continue
			}
			// The index of some languages records such a call itself, and
			// that record stays the one record of the site.
			if reason, ok := b.onCallResult(sc.profile, call, r); ok && !unknown[[2]int{call.Line, call.Column}] {
				b.addUnresolved(UnresolvedSite{Caller: from, Site: site, Text: call.Callee, Receiver: call.Receiver, Method: call.Method, Reason: reason})
			}
			switch {
			case r.kind == EdgeDirect && isAssignment(sc, call, r.target):
				r.kind, r.rule = EdgeAssignment, ruleAssignment
			case r.kind == EdgeDirect && call.Receiver != "" && b.isMethod(r.key):
				r.kind, r.rule = EdgeMember, ruleMember
			}
			if r.kind == EdgeMember && r.declared {
				r.rule = ruleDeclaredType
			}
			b.link(ix.Language, from, r.key, site, r.kind, r.rule)
		}
	}
}

// onCallResult returns the reason of the record of a call whose resolution
// took its method as a member of the external key of the called name of a
// call on a call result, as html//NewTokenizer//Next for
// html.NewTokenizer(r).Next(). The link step does not know the type of
// that result, so the key can name no symbol, and a target of the method
// of the real type would read as no-such-target. The edge stays, because
// the called name can be a type, as in a Go conversion
// http.HandlerFunc(f).ServeHTTP(w, r). The reason is no-dependency-source
// when no unit of dependency code gives the package of the key.
//
// A construction gives an instance of the called class, so the key is
// right: new Yaml().load() in Java, or Yaml().load() in Python or Kotlin,
// whose classes start with an upper case letter. A Go name starts with an
// upper case letter when the package exports it, so it tells nothing.
func (b *builder) onCallResult(p *profile, call fileindex.Call, r resolution) (fileindex.UnresolvedReason, bool) {
	called, ok := calledName(call.Receiver)
	if !ok || r.fail != nil || call.Method == "" || b.kindOf(r.key) != NodeExternal {
		return "", false
	}
	if strings.HasPrefix(call.Receiver, "new ") || !p.typeConversions && startsUpper(called) {
		return "", false
	}
	if fileindex.Last(r.key) != call.Method || fileindex.Last(fileindex.Parent(r.key)) != called {
		return "", false
	}
	if _, provided := b.unitDefinitions(b.origins[r.key]); !provided {
		return fileindex.UnresolvedNoDependencySource, true
	}
	return fileindex.UnresolvedUnknownReceiver, true
}

// calledName returns the name of the call that ends a receiver as written,
// as NewTokenizer for html.NewTokenizer(r) and new for Vec::<u8>::new().
func calledName(receiver string) (string, bool) {
	head, ok := strings.CutSuffix(receiver, ")")
	if !ok {
		return "", false
	}
	for depth := 1; depth > 0; {
		i := strings.LastIndexAny(head, "()")
		if i < 0 {
			return "", false
		}
		if head[i] == ')' {
			depth++
		} else {
			depth--
		}
		head = head[:i]
	}
	head = strings.TrimSuffix(withoutTypeArguments(head), "::")
	name := lastWritten(head)
	return name, isIdentifier(name)
}

// withoutTypeArguments removes the type arguments that end a callee as
// written, as [int] of New[int] in Go or <u8> of new::<u8> in Rust.
func withoutTypeArguments(callee string) string {
	closer := map[byte]byte{']': '[', '>': '<'}
	if callee == "" || closer[callee[len(callee)-1]] == 0 {
		return callee
	}
	end, open := callee[len(callee)-1], closer[callee[len(callee)-1]]
	depth := 0
	for i := len(callee) - 1; i >= 0; i-- {
		switch callee[i] {
		case end:
			depth++
		case open:
			depth--
			if depth == 0 {
				return callee[:i]
			}
		}
	}
	return callee
}

// thisCall resolves a call of a name that nothing binds in a language where
// such a call runs on this, as a Java method that a base of a dependency
// or an override in a subclass gives. A builtin function, as println of
// Kotlin, runs unless the class has a method of the name.
func (b *builder) thisCall(ix *fileindex.Index, caller string, call fileindex.Call, r resolution) resolution {
	if call.Kind != fileindex.CallCall || call.Receiver != "" || !isIdentifier(r.key) {
		return r
	}
	class, ok := b.enclosingClass(caller)
	if !ok {
		return r
	}
	if typed := b.walkTyped(class, []string{b.selfName(class, caller, r.key)}); typed.fail == nil || !langcode.IsBuiltin(ix.Language, r.key) {
		return typed
	}
	return r
}

// unboundEdge reports whether a call of an unbound name gets an edge to
// its external node, and records the call when it does not. A call on an
// unknown receiver, as plugin.run(), has the fallback target plugin//run,
// which names no module. The record of the index holds the call, so it
// makes no edge. A name that no builtin binds is a parameter or a local
// variable, as cb in def run(cb): cb(x). Its value is not known, so the
// call is a record and not an external.
func (b *builder) unboundEdge(ix *fileindex.Index, call fileindex.Call, from string, site Site, r resolution, unknownReceiver bool) bool {
	if unknownReceiver {
		return false
	}
	if !langcode.IsBuiltin(ix.Language, fileindex.Unit(r.key)) {
		b.addUnresolved(UnresolvedSite{
			Caller:   from,
			Site:     site,
			Text:     call.Callee,
			Receiver: call.Receiver,
			Method:   cmp.Or(call.Method, fileindex.Last(r.key)),
			Reason:   fileindex.UnresolvedUnknownReceiver,
		})
		return false
	}
	b.external(r.key)
	return true
}

// linkCHA narrows the site of a method call whose receiver can hold an
// instance of a subclass of a known class: the instance keyword or a
// declared type. It adds a cha edge to the method of each subclass that the
// call can run besides the resolution, also when the class itself has no
// such method, as an abstract method. A narrowed site gets no field or name
// edge, unless its type is structural, and its record comes from the link
// step.
//
// A receiver that a call of the class makes, as p = Parser(), holds an
// instance of that class only, so it gets no cha edge. An override in a
// unit of dependency code gets its edge through its external key. It
// returns the count of the overrides.
func (b *builder) linkCHA(call fileindex.Call, r resolution, from string, site Site) int {
	lang := b.byPath[site.Path].Language
	class, virtual := r.class, r.virtual
	// A Java call with no receiver runs on this.
	this := isInstanceKeyword(lang, call.Receiver) || profileOf(lang).thisCalls && call.Receiver == "" && call.Kind == fileindex.CallCall
	// A static method is no virtual method, unless the call binds it late,
	// as static::make() of PHP.
	late := call.Receiver != "" && call.Receiver == profileOf(lang).lateStatic
	if r.fail == nil && class == "" && this && b.isMethod(r.key) && (!b.defs[r.key].Static || late) {
		if parent := fileindex.Parent(r.key); b.isType(parent) {
			class, virtual = parent, true
		}
	}
	// A call on the instance keyword of a method that the class does not
	// define, as self.step() in a default method of a Rust trait, runs the
	// method of a subclass.
	if r.fail != nil && class != "" && isInstanceKeyword(lang, call.Receiver) {
		virtual = true
	}
	switch {
	case !virtual || class == "" || call.Method == "" || !b.dispatches(lang, class):
		return 0
	case r.fail != nil && r.fail.name != call.Method:
		return 0
	case r.fail == nil && fileindex.Last(r.key) != call.Method:
		return 0
	}
	b.narrow(site, call.Callee)
	found := ""
	if r.fail == nil {
		found = r.key
	}
	overrides := b.overrides(class, call, found)
	for _, key := range overrides {
		b.link(lang, from, b.entryKey(from, key), site, EdgeCHA, ruleCHA)
	}
	// An ECMAScript type is structural. A value of an interface type can be
	// an object literal or an instance of a class with no implements, and an
	// abstract method with no first-party override has its code elsewhere.
	// So such a site also gets the field edges of the method name.
	if profileOf(lang).structuralTypes && (b.kindOf(class) == NodeInterface || r.fail != nil && len(overrides) == 0) {
		b.structural[siteCall{site, call.Callee}] = true
	}
	return len(overrides)
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
			if r.fail != nil || r.unbound || sc.profile.typeConversions && b.isType(r.key) {
				continue
			}
			b.link(ix.Language, from, r.key, site, EdgeReference, ruleReference)
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
	if target == superKeyword && sc.profile.superCalls {
		return []resolution{b.resolveSuper(caller, nil)}
	}
	if rest, ok := strings.CutPrefix(target, superKeyword+fileindex.Separator); ok && !sc.profile.superModule {
		return []resolution{b.resolveSuper(caller, strings.Split(rest, fileindex.Separator))}
	}
	if fileindex.HasKeyPrefix(target, ix.Path) {
		return []resolution{b.resolveLocal(sc, caller, target)}
	}
	if name, rest, ok := strings.Cut(target, fileindex.Separator); ok {
		if class, _, ok := b.memberType(sc, caller, name); ok {
			return []resolution{b.walkTyped(class, strings.Split(rest, fileindex.Separator))}
		}
	}
	if imp, rest, ok := sc.importFor(target); ok {
		module, lead := importedModule(ix.Language, imp)
		names := append(lead, rest...)
		mods := b.resolveModules(ix, module, formOf(imp))
		if len(mods) == 0 {
			return []resolution{b.importedWalk(ix, target, module, formOf(imp), names)}
		}
		if len(names) == 0 && sc.profile.defaultExport != "" {
			names = []string{sc.profile.defaultExport}
		}
		var out []resolution
		for _, m := range mods {
			out = append(out, b.walkAs(m.key, names, ruleImport))
		}
		return out
	}
	if rs := b.resolveScope(sc, caller, target); len(rs) > 0 {
		return rs
	}
	return []resolution{{key: target, kind: EdgeDirect, rule: ruleExternal, unbound: true}}
}

// resolveScope resolves a target whose first name the file does not bind:
// a name of another file of the unit, then a name of a wildcard import. A
// Java class sees its methods, the methods of its bases and its nested
// classes with no receiver.
func (b *builder) resolveScope(sc *scope, caller, target string) []resolution {
	first, rest := fileindex.Unit(target), fileindex.Segments(target)
	if sc.profile.thisCalls && len(rest) == 0 {
		if r, ok := b.thisMethod(caller, first); ok {
			return []resolution{r}
		}
	}
	if key, ok := b.nestedType(sc, caller, first); ok {
		return []resolution{b.walkAs(key, rest, ruleNested)}
	}
	var out []resolution
	for _, key := range b.unitNames(sc.ix.Path, first) {
		out = append(out, b.walkAs(key, rest, rulePackage))
	}
	if len(out) > 0 {
		return out
	}
	if len(rest) == 0 {
		if rs := b.wildcard(sc, first); len(rs) > 0 {
			return rs
		}
	}
	for _, key := range b.wildcardImported(sc, first) {
		r := b.walk(key, rest)
		switch {
		case sc.profile.exactWildcards:
			r.rule = cmp.Or(r.rule, ruleWildcardImport)
		case r.kind == EdgeDirect:
			r.kind = EdgeWildcard
			r.rule = cmp.Or(r.rule, ruleDotImport)
		}
		out = append(out, r)
	}
	if len(out) == 0 && sc.profile.scopeFallback != nil {
		return sc.profile.scopeFallback(b, sc, caller, first, rest, target)
	}
	return out
}

// wildcardImported returns the definitions of the name in the packages
// that the file imports with a Go dot import or a Java wildcard import, and
// the members of the name in the classes of a Java static wildcard import.
// A Java wildcard import of a package of the project names its class
// exactly, so its edge is direct.
func (b *builder) wildcardImported(sc *scope, name string) []string {
	var keys []string
	for _, imp := range sc.wildcards {
		mod, _, ok := b.resolveModule(sc.ix, imp.Module)
		switch {
		case !ok:
		case b.units[mod] != nil:
			keys = append(keys, b.units[mod].names[name]...)
		case b.isType(mod):
			if key, fail := b.method(mod, name); fail == nil {
				keys = append(keys, key)
			}
		}
	}
	return keys
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
			return resolution{key: b.deriveExternal(base, names), kind: EdgeDirect, rule: ruleSuper}
		case len(names) == 0:
			return resolution{key: base, kind: EdgeDirect, rule: ruleSuper}
		}
		if key, fail := b.method(base, names[0]); fail == nil {
			r := b.walkAs(key, names[1:], ruleSuper)
			if r.kind == EdgeDirect && b.isMethod(r.key) {
				r.kind = EdgeMember
			}
			return r
		}
	}
	object := profileOf(lang).objectBase
	if missing || object == "" {
		return resolution{fail: noDefinition(cmp.Or(strings.Join(names, fileindex.Separator), superKeyword))}
	}
	return resolution{key: b.external(fileindex.Join(object, names...)), kind: EdgeDirect, rule: ruleSuper}
}

// enclosingClass returns the innermost class or interface that holds the
// key.
func (b *builder) enclosingClass(key string) (string, bool) {
	for segs := fileindex.Segments(key); len(segs) > 0; segs = segs[:len(segs)-1] {
		if k := fileindex.Join(fileindex.Unit(key), segs...); b.isType(k) {
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
		return b.walkAs(target, nil, ruleLocal)
	}
	unit, segs := fileindex.Unit(target), fileindex.Segments(target)
	for i := len(segs) - 1; i > 0; i-- {
		prefix := fileindex.Join(unit, segs[:i]...)
		if b.g.Nodes[prefix] == nil {
			continue
		}
		r := b.walk(prefix, segs[i:])
		if fileindex.HasKeyPrefix(caller, prefix) && b.isFunction(prefix) {
			class, reason, ok := b.memberType(sc, prefix, segs[i])
			r = resolution{fail: &failure{reason: reason, name: segs[i]}}
			if ok {
				r = b.walkTyped(class, segs[i+1:])
			}
		}
		r.rule = cmp.Or(r.rule, ruleLocal)
		return r
	}
	return b.walkAs(unit, segs, ruleLocal)
}

// bases returns the keys of the bases of a class that the file scope
// resolves. A builtin base, as object, resolves to nothing. A Java base
// always names a type.
func (b *builder) bases(rec classRecord) []string {
	sc := b.scopes[rec.ix.Path]
	var out []string
	for _, base := range rec.class.Bases {
		key, ok := b.resolveNameIn(sc, fileindex.Parent(rec.class.Key), base)
		switch {
		case ok:
			out = append(out, key)
		case profileOf(rec.ix.Language).implicitBases && langcode.IsJavaLang(fileindex.WithoutTypeArguments(base)):
			// A Java type that no import and no package of the project
			// names is a type of java.lang, as Thread or Runnable.
			out = append(out, fileindex.Join(javaLang, fileindex.WithoutTypeArguments(base)))
		case profileOf(rec.ix.Language).implicitBases:
			out = append(out, fileindex.Namespace(rec.ix.Language, base))
		}
	}
	return out
}

// resolveScopeName resolves a dotted name as written in the file, as a base
// class mod.Base.
func (b *builder) resolveScopeName(sc *scope, text string) (string, bool) {
	// A qualified name of Ruby, PHP or Rust, as Shop::Base or \App\Base,
	// has the parts of a dotted name.
	text = qualifiedNames.Replace(strings.TrimPrefix(text, `\`))
	segs := strings.Split(text, ".")
	var r resolution
	imp, ok := sc.bindings[segs[0]]
	if !ok {
		imp, ok = sc.types[segs[0]]
	}
	local := fileindex.Join(sc.ix.Path, segs[0])
	switch scoped := append(b.unitNames(sc.ix.Path, segs[0]), b.wildcardImported(sc, segs[0])...); {
	case b.g.Nodes[local] != nil:
		r = b.walk(local, segs[1:])
	case ok:
		lang := sc.ix.Language
		module, lead := importedModule(lang, imp)
		mods := b.resolveModules(sc.ix, module, formOf(imp))
		if len(mods) == 0 {
			key := fileindex.Join(boundNamespace(lang, imp), segs[1:]...)
			b.recordOrigin(sc.ix, key, module, formOf(imp), append(lead, segs[1:]...))
			return key, true
		}
		r = b.walk(mods[0].key, append(lead, segs[1:]...))
	case len(scoped) > 0:
		r = b.walk(scoped[0], segs[1:])
	case sc.profile.qualified != nil:
		return sc.profile.qualified(b, text)
	default:
		return "", false
	}
	if r.fail != nil || r.dynamic {
		return "", false
	}
	return b.deref(r.key), true
}

// resolveNameIn resolves a dotted name as written at a key of the file. A
// Java name sees the classes nested in the classes that hold the key first.
func (b *builder) resolveNameIn(sc *scope, from, text string) (string, bool) {
	first, rest, _ := strings.Cut(text, ".")
	if nested, ok := b.nestedType(sc, from, first); ok {
		if rest == "" {
			return nested, true
		}
		r := b.walk(nested, strings.Split(rest, "."))
		return r.key, r.fail == nil
	}
	return b.resolveScopeName(sc, text)
}

// resolveModule resolves a module that a file imports in the space of the
// file: its project, or its unit of dependency code. It takes the import
// form, so a caller with a require, as a walk from an import binding,
// calls resolveModules.
func (b *builder) resolveModule(ix *fileindex.Index, spec string) (string, string, bool) {
	r := b.spaceOf(ix.Path).resolvers[ix.Language]
	if spec == "" || r == nil {
		return "", "", false
	}
	return r.Module(ix.Path, spec)
}

// resolveModules resolves a module that a file imports in the form, in the
// space of the file. An exports map of a package with no condition of the
// form gives every module of the map, so a site links each one.
func (b *builder) resolveModules(ix *fileindex.Index, spec string, form modules.Form) []moduleRef {
	r := b.spaceOf(ix.Path).resolvers[ix.Language]
	if spec == "" || r == nil {
		return nil
	}
	fr, ok := r.(modules.FormResolver)
	if !ok {
		mod, via, ok := r.Module(ix.Path, spec)
		if !ok {
			return nil
		}
		return []moduleRef{{key: mod, via: via}}
	}
	mods, via, ok := fr.Modules(ix.Path, spec, form)
	if !ok {
		return nil
	}
	out := make([]moduleRef, 0, len(mods))
	for _, mod := range mods {
		out = append(out, moduleRef{key: mod, via: via})
	}
	return out
}

// mayBeFirstParty reports whether a module that resolves to no file can be
// a file of the project or the unit of the file. A search path root, an
// alias or a build step can resolve a relative module, a module whose first
// segment names a module of the project, or any module of a language with
// no resolver. A Go import path under the path of a module of the project,
// or a Java name under a package of the project, names a file of the
// project. A resolver that reads the importer, as Python does for the
// directory of a script, gets it.
func (b *builder) mayBeFirstParty(ix *fileindex.Index, spec string) bool {
	r := b.spaceOf(ix.Path).resolvers[ix.Language]
	if ir, ok := r.(modules.ImporterResolver); ok {
		return ir.FirstPartyFrom(ix.Path, spec)
	}
	return r == nil || r.FirstParty(spec)
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
	written := cmp.Or(call.Method, lastWritten(call.Callee))
	if written == "" || fileindex.Last(target) == written {
		return false
	}
	_, bound := sc.bindings[firstDotted(call.Callee)]
	return !bound
}

var qualifiedNames = strings.NewReplacer("::", ".", `\`, ".")

// lastWritten returns the last name of a callee as written, after a ., a
// \ of PHP or a :: of Rust.
func lastWritten(callee string) string {
	i := max(strings.LastIndex(callee, "."), strings.LastIndex(callee, `\`), strings.LastIndex(callee, ":"))
	return callee[i+1:]
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
