package syncx

import (
	"sync/atomic"
)

type Transaction interface{}

type TVar[T any] struct {
	v       T
	version atomic.Int64
}

func NewTVar[T any](v T) *TVar[T] {
	return &TVar[T]{v: v}
}

// func (tv *TVar[T]) Read(tx Transaction) T {
// }

// func (tv *TVar[T]) Write(tx Transaction, v T) T {
// }
