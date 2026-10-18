package indexer

import (
	"reflect"
	"strings"
	"testing"
	"unsafe"

	"github.com/safedep/code/fileindex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fill sets every string of v to its own copy of s, and gives every slice
// two elements, so a test sees each string field of the index.
func fill(v reflect.Value, s string) {
	switch v.Kind() {
	case reflect.String:
		v.SetString(strings.Clone(s))
	case reflect.Struct:
		for i := range v.NumField() {
			fill(v.Field(i), s)
		}
	case reflect.Slice:
		v.Set(reflect.MakeSlice(v.Type(), 2, 4))
		for i := range v.Len() {
			fill(v.Index(i), s)
		}
	}
}

// strs returns the data pointer of every string of v.
func strs(v reflect.Value, out map[*byte]bool) {
	switch v.Kind() {
	case reflect.String:
		out[unsafe.StringData(v.String())] = true
	case reflect.Struct:
		for i := range v.NumField() {
			strs(v.Field(i), out)
		}
	case reflect.Slice:
		for i := range v.Len() {
			strs(v.Index(i), out)
		}
	}
}

func TestInternerSharesEveryStringOfAnIndex(t *testing.T) {
	var a, b fileindex.Index
	fill(reflect.ValueOf(&a).Elem(), "same")
	fill(reflect.ValueOf(&b).Elem(), "same")

	in := interner{}
	in.index(&a)
	in.index(&b)

	data := map[*byte]bool{}
	strs(reflect.ValueOf(a), data)
	strs(reflect.ValueOf(b), data)
	assert.Len(t, data, 1, "a string field of fileindex.Index is not interned")
	require.Len(t, a.Calls, 2)
	assert.Equal(t, 2, cap(a.Calls), "the record slices fit their length")
}
