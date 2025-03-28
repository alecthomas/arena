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

// Cursor represents a pointer into the arena.
type Cursor uint64

func (c *Cursor) Load() (chunk, offset uint64) {
	v := atomic.LoadUint64((*uint64)(c))
	offset = v & 0xFFFFFFFF
	chunk = v >> 32
	return
}

func (c *Cursor) Add(size uint64) (chunk, offset uint64) {
	v := atomic.AddUint64((*uint64)(c), size)
	offset = v & 0xFFFFFFFF
	chunk = v >> 32
	return
}

// IncChunk moves the cursor to the start of the next chunk.
func (c *Cursor) IncChunk(chunk, offset uint64) bool {
	return atomic.CompareAndSwapUint64((*uint64)(c), chunk<<32|offset, (chunk+1)<<32)
}

func (c *Cursor) Reset(chunk, offset uint64) bool {
	return atomic.CompareAndSwapUint64((*uint64)(c), chunk<<32|offset, 0)
}

// Arena is a (mostly) lock-free memory allocator for a fixed-size type.
//
// "Mostly" lock-free because while individual allocations are lock-free we need to lock when expanding the arena.
type Arena struct {
	lock      sync.Mutex
	chunkSize uint64
	limit     int

	cursor Cursor
	chunks [][]byte
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
	return (*T)(arena.alloc(uint64(unsafe.Sizeof(t))))
}

// Value creates space for a new object in the arena and copies "value" into it.
//
// Note that this will _not_ perform a deep copy, so pointers, slices or maps will remain on the heap.
// Use [Clone] for that purpose.
//
// eg.
//
//	a := Value[Struct](arena, Struct{...})
//
// Typically value will be inlined and won't escape to the heap.
func Value[T any](arena *Arena, value T) *T {
	var t T
	out := (*T)(arena.alloc(uint64(unsafe.Sizeof(t))))
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
	out := unsafe.Slice((*T)(arena.alloc(uint64(int(unsafe.Sizeof(t))*cap))), cap) //nolint:gosec
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
	out := unsafe.Slice((*T)(arena.alloc(uint64(int(unsafe.Sizeof(t))*capacity))), capacity) //nolint:gosec
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
	arenaData := arena.alloc(uint64(len(value)))
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
func Create(chunkSize uint64, options ...Option) *Arena {
	if chunkSize > 0x7FFFFFFF {
		panic("chunk size too large")
	}
	a := &Arena{
		chunkSize: chunkSize,
		chunks:    [][]byte{make([]byte, chunkSize)},
	}
	for _, option := range options {
		option(a)
	}
	return a
}

func (a *Arena) alloc(n uint64) unsafe.Pointer {
	chunk, next := a.cursor.Add(n)
	if next < a.chunkSize {
		return unsafe.Pointer(&a.chunks[chunk][next-n : next][0])
	}
	return a.resize(chunk, next, n)
}

func (a *Arena) resize(chunk, cursor, n uint64) unsafe.Pointer {
	a.lock.Lock()                              // Note that we don't defer Unlock here because resize is called recursively
	if a.limit != 0 && int(chunk) >= a.limit { //nolint:gosec
		a.lock.Unlock()
		panic(fmt.Sprintf("arena limit of %d chunks reached", a.limit))
	}
	// Check that another thread hasn't already resized the arena.
	if actualChunk, actualCursor := a.cursor.Load(); actualChunk != chunk || actualCursor != cursor {
		a.lock.Unlock()
		return a.alloc(n)
	}

	// At this point we can't recurse, so we can defer the unlock.

	if chunk >= uint64(len(a.chunks)-1) && (a.limit == 0 || chunk+1 < uint64(a.limit)) { //nolint:gosec
		a.chunks = append(a.chunks, make([]byte, a.chunkSize))
	}
	if !a.cursor.IncChunk(chunk, cursor) {
		a.lock.Unlock()
		return a.alloc(n)
	}
	defer a.lock.Unlock()
	cursor = n
	if cursor > a.chunkSize {
		panic(fmt.Sprintf("object size %d is larger than chunk size %d", n, a.chunkSize))
	}
	return unsafe.Pointer(&a.chunks[chunk][cursor-n : cursor][0])
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
	beforeChunk, beforeCursor := a.cursor.Load()
	// Zero the chunks.
	for _, chunk := range a.chunks {
		for i := range chunk {
			chunk[i] = 0
		}
	}
	if !a.cursor.Reset(beforeChunk, beforeCursor) {
		panic("reset failed, another thread is using the arena")
	}
}
