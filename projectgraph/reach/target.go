package reach

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
)

// Target is an external symbol that a query asks for. Its key is the key
// form of the import and the names under it: a Go import path with // for
// /, as os//exec//Command, and a Java class under its package, as
// java//lang//Thread//sleep. A key that ends in //* matches every external
// key under it. The zero Target is no target, and a constructor validates
// the key, so a Target in a query is well formed. Its JSON form is the key.
type Target struct {
	key string
}

// TargetByKey returns the target of a key, as yaml//load or yaml//*.
func TargetByKey(key string) (Target, error) {
	if err := validateKey(key, true); err != nil {
		return Target{}, fmt.Errorf("target %q: %w", key, err)
	}
	return Target{key: key}, nil
}

// TargetByImport returns the target of a module as the source of the
// language writes it and the names under it. A Go os/exec and Command
// gives os//exec//Command, a Python yaml and load gives yaml//load, and a
// Java java.lang.Thread and sleep gives java//lang//Thread//sleep.
func TargetByImport(lang langcode.Code, module string, names ...string) (Target, error) {
	if module == "" {
		return Target{}, errors.New("target: no module")
	}
	return TargetByKey(fileindex.Join(fileindex.Namespace(lang, module), names...))
}

// TargetsByKey returns the targets of the keys, as a list of a
// configuration gives them. The first bad key is the error.
func TargetsByKey(keys ...string) ([]Target, error) {
	targets := make([]Target, 0, len(keys))
	for _, key := range keys {
		t, err := TargetByKey(key)
		if err != nil {
			return nil, err
		}
		targets = append(targets, t)
	}
	return targets, nil
}

// Key returns the key of the target.
func (t Target) Key() string { return t.key }

func (t Target) String() string { return t.key }

// IsZero reports whether the target is the zero Target.
func (t Target) IsZero() bool { return t.key == "" }

// Matches reports whether an external key is the target, or is under the
// module of a wildcard target.
func (t Target) Matches(key string) bool {
	if prefix, ok := t.wildcard(); ok {
		return strings.HasPrefix(key, prefix+fileindex.Separator)
	}
	return key == t.key
}

// MatchesSorted reports whether a key of the list, in key order, matches
// the target.
func (t Target) MatchesSorted(keys []string) bool {
	if prefix, ok := t.wildcard(); ok {
		i, _ := slices.BinarySearch(keys, prefix+fileindex.Separator)
		return i < len(keys) && t.Matches(keys[i])
	}
	_, found := slices.BinarySearch(keys, t.key)
	return found
}

// wildcard returns the module of a wildcard target, as yaml for yaml//*.
func (t Target) wildcard() (string, bool) {
	return strings.CutSuffix(t.key, wildcard)
}

// hasMethod reports whether an unresolved call of the method can be a
// call of the target: the last segment of the target is the method, or
// the target is a wildcard.
func (t Target) hasMethod(method string) bool {
	_, ok := t.wildcard()
	return ok || fileindex.Last(t.key) == method
}

// mayImport reports whether a computed import with a static prefix can load
// the module of the target.
func (t Target) mayImport(importPrefix string) bool {
	if strings.HasPrefix(t.key, importPrefix) {
		return true
	}
	prefix, ok := t.wildcard()
	return ok && strings.HasPrefix(importPrefix, prefix)
}

func (t Target) MarshalText() ([]byte, error) { return []byte(t.key), nil }

func (t *Target) UnmarshalText(text []byte) error {
	target, err := TargetByKey(string(text))
	if err != nil {
		return err
	}
	*t = target
	return nil
}

// validateKey checks the form of a key: not empty, no empty segment, so no
// leading or trailing separator, and a wildcard only as the last segment
// after another one, and only when the key can have one.
func validateKey(key string, wildcard bool) error {
	if key == "" {
		return errors.New("the key is empty")
	}
	segments := strings.Split(key, fileindex.Separator)
	for i, s := range segments {
		switch {
		case s == "":
			return errors.New("the key has an empty segment")
		case s != "*":
		case !wildcard:
			return errors.New("the key has a wildcard")
		case i == 0 || i != len(segments)-1:
			return errors.New("the wildcard is not the last segment under a module")
		}
	}
	return nil
}
