package syncx

import "unsafe"

type notifyList struct {
	wait   uint32
	notify uint32
	lock   uintptr
	head   unsafe.Pointer
	tail   unsafe.Pointer
}

//go:linkname notifyListAdd       sync.runtime_notifyListAdd
func notifyListAdd(l *notifyList) uint32

//go:linkname notifyListWait      sync.runtime_notifyListWait
func notifyListWait(l *notifyList, t uint32)

//go:linkname notifyListNotifyAll sync.runtime_notifyListNotifyAll
func notifyListNotifyAll(l *notifyList)
