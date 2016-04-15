package main

import (
	"time"
)

// A more predictable timer than time.Timer.
type Timer struct {
	deadline time.Time
	stopped  bool
}

// Reset returns true if the timer has not timed out, and false if it
// has timed out or been stopped.
func (t *Timer) Reset(d time.Duration) bool {
	ret := time.Now().Before(t.deadline) && !t.stopped
	t.stopped = false
	t.deadline = time.Now().Add(d)
	return ret
}

func (t *Timer) Stop() bool {
	t.stopped = true
	return time.Now().Before(t.deadline)
}

func (t *Timer) HasTimedOut() bool {
	return time.Now().After(t.deadline) && !t.stopped
}
