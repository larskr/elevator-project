package main

import (
	"fmt"
	"sync"
	"time"

	"ring-network/elev"
)

// stateFn represents the state of the elevator as a function that
// returns the next state.
type stateFn func(*Elevator) stateFn

// Elevator holds the state of the elevator.
type Elevator struct {
	state     stateFn
	floor     int
	direction elev.Direction

	panel *Panel

	// This map is used as set that holds the floor numbers which
	// has been pressed on command panel inside the elevator.
	dest map[int]bool

	// Pending requests.
	requests [elev.NumFloors][2]int

	// Protects dest and requests which are updated from outside the FSM goroutine.
	mu sync.Mutex
}

func (e *Elevator) String() string {
	dests := ""
	for f := range e.dest {
		dests = fmt.Sprintf("%v, %v", dests, f)
	}
	reqs := ""
	str := fmt.Sprintf("floor: %v, direction: %v\ndest [%v]\n%v",
		e.floor, e.direction, dests, reqs)
	return str
}

func NewElevator(p *Panel) *Elevator {
	e := new(Elevator)
	e.panel = p
	e.direction = elev.Stop
	e.dest = make(map[int]bool)
	return e
}

func (e *Elevator) Start() {
	go e.run()
}

func (e *Elevator) Add(req Request) {
	e.mu.Lock()
	e.requests[req.Floor][req.Direction] = 1
	e.mu.Unlock()
}

func (e *Elevator) run() {
	for e.state = start; e.state != nil; {
		e.state = e.state(e)
	}
}

func start(e *Elevator) stateFn {
	fmt.Printf("start\n%v", e)
	
	if f := elev.ReadFloorSensor(); f == -1 {
		elev.SetMotorDirection(elev.Down)
		e.direction = elev.Down
		return moving
	}
	e.floor = elev.ReadFloorSensor()
	return idle
}

func moving(e *Elevator) stateFn {
	fmt.Printf("moving\n%v", e)
	
	for elev.ReadFloorSensor() == -1 {
		time.Sleep(50 * time.Millisecond)
	}
	e.floor = elev.ReadFloorSensor()
	return atFloor
}

func atFloor(e *Elevator) stateFn {
	fmt.Printf("atFloor\n%v", e)
	
	elev.SetFloorIndicator(e.floor)
	
	e.mu.Lock()
	// Is this floor a destination?
	// Is there a request at this floor in the direction we're going?
	if e.dest[e.floor] || e.requests[e.floor][e.direction] == 1 {
		elev.SetMotorDirection(elev.Stop)

		if e.dest[e.floor] {
			delete(e.dest, e.floor)
			e.panel.Clear(elev.Command, e.floor)

			// We clear all requests (up and down) if we stop. This assumes
			// passengers will accept travelling in the wrong direction for a
			// while.
			e.clear(e.floor, elev.Up)
			e.clear(e.floor, elev.Down)
		}

		// If we are only clearing requests in the direction of motion, we can
		// do the following:
		//                               (insert opposite dir here)    
		//                                            v
		// if len(e.dest) == 0 && e.request[e.floor][...] == 1 {
		//         e.clear(e.floor, ...)
		// }

		if e.requests[e.floor][e.direction] == 1 {
			e.clear(e.floor, e.direction)
		}
		
		e.mu.Unlock()
		return doorsOpen
	}
	e.mu.Unlock()
	
	if (e.direction == elev.Up && e.floor == elev.NumFloors-1) ||
		(e.direction == elev.Down && e.floor == 0) {
		elev.SetMotorDirection(elev.Stop)
		e.direction = elev.Stop
		return idle
	}
	
	// Wait untill floor is passed.
	for elev.ReadFloorSensor() != -1 {
		time.Sleep(50 * time.Millisecond)
	}

	return moving
}

func doorsOpen(e *Elevator) stateFn {
	fmt.Printf("doorsOpen\n%v", e)
	elev.SetDoorOpenLamp(1)
	defer elev.SetDoorOpenLamp(0)

	
	timeOut := time.After(3 * time.Second)
	for {
		select {
		case <-timeOut:
			return idle
		case floor := <-e.panel.Commands:
			e.mu.Lock()
			e.dest[floor] = true
			e.mu.Unlock()
			return leavingFloor
		}
	}
}

func leavingFloor(e *Elevator) stateFn {
	fmt.Printf("leavingFloor\n%v", e)
	
	// Assumption: motor is stopped when entering this function, but
	// e.direction holds the previous direction of motion.
	
	e.mu.Lock()
	defer e.mu.Unlock()
	
	if len(e.dest) > 0 {
		for d := range e.dest {
			switch {
			// Are there more destinations in the direction of motion?
			case d > e.floor && e.direction == elev.Up:
				elev.SetMotorDirection(elev.Up)
				return moving
			case d < e.floor && e.direction == elev.Down:
				elev.SetMotorDirection(elev.Down)
				return moving
			}
		}
		// No destinations in the direction of motion. Flip directions
		// and try again.
		switch {
		case e.direction == elev.Up:
			e.direction = elev.Down
		case e.direction == elev.Down:
			e.direction = elev.Up
		}
		return leavingFloor
	}

	// If we get to this point, there are no more destinations.
	return idle
}

func idle(e *Elevator) stateFn {
	fmt.Printf("idle\n%v", e)
	
	//if len(e.dest) > 0 {
	//	return leavingFloor
	//}
	
	e.direction = elev.Stop
	// Keep checking for new reqests. It's really not necessary to lock
	// the mutex where reading repeatedly in a loop like this. A faulty
	// read out does no harm.
	for {
		for floor := 0; floor < elev.NumFloors; floor++ {
			if e.requests[floor][elev.Up] == 1 ||
				e.requests[floor][elev.Down] == 1 {

				if floor == e.floor { // Request at this floor
					e.clear(floor, elev.Up)
					e.clear(floor, elev.Down)
					return doorsOpen
				} else if floor > e.floor {
					e.direction = elev.Up
				} else {
					e.direction = elev.Down
				}
				
				e.mu.Lock()
				e.dest[floor] = true
				e.mu.Unlock()

				return leavingFloor
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	
}

func (e *Elevator) clear(floor int, dir elev.Direction) {
	e.requests[floor][dir] = 0
	switch dir {
	case elev.Up:
		e.panel.Clear(elev.CallUp, floor)
	case elev.Down:
		e.panel.Clear(elev.CallDown, floor)
	}
}
