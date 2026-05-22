package syncx

import (
	"slices"
	"testing"
	"time"
)

func TestOrderedChannelBasicFIFO(t *testing.T) {
	in := make(chan int)
	out := OrderedChannel(in)

	go func() {
		for i := 1; i <= 5; i++ {
			in <- i
		}
		close(in)
	}()

	var got []int
	for v := range out {
		got = append(got, v)
	}

	want := []int{1, 2, 3, 4, 5}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestOrderedChannelEmptyInputClosesOut(t *testing.T) {
	in := make(chan int)
	close(in)
	out := OrderedChannel(in)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for v := range out {
			t.Errorf("got unexpected value %d from empty channel", v)
		}
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("out did not close after in closed")
	}
}

// TestOrderedChannelNoPhantomZeros — catches the phantom-zero bug: if `case out <- first:`
// is unconditionally in the select, an empty buffer's zero `first` keeps the send ready
// and leaks zeros to the consumer. Expect exactly [1, 2].
func TestOrderedChannelNoPhantomZeros(t *testing.T) {
	in := make(chan int)
	out := OrderedChannel(in)

	done := make(chan struct{})
	var got []int
	go func() {
		defer close(done)
		for v := range out {
			got = append(got, v)
		}
	}()

	in <- 1
	time.Sleep(50 * time.Millisecond) // window for buggy code to leak zeros
	in <- 2
	close(in)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for out to close")
	}

	want := []int{1, 2}
	if !slices.Equal(got, want) {
		preview := got
		if len(preview) > 10 {
			preview = preview[:10]
		}
		t.Errorf("got %d items %v..., want exactly %v — likely phantom zeros from empty-buffer send",
			len(got), preview, want)
	}
}

// TestOrderedChannelManyItemsInOrder — bulk interleaved send/receive;
// verifies order preservation and no extra elements.
func TestOrderedChannelManyItemsInOrder(t *testing.T) {
	const N = 10000
	in := make(chan int)
	out := OrderedChannel(in)

	go func() {
		for i := range N {
			in <- i
		}
		close(in)
	}()

	var got []int
	for v := range out {
		got = append(got, v)
	}

	if len(got) != N {
		t.Fatalf("got %d items, want %d", len(got), N)
	}
	for i, v := range got {
		if v != i {
			t.Errorf("position %d: got %d, want %d", i, v, i)
			return
		}
	}
}

// TestOrderedChannelProducerDoesNotBlock — unlimited buffer's core contract:
// producer sends all N items and closes `in` without blocking, even if consumer never reads.
func TestOrderedChannelProducerDoesNotBlock(t *testing.T) {
	const N = 100000
	in := make(chan int)
	out := OrderedChannel(in)

	producerDone := make(chan struct{})
	go func() {
		defer close(producerDone)
		for i := range N {
			in <- i
		}
		close(in)
	}()

	select {
	case <-producerDone:
	case <-time.After(5 * time.Second):
		t.Fatal("producer blocked — buffer should be unlimited")
	}

	count := 0
	for v := range out {
		if v != count {
			t.Errorf("position %d: got %d", count, v)
		}
		count++
	}
	if count != N {
		t.Errorf("got %d items, want %d", count, N)
	}
}
