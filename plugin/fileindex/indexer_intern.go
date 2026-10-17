package fileindex

import fidx "github.com/safedep/code/fileindex"

// interner makes the indexes of a run share one copy of each string. An
// index holds the key of its caller in each call and the same targets and
// kinds in many records, and a JSON decode gives each of them its own
// copy. The result of a run holds every index, so the copies add up.
type interner map[string]string

func intern[S ~string](t interner, s *S) {
	if *s == "" {
		return
	}
	if v, ok := t[string(*s)]; ok {
		*s = S(v)
		return
	}
	t[string(*s)] = string(*s)
}

func internAll[S ~string](t interner, ss []S) {
	for i := range ss {
		intern(t, &ss[i])
	}
}

func (t interner) typed(ts []fidx.Typed) {
	for i := range ts {
		intern(t, &ts[i].Name)
		intern(t, &ts[i].Type)
	}
}

func (t interner) flags(fs []fidx.Flag) {
	for i := range fs {
		intern(t, &fs[i].Kind)
	}
}

// fit copies a slice into one of its length. An append that builds the
// records of an index can leave up to half of the room free.
func fit[T any](s []T) []T {
	if cap(s) == len(s) {
		return s
	}
	return append(make([]T, 0, len(s)), s...)
}

// index interns every string of the index in place, and fits its record
// slices to their length.
func (t interner) index(ix *fidx.Index) {
	ix.Imports = fit(ix.Imports)
	ix.Exports = fit(ix.Exports)
	ix.Definitions = fit(ix.Definitions)
	ix.Classes = fit(ix.Classes)
	ix.Calls = fit(ix.Calls)
	ix.References = fit(ix.References)
	ix.Unresolved = fit(ix.Unresolved)
	intern(t, &ix.Version)
	intern(t, &ix.Path)
	intern(t, &ix.Language)
	intern(t, &ix.ContentHash)
	intern(t, &ix.Package)
	for i := range ix.Imports {
		imp := &ix.Imports[i]
		intern(t, &imp.Module)
		intern(t, &imp.Item)
		intern(t, &imp.Alias)
		intern(t, &imp.Kind)
		intern(t, &imp.Namespace)
	}
	for i := range ix.Exports {
		ex := &ix.Exports[i]
		intern(t, &ex.Name)
		intern(t, &ex.Target)
		intern(t, &ex.From)
		intern(t, &ex.Item)
	}
	for i := range ix.Definitions {
		def := &ix.Definitions[i]
		intern(t, &def.Key)
		intern(t, &def.Kind)
		intern(t, &def.Name)
		intern(t, &def.Parent)
		internAll(t, def.Decorators)
		internAll(t, def.Values)
		t.typed(def.Params)
		t.typed(def.Locals)
		intern(t, &def.Returns)
		t.flags(def.Flags)
	}
	for i := range ix.Classes {
		c := &ix.Classes[i]
		intern(t, &c.Key)
		internAll(t, c.Bases)
		internAll(t, c.Methods)
		t.typed(c.Fields)
	}
	for i := range ix.Calls {
		c := &ix.Calls[i]
		intern(t, &c.Caller)
		intern(t, &c.Callee)
		internAll(t, c.Targets)
		intern(t, &c.Receiver)
		intern(t, &c.Method)
		intern(t, &c.Kind)
		for j := range c.Args {
			intern(t, &c.Args[j].Shape)
			internAll(t, c.Args[j].Targets)
		}
	}
	for i := range ix.References {
		r := &ix.References[i]
		intern(t, &r.Caller)
		intern(t, &r.Text)
		internAll(t, r.Targets)
	}
	for i := range ix.Unresolved {
		u := &ix.Unresolved[i]
		intern(t, &u.Caller)
		intern(t, &u.Text)
		intern(t, &u.Receiver)
		intern(t, &u.Method)
		intern(t, &u.Reason)
	}
	t.flags(ix.Flags)
	for i := range ix.Truncated {
		intern(t, &ix.Truncated[i].Kind)
		intern(t, &ix.Truncated[i].Namespace)
	}
}
