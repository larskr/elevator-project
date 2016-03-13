package network

import (
	"time"
)

// SafeTimer behaves differently from time.Timer because it will
// clear the channel when reset.
type SafeTimer struct {
	*time.Timer
	seen bool
}

// Allocates and starts a new SafeTimer.
func NewSafeTimer(d time.Duration) *SafeTimer {
	return &SafeTimer{Timer: time.NewTimer(d)}
}

// Seen must be called after receiving from the channel. If this is
// not done SafeReset will deadlock.
func (t *SafeTimer) Seen() {
	t.seen = true
}

// SafeReset clears the channel and resets the timer.
func (t *SafeTimer) SafeReset(d time.Duration) bool {
	ret := t.Stop()
	if !ret && !t.seen {
		<-t.C
	}
	t.Reset(d)
	t.seen = false
	return ret
}



