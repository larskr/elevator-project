package network

import "sync"

// Thread-safe queue built on top of the builtin slice type. Note that
// the operations aren't always O(1) since we have to grow the underlying
// slice, but this may be faster if the queue stays roughly constant in
// length.
type Queue struct {
	rep    []interface{}
	first  int
	last   int // -1 when empty
	length int
	sync.Mutex
	
}

func NewQueue() *Queue {
	q := &Queue{rep: make([]interface{}, 2), last: -1}
	return q
}

func (q *Queue) Add(v interface{}) {
	q.Lock()
	q.lazyGrow()
	q.last = q.inc(q.last)
	q.rep[q.last] = v
	q.length++
	q.Unlock()
}

func (q *Queue) Pop() interface{} {
	if q.length == 0 {
		return nil
	}
	q.Lock()
	v := q.rep[q.first]
	q.rep[q.first] = nil // for debug
	q.first = q.inc(q.first)
	q.length--
	q.Unlock()
	return v
}

func (q *Queue) inc(index int) int {
	return (index + 1) % len(q.rep)
}

func (q *Queue) lazyGrow() {
	// Only grow when the slice is full
	if q.length == len(q.rep) {
		rep := make([]interface{}, 2 * q.length)
		if q.first < q.last {
			// no wrap-around
			q.last = copy(rep, q.rep[q.first:(q.last + 1)]) - 1
		} else {
			n := copy(rep, q.rep[q.first:])
			q.last = copy(rep[n:], q.rep[:q.last+1]) + n - 1
		}
		q.first = 0
		q.rep = rep
	}
}
