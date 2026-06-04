package peterson

import "runtime"

type Lock struct {
	flag [2]bool
	turn int
}

func (l *Lock) Lock(me int) {
	other := 1 - me
	l.flag[me] = true                      // (1) declare intent
	l.turn = other                         // (2) yield turn
	for l.flag[other] && l.turn == other { // (3) wait while other wants AND it's their turn
		runtime.Gosched()
	}
}

func (l *Lock) Unlock(me int) {
	l.flag[me] = false
}
