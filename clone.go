package arena

import (
	"fmt"
	"reflect"
)

// Clone as much of T as possible into the given [Arena], recursively.
func Clone[T any](arena *Arena, existing T) *T {
	out := New[T](arena)
	cloneValue(arena, reflect.ValueOf(&existing), reflect.ValueOf(&out).Elem())
	return out
}

func cloneValue(arena *Arena, in, out reflect.Value) {
	switch in.Kind() {
	case reflect.Ptr:
		if in.IsNil() {
			return
		}
		it := in.Type().Elem()
		out.Set(reflect.NewAt(it, arena.alloc(uint64(it.Size()))))
		cloneValue(arena, in.Elem(), out.Elem())

	case reflect.Struct:
		for i := range in.NumField() {
			inf := in.Field(i)
			outf := out.Field(i)
			cloneValue(arena, inf, outf)
		}

	case reflect.Array:
		if in.Len() == 0 {
			return
		}
		for i := range in.Len() {
			cloneValue(arena, in.Index(i), out.Index(i))
		}

	case reflect.Slice:
		if in.Len() == 0 {
			return
		}
		it := in.Type().Elem()
		out.Set(reflect.SliceAt(it, arena.alloc(uint64(in.Len()*int(it.Size()))), in.Len())) //nolint:gosec
		for i := range in.Len() {
			cloneValue(arena, in.Index(i), out.Index(i))
		}

	case reflect.Map:
		if in.Len() == 0 {
			return
		}
		m := reflect.MakeMapWithSize(in.Type(), in.Len())
		it := in.MapRange()
		for it.Next() {
			ki, vi := it.Key(), it.Value()
			ko := reflect.NewAt(ki.Type(), arena.alloc(uint64(ki.Type().Size()))).Elem()
			vo := reflect.NewAt(vi.Type(), arena.alloc(uint64(vi.Type().Size()))).Elem()
			cloneValue(arena, ki, ko)
			cloneValue(arena, vi, vo)
			m.SetMapIndex(ko, vo)
		}
		out.Set(m)

	case reflect.Chan:
		panic(fmt.Sprintf("cannot clone channel %s", in.Type()))

	default:
		out.Set(in)
	}
}
