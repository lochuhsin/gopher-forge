package peterson

import (
	"sync"
	"testing"
)

func TestPeterson(t *testing.T) {
	const N = 10_000_000

	var l Lock
	var counter int
	var wg sync.WaitGroup
	wg.Add(2)

	for me := range 2 {
		go func(me int) {
			defer wg.Done()
			for range N {
				l.Lock(me)
				counter++
				l.Unlock(me)
			}
		}(me)
	}
	wg.Wait()

	want := 2 * N
	if counter != want {
		t.Errorf("lost updates: got %d, want %d (lost %d)", counter, want, want-counter)
	}
}
