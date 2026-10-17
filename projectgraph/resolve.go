package projectgraph

import (
	"cmp"
	"strings"
	"unicode"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
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
	rule string
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
		for _, r := range b.resolveTargets(sc, caller, targets) {
			// A Java call of a name that nothing binds runs on this, as
			// a method that a base of a dependency or an override in a
			// subclass gives. A builtin function, as println of Kotlin,
			// runs unless the class has a method of the name.
			if r.unbound && sc.profile.thisCalls && call.Kind == fileindex.CallCall && call.Receiver == "" && isIdentifier(r.key) {
				if class, ok := b.enclosingClass(caller); ok {
					if typed := b.walkTyped(class, []string{b.selfName(class, caller, r.key)}); typed.fail == nil || !langcode.IsBuiltin(ix.Language, r.key) {
						r = typed
					}
				}
			}
			b.linkCHA(call, r, from, site)
			// A declared type gives the class of the receiver, so the
			// link step owns the record of the site and no name edge
			// goes from it.
			if r.declared && (r.fail == nil || r.fail.name == call.Method) && (r.fail == nil || !b.forwards(r.class)) {
				b.narrow(site, call.Callee)
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
			if r.dynamic {
				b.addUnresolved(UnresolvedSite{Caller: from, Site: site, Text: call.Callee, Receiver: call.Receiver, Reason: fileindex.UnresolvedComputedName})
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
// edge, unless its type is structural, and its record comes from the link
// step.
//
// A receiver that a call of the class makes, as p = Parser(), holds an
// instance of that class only, so it gets no cha edge.
func (b *builder) linkCHA(call fileindex.Call, r resolution, from string, site Site) {
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
		return
	case r.fail != nil && r.fail.name != call.Method:
		return
	case r.fail == nil && fileindex.Last(r.key) != call.Method:
		return
	}
	b.narrow(site, call.Callee)
	found := ""
	if r.fail == nil {
		found = r.key
	}
	overrides := b.overrides(class, call, found)
	for _, key := range overrides {
		b.addEdge(Edge{From: from, To: key, Site: site, Kind: EdgeCHA, Resolver: resolverName(lang, "cha")})
	}
	// An ECMAScript type is structural. A value of an interface type can be
	// an object literal or an instance of a class with no implements, and an
	// abstract method with no first-party override has its code elsewhere.
	// So such a site also gets the field edges of the method name.
	if profileOf(lang).structuralTypes && (b.kindOf(class) == NodeInterface || r.fail != nil && len(overrides) == 0) {
		b.structural[siteCall{site, call.Callee}] = true
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
			if r.fail != nil || r.unbound || sc.profile.typeConversions && b.isType(r.key) {
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
		if class, ok := b.memberType(sc, caller, name); ok {
			return []resolution{b.walkTyped(class, strings.Split(rest, fileindex.Separator))}
		}
	}
	if imp, rest, ok := sc.importFor(target); ok {
		module, lead := importedModule(ix.Language, imp)
		names := append(lead, rest...)
		mod, _, found := b.resolveModule(ix, module)
		if !found {
			return []resolution{{key: b.importedExternal(ix, target, module, names), kind: EdgeDirect, rule: "external"}}
		}
		if len(names) == 0 && sc.profile.defaultExport != "" {
			names = []string{sc.profile.defaultExport}
		}
		r := b.walk(mod, names)
		r.rule = cmp.Or(r.rule, "import")
		return []resolution{r}
	}
	if rs := b.resolveScope(sc, caller, target); len(rs) > 0 {
		return rs
	}
	return []resolution{{key: target, kind: EdgeDirect, rule: "external", unbound: true}}
}

// resolveScope resolves a target whose first name the file does not bind:
// a name of another file of the unit, then a name of a wildcard import. A
// Java class sees its methods, the methods of its bases and its nested
// classes with no receiver.
func (b *builder) resolveScope(sc *scope, caller, target string) []resolution {
	first, rest := splitKey(target)
	if sc.profile.thisCalls && len(rest) == 0 {
		if r, ok := b.thisMethod(caller, first); ok {
			return []resolution{r}
		}
	}
	if key, ok := b.nestedType(sc, caller, first); ok {
		r := b.walk(key, rest)
		r.rule = cmp.Or(r.rule, "nested")
		return []resolution{r}
	}
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
	for _, key := range b.wildcardImported(sc, first) {
		r := b.walk(key, rest)
		switch {
		case sc.profile.exactWildcards:
			r.rule = cmp.Or(r.rule, "wildcard-import")
		case r.kind == EdgeDirect:
			r.kind = EdgeWildcard
			r.rule = cmp.Or(r.rule, "dot-import")
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

// walkTyped resolves the names on a receiver of a declared type.
func (b *builder) walkTyped(class string, names []string) resolution {
	r := b.walkFrom(resolution{key: class, kind: EdgeDirect, virtual: true}, names)
	r.virtual, r.declared = true, true
	return r
}

// forwards reports whether a class forwards a call that it has no method
// for to another type, as a Rust type with Deref runs the methods of its
// target. The walk does not know that type, so the receiver is unknown.
func (b *builder) forwards(class string) bool {
	m := profileOf(b.languageOf(class)).forwardMethod
	if m == "" || class == "" {
		return false
	}
	_, ok := b.ownMethod(class, m)
	return ok
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
			return resolution{key: b.deriveExternal(base, names), kind: EdgeDirect, rule: "super"}
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
	object := profileOf(lang).objectBase
	if missing || object == "" {
		return resolution{fail: noDefinition(cmp.Or(strings.Join(names, fileindex.Separator), superKeyword))}
	}
	return resolution{key: b.external(fileindex.Join(object, names...)), kind: EdgeDirect, rule: "super"}
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
			if class, ok := b.memberType(sc, prefix, segs[i]); ok {
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
	return b.walkFrom(resolution{key: key, kind: EdgeDirect}, names)
}

// walkFrom walks the names from the key of r. A virtual r is an instance of
// its class, whose class methods the walk does not read.
func (b *builder) walkFrom(r resolution, names []string) resolution {
	instance := r.virtual
	for i := 0; i < len(names); {
		name := names[i]
		switch kind := b.kindOf(r.key); {
		case b.units[r.key] != nil:
			keys := b.units[r.key].names[name]
			if len(keys) == 0 {
				key, ok := b.unitMember(r.key, name)
				if !ok {
					r.fail = noDefinition(name)
					return r
				}
				keys = []string{key}
			}
			r.key = keys[0]
			i++
		case b.unindexed(r.key):
			return r
		case isNamespacePackage(r.key) || kind == NodeModule:
			key, fail := b.resolveName(r.key, name, 0)
			if fail != nil {
				if value, ok := b.defaultValue(r.key, name); ok {
					r.key = value
					continue
				}
				if getter, ok := b.moduleGetattr(r.key); ok {
					r.key, r.kind, r.rule, r.dynamic = getter, EdgeDirect, "module-getattr", true
					return r
				}
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
			lang := b.byPath[fileindex.Unit(r.key)].Language
			if isInstanceKeyword(lang, name) {
				r.virtual = true
				i++
				continue
			}
			// Client.new makes an instance of the class, and the edge to
			// the class leads to its constructor.
			if c := profileOf(lang).constructorCall; c != "" && name == c {
				if _, own := b.ownMethod(r.key, name); !own {
					instance = true
					i++
					continue
				}
			}
			// self.class in Ruby is the class of the instance, whose class
			// methods a call on it runs.
			if c := profileOf(lang).classOf; c != "" && name == c {
				instance, r.virtual = false, false
				i++
				continue
			}
			// A call on the class itself, as Report.build in Ruby, runs a
			// method of the class before a method of its instances.
			if prefix := profileOf(lang).classMethodPrefix; prefix != "" && !instance && !r.virtual {
				if key, ok := b.ownMethod(r.key, prefix+name); ok {
					r.key, r.kind, r.rule, r.class = key, EdgeMember, "member", r.key
					i++
					continue
				}
			}
			r.class = r.key
			// A dependency base can give the method that a call names
			// last. A name before it is an attribute of the instance,
			// whose value the walk does not know, so the field and name
			// edges of the call apply.
			last := i+1 == len(names)
			key, ok := b.ownMethod(r.key, name)
			switch {
			case ok:
				r.key, r.kind, r.rule = key, EdgeMember, "member"
				i++
				continue
			case !last:
				if field, ok := b.fieldType(r.key, name); ok {
					r.key, r.class, r.virtual, r.declared = field, "", true, true
					i++
					continue
				}
			}
			base, external := b.externalBase(r.key)
			switch {
			case b.forwards(r.key):
				r.fail = &failure{reason: fileindex.UnresolvedUnknownReceiver, name: name}
				return r
			case external && last:
				r.key, r.kind, r.rule = b.deriveExternal(base, []string{name}), EdgeMember, "member"
				i++
			case external:
				r.fail = &failure{reason: fileindex.UnresolvedUnknownReceiver, name: name}
				return r
			default:
				r.fail = noDefinition(name)
				return r
			}
		case kind == NodeExternal || kind == "" && b.g.IsExternal(r.key):
			r.key = b.deriveExternal(r.key, names[i:])
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

// defaultValue returns the class or the function that a module exports as
// its whole value, as module.exports = class {} or export default class
// {}, when the module has no name of its own for the member. A default
// import and a require bind that value, so a member of it, as run in
// new Job().run(), is a member of the class.
func (b *builder) defaultValue(module, name string) (string, bool) {
	def := b.profileAt(module).defaultExport
	if def == "" || name == def {
		return "", false
	}
	key, fail := b.resolveName(module, def, 0)
	if fail != nil {
		return "", false
	}
	switch b.kindOf(b.deref(key)) {
	case NodeClass, NodeFunction:
		return key, true
	}
	return "", false
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
		if r.fail != nil || r.dynamic || r.unbound && !langcode.IsBuiltin(sc.ix.Language, fileindex.Unit(r.key)) {
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
		case profileOf(rec.ix.Language).implicitBases:
			// A Java type that no import and no package of the project
			// names is a type of java.lang, as Thread or Runnable.
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
		mod, _, found := b.resolveModule(sc.ix, module)
		if !found {
			key := fileindex.Join(boundNamespace(lang, imp), segs[1:]...)
			b.recordOrigin(sc.ix, key, module, append(lead, segs[1:]...))
			return key, true
		}
		r = b.walk(mod, append(lead, segs[1:]...))
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
// file: its project, or its unit of dependency code.
func (b *builder) resolveModule(ix *fileindex.Index, spec string) (string, string, bool) {
	r := b.spaceOf(ix.Path).resolvers[ix.Language]
	if spec == "" || r == nil {
		return "", "", false
	}
	return r.Module(ix.Path, spec)
}

// mayBeFirstParty reports whether a module that resolves to no file can be
// a file of the project or the unit of the file. A search path root, an
// alias or a build step can resolve a relative module, a module whose first
// segment names a file or a directory of the project, or any module of a
// language with no resolver. A Go import path under the path of a module of
// the project, or a Java name under a package of the project, names a file
// of the project.
func (b *builder) mayBeFirstParty(ix *fileindex.Index, spec string) bool {
	r := b.spaceOf(ix.Path).resolvers[ix.Language]
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
