package arena

import (
	"fmt"
	"testing"
	"unsafe"

	"github.com/alecthomas/assert/v2"
)

type Struct struct {
	String  string
	Int     int
	Float32 float32
	Float64 float64
	Int32   int32
	Int64   int64
	Uint32  uint32
	Uint64  uint64
}

func TestArenaObjectTooLarge(t *testing.T) {
	arena := Create(4)
	assert.Panics(t, func() { New[Struct](arena) })
}

func TestArenaLimit(t *testing.T) {
	arena := Create(int(unsafe.Sizeof(Struct{})), WithLimit(2))
	assert.Equal(t, 1, len(arena.chunks))
	New[Struct](arena)
	assert.Equal(t, 2, len(arena.chunks))
	New[Struct](arena)
	assert.Equal(t, 2, len(arena.chunks), "should not expand once limit is reached")
	assert.Panics(t, func() { New[Struct](arena) })
	assert.Equal(t, 2, len(arena.chunks), "should not expand once limit is reached")
}

func TestArenaReuse(t *testing.T) {
	arena := Create(100)
	a := Value[Struct](arena, Struct{Int: 42})
	a.String = "hello"
	arena.Reset()
	b := New[Struct](arena)
	assert.Equal(t, a, b)
	assert.Equal(t, a, &Struct{})
	assert.Equal(t, unsafe.Pointer(unsafe.StringData(b.String)), unsafe.Pointer(unsafe.StringData(a.String)))
}

func TestSlice(t *testing.T) {
	arena := Create(1000)
	s := Make[Struct](arena, 10, 10)
	for i := range 10 {
		s[i].Int = i
	}
	for i := range 10 {
		assert.Equal(t, i, s[i].Int)
	}
}

func TestAppend(t *testing.T) {
	arena := Create(10000)
	s := make([]Struct, 0, 10)
	expected := unsafe.SliceData(s)
	s = Append(arena, s, Struct{Int: 1})
	actual := unsafe.SliceData(s)
	assert.Equal(t, 1, s[0].Int)
	// Ensure the slice was not reallocated.
	assert.Equal(t, unsafe.Pointer(expected), unsafe.Pointer(actual))
	for i := range 20 {
		s = Append(arena, s, Struct{Int: i + 2})
	}
	actual = unsafe.SliceData(s)
	// Ensure the slice was reallocated.
	assert.NotEqual(t, unsafe.Pointer(expected), unsafe.Pointer(actual))
	for i := range 20 {
		assert.Equal(t, i+1, s[i].Int)
	}
}

func TestAppendShort(t *testing.T) {
	arena := Create(1024)
	s := Make[Struct](arena, 0, 10)
	s = Append(arena, s, Struct{Int: 1})
	assert.Equal(t, 1, s[0].Int)
}

func TestNewString(t *testing.T) {
	arena := Create(100)
	s := String(arena, "hello")
	assert.Equal(t, "hello", s)
}

func TestNewInPlace(t *testing.T) {
	arena := Create(100)
	a := Value[Struct](arena, Struct{Int: 42})
	assert.Equal(t, 42, a.Int)
}

func TestSetField(t *testing.T) {
	arena := Create(1024 * 1024)
	array := make([]*Struct, 1000)
	for i := range 1000 {
		a := New[Struct](arena)
		a.Int = i
		array[i] = a
	}
	for i := range 1000 {
		assert.Equal(t, i, array[i].Int)
	}
}

func BenchmarkArena(b *testing.B) {
	arena := Create(32 * 1024 * 1024) // 32Mb chunk size

	for _, objectCount := range []int{100, 1_000, 10_000, 100_000, 1_000_000} {
		b.Run(fmt.Sprintf("%d", objectCount), func(b *testing.B) {
			array := make([]*Struct, objectCount)
			arena.Reset()
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				for j := range objectCount {
					array[j] = New[Struct](arena)
				}
			}
		})
	}
}

func BenchmarkGoRuntime(b *testing.B) {
	for _, objectCount := range []int{100, 1_000, 10_000, 100_000, 1_000_000} {
		array := make([]*Struct, objectCount)
		b.Run(fmt.Sprintf("%d", objectCount), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				for j := range objectCount {
					array[j] = new(Struct)
				}
			}
		})
	}
}

func BenchmarkArenaAppend(b *testing.B) {
	arena := Create(512 * 1024 * 1024) // 512MB chunk size

	for _, objectCount := range []int{100, 1_000, 10_000} {
		b.Run(fmt.Sprintf("%d", objectCount), func(b *testing.B) {
			arena.Reset()
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				s := Make[Struct](arena, 0, 16)
				for j := range objectCount {
					s = Append(arena, s, Struct{Int: j})
				}
			}
		})
	}
}

func BenchmarkGoRuntimeAppend(b *testing.B) {
	for _, objectCount := range []int{100, 1_000, 10_000} {
		b.Run(fmt.Sprintf("%d", objectCount), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				s := make([]Struct, 0, 16)
				for j := range objectCount {
					s = append(s, Struct{Int: j}) //nolint:staticcheck
				}
			}
		})
	}
}

func BenchmarkReset(b *testing.B) {
	arena := Create(64 * 1024 * 1024) // 64MB chunk size

	for _, objectCount := range []int{100, 1_000, 10_000} {
		b.Run(fmt.Sprintf("%d", objectCount), func(b *testing.B) {
			for range b.N {
				s := Make[Struct](arena, 0, 16)
				for j := range objectCount {
					s = Append(arena, s, Struct{Int: j})
				}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				arena.Reset()
			}
		})
	}
}
