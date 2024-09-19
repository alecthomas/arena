//	Package arena contains a very fast arena allocator for Go
//
// This package provides a very fast _almost_ lock-free arena allocator for Go. "Almost"
// lock-free because it locks when expanding the arena after a chunk has been exhausted.
package arena

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"unsafe"
)

// Arena is a (mostly) lock-free memory allocator for a fixed-size type.
//
// "Mostly" lock-free because while individual allocations are lock-free we need to lock when expanding the arena.
type Arena struct {
	lock      sync.Mutex
	chunkSize int64
	limit     int

	cursor      atomic.Int64
	chunkCursor int64
	current     []byte
	chunks      [][]byte
}

type contextKey struct{}

// WithContext returns a new context with the given arena attached.
func WithContext(ctx context.Context, arena *Arena) context.Context {
	return context.WithValue(ctx, contextKey{}, arena)
}

// FromContext returns the arena attached to the given context.
func FromContext(ctx context.Context) *Arena {
	return ctx.Value(contextKey{}).(*Arena) //nolint:forcetypeassert
}

// New creates a new zeroed object in the arena.
//
// New will typically be inlined.
func New[T any](arena *Arena) *T {
	var t T
	return (*T)(arena.alloc(int(unsafe.Sizeof(t))))
}

// Value creates space for a new object in the arena and copies "value" into it.
//
// eg.
//
//	a := Value[Struct](arena, Struct{...})
//
// Typically value will be inlined and won't escape to the heap.
func Value[T any](arena *Arena, value T) *T {
	var t T
	out := (*T)(arena.alloc(int(unsafe.Sizeof(t))))
	*out = value
	return out
}

// Make creates a new slice of T in the arena with the given size and capacity.
//
// Use [Append] to grow and add elements to the slice within the arena.
//
// The returned slice can be used with `append()`, but once the capacity is
// exhausted a new slice will be allocated from the Go heap, not the arena.
//
// Make will typically be inlined.
func Make[T any](arena *Arena, size, cap int) []T {
	var t T
	out := unsafe.Slice((*T)(arena.alloc(int(unsafe.Sizeof(t))*cap)), cap)
	return out[:size]
}

// Append elements to a slice.
//
// Neither the slice nor the elements need have been allocated from the arena.
//
// If the slice has sufficient capacity, the elements will be appended
// to it and the slice returned as-is without allocating new memory.
//
// If the slice does not have sufficient capacity, a new slice will be
// allocated from the arena and the existing slice and elements copied
// into it.
//
// Append will typically be inlined.
func Append[T any](arena *Arena, slice []T, elements ...T) []T {
	if cap(slice) >= len(slice)+len(elements) {
		return append(slice, elements...)
	}
	// Separate function to allow inlining.
	return growSlice(arena, slice, elements)
}

func growSlice[T any](arena *Arena, slice []T, elements []T) []T {
	var t T
	newLen := len(slice) + len(elements)
	capacity := cap(slice)
	for newLen >= capacity {
		capacity *= 2
	}
	out := unsafe.Slice((*T)(arena.alloc(int(unsafe.Sizeof(t))*capacity)), capacity)
	copy(out, slice)
	copy(out[len(slice):], elements)
	return out[:newLen]
}

// String creates a new string in the arena.
//
// The data for "value" is copied into the arena and a new string returned using that data.
//
// String will typically be inlined.
func String(arena *Arena, value string) string {
	arenaData := arena.alloc(len(value))
	copy(unsafe.Slice((*byte)(arenaData), len(value)), value)
	return unsafe.String((*byte)(arenaData), len(value))
}

// An Option used to configure an Arena.
type Option func(*Arena)

// WithLimit sets the maximum number of chunks that can be allocated.
func WithLimit(limit int) Option {
	return func(a *Arena) {
		a.limit = limit
	}
}

// Create a new Arena with the given chunk size in bytes.
//
// The chunk size is the increment by which the arena will allocate new memory.
// It is also the maximum size for a single object.
//
// Limit is the maximum number of chunks that can be allocated. A value of 0
// means there is no limit to the number of chunks that can be allocated.
func Create(chunkSize int, options ...Option) *Arena {
	current := make([]byte, chunkSize)
	a := &Arena{
		current:   current,
		chunkSize: int64(chunkSize),
		chunks:    [][]byte{current},
	}
	for _, option := range options {
		option(a)
	}
	return a
}

func (a *Arena) alloc(n int) unsafe.Pointer {
	next := a.cursor.Add(int64(n))
	if next < a.chunkSize {
		return unsafe.Pointer(&a.current[next-int64(n) : next][0])
	}
	return a.resize(n, next)
}

func (a *Arena) resize(n int, next int64) unsafe.Pointer {
	a.lock.Lock()
	defer a.lock.Unlock()
	if a.limit != 0 && int(a.chunkCursor) >= a.limit {
		panic(fmt.Sprintf("arena limit of %d chunks reached", a.limit))
	}
	// Another thread may have already expanded the arena.
	if !a.cursor.CompareAndSwap(next, int64(n)) {
		return a.alloc(n)
	}
	if a.chunkCursor < int64(len(a.chunks)-1) {
		a.current = a.chunks[a.chunkCursor]
	} else if len(a.chunks) < a.limit {
		a.current = make([]byte, a.chunkSize)
		a.chunks = append(a.chunks, a.current)
	}
	a.chunkCursor++
	next = int64(n)
	if next > a.chunkSize {
		panic(fmt.Sprintf("object size %d is larger than chunk size %d", n, a.chunkSize))
	}
	return unsafe.Pointer(&a.current[next-int64(n) : next][0])
}

// Reset the arena, zeroing all memory and resetting the cursor.
//
// Note that continuing to use any existing data allocated from the arena
// after a [Reset] will result in undefined behaviour.
//
// Also note that for large arenas this can be slow as the memory is zeroed. If this is a concern,
// just recreate the arena.
func (a *Arena) Reset() {
	a.lock.Lock()
	defer a.lock.Unlock()
	before := a.cursor.Load()
	// Zero the chunks.
	for _, chunk := range a.chunks {
		for i := range chunk {
			chunk[i] = 0
		}
	}
	a.current = a.chunks[0]
	a.chunks = [][]byte{a.current}
	a.chunkCursor = 0
	if !a.cursor.CompareAndSwap(before, 0) {
		panic("reset failed, another thread is using the arena")
	}
}
