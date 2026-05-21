package stack

type Stack interface {
	Pop() (int, bool)
	Push(v int)
}

type node struct {
	v    int
	next *node
}
