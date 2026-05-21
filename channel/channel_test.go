package channel

import (
	"slices"
	"testing"
	"time"
)

func TestOrderedChannelBasicFIFO(t *testing.T) {
	in := make(chan int)
	out := OrderedChannel(in, nil)

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
	out := OrderedChannel(in, nil)

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

// TestOrderedChannelNoPhantomZeros — 抓 phantom-zero bug。
//
// 觸發條件：producer 送 1、停一陣子、送 2、close。期間 consumer 一直在 <-out 等。
// 如果 OrderedChannel 把 `case out <- first:` 無條件放進 select，當 buff 空時
// `first` 是零值，send case 仍然 ready → 會把零值送給 consumer。
//
// 預期：consumer 應該 exactly 收到 [1, 2]。
func TestOrderedChannelNoPhantomZeros(t *testing.T) {
	in := make(chan int)
	out := OrderedChannel(in, nil)

	done := make(chan struct{})
	var got []int
	go func() {
		defer close(done)
		for v := range out {
			got = append(got, v)
		}
	}()

	in <- 1
	time.Sleep(50 * time.Millisecond) // 留給 buggy code 噴零值的窗口
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

// TestOrderedChannelManyItemsInOrder — 大量資料 + 邊送邊收，
// 同時驗證「順序保留」跟「沒有多出來的元素」。
func TestOrderedChannelManyItemsInOrder(t *testing.T) {
	const N = 10000
	in := make(chan int)
	out := OrderedChannel(in, nil)

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

// TestOrderedChannelProducerDoesNotBlock — unlimited buffer 的核心契約：
// consumer 完全不讀的情況下，producer 仍然能把 N 筆全部送進去 + close in，
// 不會卡死。
func TestOrderedChannelProducerDoesNotBlock(t *testing.T) {
	const N = 100000
	in := make(chan int)
	out := OrderedChannel(in, nil)

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
