package stack

type Stack[T any] interface {
	Pop() (T, bool)
	Push(v T)
}

type node[T any] struct {
	v    T
	next *node[T]
}
