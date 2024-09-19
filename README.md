# A very fast arena allocator for Go

[![Golang Documentation](https://godoc.org/github.com/alecthomas/arena?status.svg)](https://godoc.org/github.com/alecthomas/arena) [![CI](https://github.com/alecthomas/arena/actions/workflows/ci.yml/badge.svg)](https://github.com/alecthomas/arena/actions/workflows/ci.yml) [![Slack chat](https://img.shields.io/static/v1?logo=slack&style=flat&label=slack&color=green&message=gophers)](https://invite.slack.golangbridge.org/)

This package provides a very fast _almost_ lock-free arena allocator for Go. "Almost" lock-free because while individual
allocations are a single atomic add, a lock is acquired when expanding the arena after a chunk has been exhausted.

> [!WARNING]
> Note that continuing to use any existing data allocated from the arena
> after a `Reset()` will result in undefined behaviour.

## Usage

```go
alloc := arena.Create(32 * 1024 * 1024) // 32MB chunk size
m := arena.New[MyStruct](alloc)
s := arena.Slice[MyStruct](alloc, 0, 100)
s = arena.Append(alloc, s, MyStruct{})

// Once done, reset the arena to reuse the memory.
alloc.Reset()
```

## Performance

For allocating individual objects, the arena allocator is about an order of magnitude faster
than the Go runtime and, of course, has zero allocations and minimal GC overhead.

```
~/dev/arena $ go test -benchmem -bench .
goos: darwin
goarch: arm64
pkg: github.com/alecthomas/arena
cpu: Apple M3
BenchmarkArena/100-8               6155918         197 ns/op         0 B/op         0 allocs/op
BenchmarkArena/1000-8               599896        1976 ns/op         0 B/op         0 allocs/op
BenchmarkArena/10000-8               61201       19957 ns/op         0 B/op         0 allocs/op
BenchmarkArena/100000-8               6133      198741 ns/op         0 B/op         0 allocs/op
BenchmarkArena/1000000-8               588     2025277 ns/op         0 B/op         0 allocs/op
BenchmarkGoRuntime/100-8            641896        1809 ns/op      8000 B/op       100 allocs/op
BenchmarkGoRuntime/1000-8            66020       18014 ns/op     80000 B/op      1000 allocs/op
BenchmarkGoRuntime/10000-8            5764      209803 ns/op    800011 B/op     10000 allocs/op
BenchmarkGoRuntime/100000-8            560     2114053 ns/op   8000037 B/op    100000 allocs/op
BenchmarkGoRuntime/1000000-8            56    21004230 ns/op  80000029 B/op   1000000 allocs/op
PASS
ok    github.com/alecthomas/arena  14.069s
```
