package arena

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
)

func FuzzArena(f *testing.F) {
	type User struct {
		Born   int
		Height int
	}
	f.Add(uint64(1024), uint64(10), uint64(10000))
	f.Fuzz(func(t *testing.T, size uint64, threads uint64, iterations uint64) {
		threads = (threads % 16) + 1
		size = max(min(size, 1024), 1024*1024*32)
		iterations = max(iterations, 1024)

		arena := Create(size)
		t.Log(threads)
		errs := make(chan error, threads)
		wg := sync.WaitGroup{}
		for range threads {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range iterations {
					actual := User{Born: rand.Int(), Height: rand.Int()} //nolint:gosec
					user := Value[User](arena, actual)
					if *user != actual {
						errs <- fmt.Errorf("%#v != %#v", *user, User{Born: rand.Int(), Height: rand.Int()}) //nolint:gosec
						return
					}
				}
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			t.Error(err.Error())
		}
	})
}
