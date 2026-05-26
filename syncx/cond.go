package syncx

import "sync"

type MesaQueueCond struct{}

func (m *MesaQueueCond) Wait(lock *sync.Mutex) {
}

func (m *MesaQueueCond) Signal() {
}

func (m *MesaQueueCond) Broadcast() {
}

type MesaNotifyListCond struct{}

func (m *MesaNotifyListCond) Wait(lock *sync.Mutex) {
}

func (m *MesaNotifyListCond) Signal() {
}

func (m *MesaNotifyListCond) Broadcast() {
}
