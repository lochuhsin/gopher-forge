package channel

func UnorderedChannel[T any](in <-chan T) <-chan T {
	out := make(chan T)

	go func() {
		buf := make([]T, 0, 1024)

		for v := range in {
			select {
			case out <- v:
			default:
				buf = append(buf, v)
			}
		}

		for _, v := range buf {
			out <- v
		}
		close(out)
	}()

	return out
}

func OrderedChannel[T any](in <-chan T) <-chan T {
	out := make(chan T)
}
