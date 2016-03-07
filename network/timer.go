package network

import (
	"time"
)

type SafeTimer struct {
	*time.Timer
	seen bool
}

func NewSafeTimer(d time.Duration) *SafeTimer {
	return &SafeTimer{Timer: time.NewTimer(d)}
}

func (t *SafeTimer) Seen() {
	t.seen = true
}

func (t *SafeTimer) SafeReset(d time.Duration) bool {
	ret := t.Stop()
	if !ret && !t.seen {
		<-t.C
	}
	t.Reset(d)
	t.seen = false
	return ret
}



