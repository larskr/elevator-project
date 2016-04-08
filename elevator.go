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

	// This map is used as a set that holds the floor numbers which
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

func (e *Elevator) AddRequest(req Request) {
	e.queue <- req
}

func (e *Elevator) Requests() []Request {
	reqs := make([]Request, 0, maxRequests)
	for floor := 0; floor < elev.NumFloors; floor++ {
		if e.requests[floor][indexOfDir(elev.Up)] {
			reqs = append(reqs, Request{floor, elev.Up})
		} else if e.requests[floor][indexOfDir(elev.Down)] {
			reqs = append(reqs, Request{floor, elev.Down})
		}
	}
	return reqs
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
	elev.SetFloorIndicator(e.floor)
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

	// Is this floor a destination?
	if e.dest[e.floor] {
		elev.SetMotorDirection(elev.Stop)
		delete(e.dest, e.floor)

		// Update lamps on internal elevator panel.
		e.panel.SetLamp(elev.Command, e.floor, false)

		// We clear all requests (up and down) if we stop. This assumes
		// passengers will accept travelling in the wrong direction for a
		// while.
		e.clearRequest(e.floor, elev.Up)
		e.clearRequest(e.floor, elev.Down)

		return doorsOpen
	}

	// Is there a request at this floor in the direction we're going?
	if e.requests[e.floor][indexOfDir(e.direction)] {
		elev.SetMotorDirection(elev.Stop)

		e.clearRequest(e.floor, elev.Up)
		e.clearRequest(e.floor, elev.Down)

		return doorsOpen
	}

	// No more destinations, and no more requests in the direction we are going.
	if len(e.dest) == 0 && !e.hasWork() {
		elev.SetMotorDirection(elev.Stop)
		e.direction = elev.Stop
		return idle
	}

	// Fail safe; should never be true.
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

	// Check for request in diection of motion.
	if e.hasWork() {
		elev.SetMotorDirection(e.direction)
		return moving
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

			return gotoFloor
		}
	}

	time.Sleep(100 * time.Millisecond)

	return idle
}

// Checks if there are more requests in the current direction of motion.
func (e *Elevator) hasWork() bool {
	for floor := 0; floor < elev.NumFloors; floor++ {
		if e.requests[floor][indexOfDir(elev.Up)] || e.requests[floor][indexOfDir(elev.Down)] {
			if (e.direction == elev.Up && floor > e.floor) ||
				(e.direction == elev.Down && floor < e.floor) {
				return true
			}
		}
	}
	return false
}

// Clear requests and resets panel lamp.
func (e *Elevator) clearRequest(floor int, dir elev.Direction) {
	if (floor == 0 && dir == elev.Down) || (floor == elev.NumFloors-1 && dir == elev.Up) {
		return // invalid request
	}
	e.requests[floor][indexOfDir(dir)] = false
	e.panel.SetLamp(btnFromDir(dir), floor, false)
}



/////// COST PART //////////

type stateFn func(*Simulator) simStateFn

// Elevator holds the state of the elevator.
type Simulator struct {
	state       simStateFn
	floor       int
	direction   elev.Direction
	cost	    float64
	dest map[int]bool

	// Pending requests.
	virtualReq Request
	requests [elev.NumFloors][2]bool
}

func NewElevatorCost() *Simulator {
	e := &Simulator{
		direction:  elev.Stop,
		cost:	    0,
		dest:       make(map[int]bool),
	}
	return e
}

func (e *Simulator) AddCost(req Request) {
	e.requests[req.Floor][indexOfDir(req.Direction)] = true
}

func (e *Elevator) SimulateCost(req Request) float64 {
	//create virtual elevator used for simulating cost
	ve = NewElevator()
	//copy data from real elevator
	ve.direction 	= e.direction
	ve.dest 	= e.dest
	ve.state 	= e.state
	ve.floor 	= e.floor
	ve.Add(Request)
	ve.virtualReq = Request
	for ve.state != nil {
		ve.state = ve.state(ve);
	}
	return ve.cost
}

func movingCost(e *Simulator) simStateFn {

	if elev.ReadFloorSensor == -1 {
		e.floor = e.floor + sign(e.direction)
		cost = cost + 3
		return atFloorCost
	}

	e.floor = elev.ReadFloorSensor()
	return atFloorCost
}

func atFloorCost(e *Simulator) simStateFn {
	
	if e.dest[e.floor] {
		delete(e.dest, e.floor)

		if e.floor == e.virtualReq.floor {
			return nil
		}
		e.clearRequest(e.floor, elev.Up)
		e.clearRequest(e.floor, elev.Down)

		return doorsOpenCost
	}
	
	if e.requests[e.floor][indexOfDir(e.direction)] {

		if e.floor == e.virtualReq.floor && e.direction == e.virtualDest.direction {
			return nil
		}
		e.clearRequest(e.floor, elev.Up)
		e.clearRequest(e.floor, elev.Down)
		
		return doorsOpenCost
	}
	
	return movingCost
}

func doorsOpenCost(e *Simulator) simStateFn {

	cost = cost + 4
	return gotoFloorCost;
}

func gotoFloorCost(e *Simulator) simStateFn {

	if len(e.dest) > 0 {
		for d := range e.dest {

			if d > e.floor && e.direction == elev.Up {
				return movingCost
			} else if d < e.floor && e.direction == elev.Down {
				return movingCost
			}
		}

		if e.direction == elev.Up {
			e.direction = elev.Down
		} else if e.direction == elev.Down {
			e.direction = elev.Up
		}
		return gotoFloorCost
	}
	
	if e.hasWork() {
		return movingCost
	}
	return idleCost
}

func idleCost(e *Simulator) simStateFn {
	for floor := 0; floor < elev.NumFloors; floor++ {
		if e.requests[floor][indexOfDir(elev.Up)] || e.requests[floor][indexOfDir(elev.Down)] {
			if floor == e.floor{
				return = nil
			} else if floor > e.floor {
				e.direction = elev.Up
			} else {
				e.direction = elev.Down
			}
			
			e.dest[floor] = true
			return gotoFloorCost
		}
	}
	return idleCost
}

func (e *Simulator) hasWorkCost() bool {
	for floor := 0; floor < elev.NumFloors; floor++ {
		if e.requests[floor][indexOfDir(elev.Up)] || e.requests[floor][indexOfDir(elev.Down)] {
			if (e.direction == elev.Up && floor > e.floor) ||
				(e.direction == elev.Down && floor < e.floor) {
				return true
			}
		}
	}
	return false
}

func (e *Simulator) clearRequestCost(floor int, dir elev.Direction) {
	if (floor == 0 && dir == elev.Down) || (floor == elev.NumFloors-1 && dir == elev.Up) {
		return // invalid request
	}
	e.requests[floor][indexOfDir(dir)] = false
}



