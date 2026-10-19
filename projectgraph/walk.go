package projectgraph

import (
	"cmp"
	"strings"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
)

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

// walk resolves the names one at a time from a key: a name of a module, a
// method of a class, or a member of an external key.
func (b *builder) walk(key string, names []string) resolution {
	return b.walkFrom(resolution{key: key, kind: EdgeDirect}, names)
}

// walkAs walks the names from a key, and names the rule of the walk when
// no step of the walk named one.
func (b *builder) walkAs(key string, names []string, r rule) resolution {
	out := b.walk(key, names)
	out.rule = cmp.Or(out.rule, r)
	return out
}

// walkFrom walks the names from the key of r. A virtual r is an instance of
// its class, whose class methods the walk does not read. Each step reads
// the node of the key: a unit, a module, a variable, a class, an external
// key or a function. A step consumes the name, or moves the key on to a
// value and reads the name again, or ends the walk.
func (b *builder) walkFrom(r resolution, names []string) resolution {
	instance := r.virtual
	for i := 0; i < len(names); {
		name := names[i]
		var consumed, done bool
		switch kind := b.kindOf(r.key); {
		case b.units[r.key] != nil:
			consumed, done = b.walkUnit(&r, name)
		case b.unindexed(r.key):
			return r
		case isNamespacePackage(r.key) || kind == NodeModule:
			consumed, done = b.walkModule(&r, name)
		case kind == NodeExport:
			r.key = b.bindings[r.key]
		case kind == NodeVariable:
			consumed, done = b.walkVariable(&r, name)
		case kind == NodeClass || kind == NodeInterface:
			consumed, done = b.walkClass(&r, name, i+1 == len(names), &instance)
		case kind == NodeExternal || kind == "" && b.g.IsExternal(r.key):
			// A name before the last one under an external key is a call
			// of a function or a method whose result has the next name,
			// when a unit of dependency code gives the function.
			if i+1 < len(names) {
				if class, ok := b.externalResult(r.key, name); ok {
					r.key, r.virtual, r.declared = class, true, true
					i++
					continue
				}
			}
			r.key = b.deriveExternal(r.key, names[i:])
			r.rule = cmp.Or(r.rule, ruleExternal)
			return r
		case kind == NodeFunction || kind == NodeMethod || kind == NodeConstructor:
			consumed, done = b.walkFunction(&r, name)
		default:
			r.fail = noDefinition(name)
			return r
		}
		if done {
			return r
		}
		if consumed {
			i++
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

// walkUnit reads a name of a unit: a definition of one of its files, or a
// member of the unit.
func (b *builder) walkUnit(r *resolution, name string) (consumed, done bool) {
	keys := b.units[r.key].names[name]
	if len(keys) == 0 {
		key, ok := b.unitMember(r.key, name)
		if !ok {
			r.fail = noDefinition(name)
			return false, true
		}
		keys = []string{key}
	}
	r.key = keys[0]
	return true, false
}

// walkModule reads a name of a module or of a namespace package: a name
// that the module binds, the member of its default value, or its
// __getattr__.
func (b *builder) walkModule(r *resolution, name string) (consumed, done bool) {
	key, fail := b.resolveName(r.key, name, 0)
	if fail == nil {
		r.key = key
		return true, false
	}
	if value, ok := b.defaultValue(r.key, name); ok {
		r.key = value
		return false, false
	}
	if getter, ok := b.moduleGetattr(r.key); ok {
		r.key, r.kind, r.rule, r.dynamic = getter, EdgeDirect, ruleModuleGetattr, true
		return false, true
	}
	r.fail = fail
	return false, true
}

// walkVariable reads a name of a variable: a function that a property of
// the object of the variable holds, as api//go, or else the name on the
// value of the variable.
func (b *builder) walkVariable(r *resolution, name string) (consumed, done bool) {
	if n := b.g.Nodes[fileindex.Join(r.key, name)]; n != nil {
		r.key = n.Key
		return true, false
	}
	value, ok := b.variableValue(r.key)
	if !ok {
		r.fail = &failure{reason: fileindex.UnresolvedUnknownReceiver, name: name}
		return false, true
	}
	r.key = value
	return false, false
}

// walkClass reads a name of a class or an interface: the instance keyword,
// the constructor call or the class-of method of the language, a method of
// the class, the type of a field before the last name, or a member of an
// external base. last marks the last name of the walk, and instance holds
// whether the key is an instance of the class.
func (b *builder) walkClass(r *resolution, name string, last bool, instance *bool) (consumed, done bool) {
	lang := b.byPath[fileindex.Unit(r.key)].Language
	p := profileOf(lang)
	if isInstanceKeyword(lang, name) {
		r.virtual = true
		return true, false
	}
	// Client.new makes an instance of the class, and the edge to the class
	// leads to its constructor.
	if p.constructorCall != "" && name == p.constructorCall {
		if _, own := b.ownMethod(r.key, name); !own {
			*instance = true
			return true, false
		}
	}
	// self.class in Ruby is the class of the instance, whose class methods a
	// call on it runs.
	if p.classOf != "" && name == p.classOf {
		*instance, r.virtual = false, false
		return true, false
	}
	// A call on the class itself, as Report.build in Ruby, runs a method of
	// the class before a method of its instances.
	if p.classMethodPrefix != "" && !*instance && !r.virtual {
		if key, ok := b.ownMethod(r.key, p.classMethodPrefix+name); ok {
			r.key, r.kind, r.rule, r.class = key, EdgeMember, ruleMember, r.key
			return true, false
		}
	}
	r.class = r.key
	// A dependency base can give the method that a call names last. A name
	// before it is an attribute of the instance, whose value the walk does
	// not know, so the field and name edges of the call apply.
	if key, ok := b.ownMethod(r.key, name); ok {
		r.key, r.kind, r.rule = key, EdgeMember, ruleMember
		return true, false
	}
	if !last {
		if field, ok := b.fieldType(r.key, name); ok {
			r.key, r.class, r.virtual, r.declared = field, "", true, true
			return true, false
		}
	}
	base, external := b.externalBase(r.key)
	switch {
	case b.forwards(r.key):
		r.fail = &failure{reason: fileindex.UnresolvedUnknownReceiver, name: name}
	case external && last:
		r.key, r.kind, r.rule = b.deriveExternal(base, []string{name}), EdgeMember, ruleMember
		return true, false
	case external:
		r.fail = &failure{reason: fileindex.UnresolvedUnknownReceiver, name: name}
	default:
		r.fail = noDefinition(name)
	}
	return false, true
}

// walkFunction reads a name of a function: a Go call of the function gives
// a value of its first result, as New().Run(). Else the name is a local
// variable or a parameter of the function, and the index does not know its
// value.
func (b *builder) walkFunction(r *resolution, name string) (consumed, done bool) {
	class, reason, ok := b.resultType(fileindex.PackageOf(r.key), r.key, 0)
	if ok {
		r.key, r.virtual, r.declared = class, true, true
		return false, false
	}
	r.fail = &failure{reason: reason, name: name}
	return false, true
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
