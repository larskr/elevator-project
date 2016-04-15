package main

import (
	"elevator-project/pkg/elev"
)

type simStateFn func(*Simulator) simStateFn

// Elevator holds the state of the elevator.
type Simulator struct {
	state     simStateFn
	floor     int
	direction elev.Direction
	cost      float64
	dest      [elev.NumFloors]bool

	// Pending requests.
	virtualReq Request
	requests   [elev.NumFloors][2]bool
}

func (e *Elevator) SimulateCost(req Request) float64 {
	//create virtual elevator used for simulating cost
	ve := &Simulator{
		floor:     e.floor,
		direction: e.direction,
		cost:      0.0,
		requests:  e.requests,
		dest:      e.dest,
	}
	ve.requests[req.Floor][indexOfDir(req.Direction)] = true
	ve.virtualReq = req
	for ve.state = gotoFloorCost; ve.state != nil; {
		ve.state = ve.state(ve)
	}
	return ve.cost
}

func movingCost(e *Simulator) simStateFn {
	//fmt.Printf("movingCost: floor=%v  dir=%v\n", e.floor, e.direction)

	e.floor = e.floor + int(e.direction)
	e.cost += 3
	return atFloorCost
}

func atFloorCost(e *Simulator) simStateFn {
	//fmt.Printf("atFloorCost: floor=%v  dir=%v\n", e.floor, e.direction)

	if e.dest[e.floor] {
		e.dest[e.floor] = false

		if e.floor == e.virtualReq.Floor {
			return nil
		}
		e.clearRequest(e.floor, elev.Up)
		e.clearRequest(e.floor, elev.Down)

		return doorsOpenCost
	}

	if e.requests[e.floor][indexOfDir(e.direction)] {

		if e.floor == e.virtualReq.Floor {
			return nil
		}
		e.clearRequest(e.floor, elev.Up)
		e.clearRequest(e.floor, elev.Down)

		return doorsOpenCost
	}

	// No more destinations, and no more requests in the direction we are going.
	if !e.hasDest() && !e.hasWork() {
		e.direction = elev.Stop
		return idleCost
	}

	// Fail safe; should never be true.
	if (e.direction == elev.Up && e.floor == elev.NumFloors-1) ||
		(e.direction == elev.Down && e.floor == 0) {
		e.direction = elev.Stop
		return idleCost
	}

	return movingCost
}

func doorsOpenCost(e *Simulator) simStateFn {
	//fmt.Printf("doorsOpenCost: floor=%v  dir=%v\n", e.floor, e.direction)

	e.cost = e.cost + 4
	return gotoFloorCost
}

func gotoFloorCost(e *Simulator) simStateFn {
	//fmt.Printf("gotoFloorCost: floor=%v  dir=%v\n", e.floor, e.direction)

	if e.hasDest() {
		for f := range e.dest {

			if e.dest[f] && f > e.floor && e.direction == elev.Up {
				return movingCost
			} else if e.dest[f] && f < e.floor && e.direction == elev.Down {
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
	//fmt.Printf("idleCost: floor=%v  dir=%v\n", e.floor, e.direction)

	for floor := 0; floor < elev.NumFloors; floor++ {
		if e.requests[floor][indexOfDir(elev.Up)] || e.requests[floor][indexOfDir(elev.Down)] {
			if floor == e.floor && floor == e.virtualReq.Floor {
				return nil
			} else if floor > e.floor {
				e.direction = elev.Up
			} else {
				e.direction = elev.Down
			}

			return gotoFloorCost
		}
	}
	return idleCost
}

func (e *Simulator) hasWork() bool {
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

func (e *Simulator) hasDest() bool {
	for floor := 0; floor < elev.NumFloors; floor++ {
		if e.dest[floor] {
			return true
		}
	}
	return false
}

func (e *Simulator) clearRequest(floor int, dir elev.Direction) {
	if (floor == 0 && dir == elev.Down) || (floor == elev.NumFloors-1 && dir == elev.Up) {
		return // invalid request
	}
	e.requests[floor][indexOfDir(dir)] = false
}
