package network

import (
	"testing"
	"time"
	"math/rand"
)

func TestSafeTimer(test *testing.T) {
	const timeout = 1 * time.Millisecond
	t := NewSafeTimer(timeout)
	for i := 0; i < 100; i++ {
		// t2 in [0.5,1.5)*timeout
		t2 := timeout + time.Duration(rand.Intn(100)-50)*timeout/100
		start := time.Now()
		select {
		case <-time.After(t2):
		case <-t.C:
			t.Seen()
		}
		elapsed := time.Since(start)
		
		if elapsed < timeout/4 {
			test.Errorf("short sleep: %v", elapsed)
		}
		time.Sleep(10 * timeout)  // make sure that t has expired
		t.SafeReset(timeout)
	}
}

	
