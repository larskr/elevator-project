package main

import (
	"math/rand"
	"time"

	"elevator-project/elev"
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
	requests [elev.NumFloors][2]bool
	queue    chan Request
}

func NewElevator(p *Panel) *Elevator {
	e := &Elevator{
		panel:     p,
		direction: elev.Stop,
		dest:      make(map[int]bool),
		queue:     make(chan Request, maxRequests),
	}
	return e
}

func (e *Elevator) Start() {
	go e.run()
}

func (e *Elevator) Add(req Request) {
	e.queue <- req
}

func (e *Elevator) SimulateCost(req Request) float64 {
	return rand.Float64()
}

func (e *Elevator) run() {
	for e.state = start; e.state != nil; {

	empty: // empty request queue
		for {
			select {
			case req := <-e.queue:
				e.requests[req.Floor][indexOfDir(req.Direction)] = true
			default:
				break empty
			}
		}

		// advance to next state
		e.state = e.state(e)
	}
}

func start(e *Elevator) stateFn {
	if f := elev.ReadFloorSensor(); f == -1 {
		elev.SetMotorDirection(elev.Down)
		e.direction = elev.Down
		return moving
	}
	e.floor = elev.ReadFloorSensor()
	return idle
}

func moving(e *Elevator) stateFn {
	for elev.ReadFloorSensor() == -1 {
		time.Sleep(100 * time.Millisecond)
	}
	e.floor = elev.ReadFloorSensor()
	return atFloor
}

func atFloor(e *Elevator) stateFn {
	elev.SetFloorIndicator(e.floor)

	if len(e.dest) == 0 {
		elev.SetMotorDirection(elev.Stop)
		e.direction = elev.Stop
		return idle
	}

	// Is this floor a destination?
	// Is there a request at this floor in the direction we're going?
	if e.dest[e.floor] || e.requests[e.floor][indexOfDir(e.direction)] {
		elev.SetMotorDirection(elev.Stop)

		if e.dest[e.floor] {
			delete(e.dest, e.floor)

			// Update lamps on internal elevator panel.
			e.panel.SetLamp(elev.Command, e.floor, false)

			// We clear all requests (up and down) if we stop. This assumes
			// passengers will accept travelling in the wrong direction for a
			// while.
			e.clearRequest(e.floor, elev.Up)
			e.clearRequest(e.floor, elev.Down)
		}

		// If we are only clearing requests in the direction of motion, we can
		// do the following:
		//                               (insert opposite dir here)
		//                                            v
		// if len(e.dest) == 0 && e.request[e.floor][...] {
		//         e.clearRequest(e.floor, ...)
		//         e.direction = ...
		// }

		if e.requests[e.floor][indexOfDir(e.direction)] {
			e.clearRequest(e.floor, e.direction)
		}

		return doorsOpen
	}

	if (e.direction == elev.Up && e.floor == elev.NumFloors-1) ||
		(e.direction == elev.Down && e.floor == 0) {
		elev.SetMotorDirection(elev.Stop)
		e.direction = elev.Stop
		return idle
	}

	// Wait untill floor is passed.
	for elev.ReadFloorSensor() != -1 {
		time.Sleep(100 * time.Millisecond)
	}

	return moving
}

func doorsOpen(e *Elevator) stateFn {
	elev.SetDoorOpenLamp(1)
	defer elev.SetDoorOpenLamp(0)

	timeOut := time.After(3 * time.Second)
	for {
		select {
		case <-timeOut:
			return gotoFloor
		case floor := <-e.panel.Commands:
			e.dest[floor] = true
			return gotoFloor
		}
	}
}

func gotoFloor(e *Elevator) stateFn {
	// Assumption: motor is stopped when entering this function, but
	// e.direction holds the previous direction of motion.
	if len(e.dest) > 0 {
		for d := range e.dest {
			// Are there more destinations in the direction of motion?
			if d > e.floor && e.direction == elev.Up {
				elev.SetMotorDirection(elev.Up)
				return moving
			} else if d < e.floor && e.direction == elev.Down {
				elev.SetMotorDirection(elev.Down)
				return moving
			}
		}
		// No destinations in the direction of motion. Flip directions
		// and try again.
		if e.direction == elev.Up {
			e.direction = elev.Down
		} else if e.direction == elev.Down {
			e.direction = elev.Up
		}
		return gotoFloor
	}

	// If we get to this point, there are no more destinations.
	elev.SetMotorDirection(elev.Stop)
	e.direction = elev.Stop
	return idle
}

func idle(e *Elevator) stateFn {
	for floor := 0; floor < elev.NumFloors; floor++ {
		if e.requests[floor][indexOfDir(elev.Up)] || e.requests[floor][indexOfDir(elev.Down)] {
			if floor == e.floor && e.requests[floor][indexOfDir(elev.Up)] {
				e.clearRequest(floor, elev.Up)
				e.clearRequest(floor, elev.Down) // clear both directions
				e.direction = elev.Up
				return doorsOpen
			} else if floor == e.floor && e.requests[floor][indexOfDir(elev.Down)] {
				e.clearRequest(floor, elev.Up) // clear both directions
				e.clearRequest(floor, elev.Down)
				e.direction = elev.Down
				return doorsOpen
			} else if floor > e.floor {
				e.direction = elev.Up
			} else {
				e.direction = elev.Down
			}

			e.dest[floor] = true
			return gotoFloor
		}
	}

	time.Sleep(100 * time.Millisecond)

	return idle
}

func (e *Elevator) clearRequest(floor int, dir elev.Direction) {
	if (floor == 0 && dir == elev.Down) || (floor == elev.NumFloors-1 && dir == elev.Up) {
		return // invalid request
	}
	e.requests[floor][indexOfDir(dir)] = false
	e.panel.SetLamp(btnFromDir(dir), floor, false)
}
