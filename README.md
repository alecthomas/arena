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
cpu: Apple M4
BenchmarkArena/100-10          	 6332707	       188.5 ns/op	    6061 B/op	       0 allocs/op
BenchmarkArena/1000-10         	  652868	      1868 ns/op	    2158 B/op	       0 allocs/op
BenchmarkArena/10000-10        	   64929	     18521 ns/op	       0 B/op	       0 allocs/op
BenchmarkArena/100000-10       	    6398	    187039 ns/op	       0 B/op	       0 allocs/op
BenchmarkArena/1000000-10      	     615	   1941853 ns/op	       0 B/op	       0 allocs/op
BenchmarkGoRuntime/100-10      	  342405	      3567 ns/op	    8000 B/op	     100 allocs/op
BenchmarkGoRuntime/1000-10     	   32731	     35403 ns/op	   80000 B/op	    1000 allocs/op
BenchmarkGoRuntime/10000-10    	    2656	    437390 ns/op	  800007 B/op	   10000 allocs/op
BenchmarkGoRuntime/100000-10   	     457	   2566789 ns/op	 8000035 B/op	  100000 allocs/op
BenchmarkGoRuntime/1000000-10  	      60	  19900749 ns/op	80000014 B/op	 1000000 allocs/op
BenchmarkArenaAppend/100-10    	 1000000	      3158 ns/op	   17179 B/op	       0 allocs/op
BenchmarkArenaAppend/1000-10   	   47565	     28922 ns/op	       0 B/op	       0 allocs/op
BenchmarkArenaAppend/10000-10  	    3813	    361884 ns/op	       0 B/op	       0 allocs/op
BenchmarkGoRuntimeAppend/100-10         	  194024	      6184 ns/op	   18944 B/op	       3 allocs/op
BenchmarkGoRuntimeAppend/1000-10        	   24258	     50690 ns/op	  171777 B/op	       6 allocs/op
BenchmarkGoRuntimeAppend/10000-10       	    1341	    900364 ns/op	 3391268 B/op	      14 allocs/op
BenchmarkReset/100-10                   	    1928	    790078 ns/op	       0 B/op	       0 allocs/op
BenchmarkReset/1000-10                  	    2125	   4050401 ns/op	       0 B/op	       0 allocs/op
BenchmarkReset/10000-10                 	     366	  10557410 ns/op	       0 B/op	       0 allocs/op
BenchmarkConcurrent/100-10              	  260062	      4850 ns/op	    6838 B/op	       0 allocs/op
BenchmarkConcurrent/1000-10             	   26149	     43108 ns/op	    1283 B/op	       0 allocs/op
BenchmarkConcurrent/10000-10            	    3010	    430277 ns/op	   89182 B/op	       0 allocs/op
```
