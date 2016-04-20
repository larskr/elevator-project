package main

import (
	"time"

	"elevator-project/pkg/elev"
)

const maxSimulationSteps = 64

// stateFn represents the state of the elevator as a function that
// returns the next state.
type stateFn func(*Elevator) stateFn

// Elevator holds the state of the elevator.
type Elevator struct {
	state     stateFn
	floor     int
	direction elev.Direction

	stopped bool

	panel *Panel

	dest       [elev.NumFloors]bool
	destBuffer [elev.NumFloors]bool

	requests       [elev.NumFloors][2]bool
	requestsBuffer [elev.NumFloors][2]bool

	// simulator
	simulate   bool
	cost       float64
	virtualreq Request
}

func NewElevator(p *Panel) *Elevator {
	e := &Elevator{
		panel:     p,
		direction: elev.Stop,
	}
	return e
}

// Initializes elevator from backup. LoadBackup may be called with a
// empty backupData struct.
func (e *Elevator) LoadBackup(bd *backupData) {
	e.floor = bd.floor
	e.direction = bd.direction
	e.dest = bd.dest
	e.destBuffer = bd.dest
	//e.requests = bd.requests
	//e.requestsBuffer = bd.requests
}

func (e *Elevator) SimulateCost(req Request) float64 {
	//create virtual elevator used for simulating cost
	var ve *Elevator = new(Elevator)
	*ve = *e

	ve.simulate = true
	ve.requests[req.floor][indexOfDir(req.direction)] = true
	ve.virtualreq = req

	for i := 0; ve.state != nil && i < maxSimulationSteps; i++ {
		ve.state = ve.state(ve)
	}
	return ve.cost
}

func (e *Elevator) Start() {
	go e.run()
	go e.readPanel()
}

func (e *Elevator) IsRunning() bool {
	return !e.stopped
}

func (e *Elevator) AddRequest(req Request) {
	if req.isValid() {
		e.requestsBuffer[req.floor][indexOfDir(req.direction)] = true
	} else {
		errorlog.Println("Invalid request")
	}
}

func (e *Elevator) readPanel() {
	for {
		floor := <-e.panel.Commands
		e.destBuffer[floor] = true
	}
}

func (e *Elevator) run() {
	for e.state = start; e.state != nil; {

		e.requests = e.requestsBuffer
		e.dest = e.destBuffer

		// advance to next state
		e.state = e.state(e)
	}
}

func start(e *Elevator) stateFn {
	if f := elev.ReadFloorSensor(); f == -1 {
		if e.direction == elev.Stop {
			elev.SetMotorDirection(elev.Down)
			e.direction = elev.Down
		} else {
			elev.SetMotorDirection(e.direction)
		}
		return moving
	}
	e.floor = elev.ReadFloorSensor()
	elev.SetFloorIndicator(e.floor)
	return idle
}

func moving(e *Elevator) stateFn {
	if e.simulate {
		e.floor = e.floor + int(e.direction)
		e.cost += 3
		return atFloor
	}

	timeout := time.After(2 * time.Second)
	
	for elev.ReadFloorSensor() == -1 {
		select {
		case <-timeout:
			e.stopped = true
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
	e.stopped = false
	
	e.floor = elev.ReadFloorSensor()
	return atFloor
}

func atFloor(e *Elevator) stateFn {
	if !e.simulate {
		elev.SetFloorIndicator(e.floor)
	}

	// Is this floor a destination?
	if e.dest[e.floor] {
		if !e.simulate {
			elev.SetMotorDirection(elev.Stop)
		}
		e.dest[e.floor] = false
		e.destBuffer[e.floor] = false

		if e.simulate && e.floor == e.virtualreq.floor {
			return nil
		}

		// Update lamps on internal elevator panel.
		if !e.simulate {
			e.panel.SetLamp(elev.Command, e.floor, false)
		}

		// We clear all requests (up and down) if we stop. This assumes
		// passengers will accept travelling in the wrong direction for a
		// while.
		e.clearRequest(e.floor, elev.Up)
		e.clearRequest(e.floor, elev.Down)

		return doorsOpen
	}

	// Is there a request at this floor in the direction we're going?
	if e.requests[e.floor][indexOfDir(e.direction)] {
		if !e.simulate {
			elev.SetMotorDirection(elev.Stop)
		}

		if e.simulate && e.floor == e.virtualreq.floor {
			return nil
		}

		e.clearRequest(e.floor, elev.Up)
		e.clearRequest(e.floor, elev.Down)

		return doorsOpen
	}

	// No more destinations, and no more requests in the direction we are going.
	if !e.hasDest() && !e.hasWork() {
		if !e.simulate {
			elev.SetMotorDirection(elev.Stop)
		}
		e.direction = elev.Stop
		return idle
	}

	// Fail safe; should never be true.
	if (e.direction == elev.Up && e.floor == elev.NumFloors-1) ||
		(e.direction == elev.Down && e.floor == 0) {
		if !e.simulate {
			elev.SetMotorDirection(elev.Stop)
		}
		e.direction = elev.Stop
		return idle
	}

	// Wait untill floor is passed.
	if !e.simulate {
		timeout := time.After(1 * time.Second)
		
		for elev.ReadFloorSensor() == -1 {
			select {
			case <-timeout:
				e.stopped = true
			default:
				time.Sleep(100 * time.Millisecond)
			}
		}
		e.stopped = false
	}

	return moving
}

func doorsOpen(e *Elevator) stateFn {
	if e.simulate {
		if (e.floor < e.virtualreq.floor && e.direction == elev.Down) ||
			(e.floor > e.virtualreq.floor && e.direction == elev.Up) {
			e.cost = e.cost + 3 // fixed cost for internal commands
		}

		e.cost = e.cost + 4 // fixed cost for waiting until the doors close
		return gotoFloor
	}

	elev.SetDoorOpenLamp(1)
	defer elev.SetDoorOpenLamp(0)

	timeOut := time.After(3 * time.Second)
	<-timeOut
	return gotoFloor
}

func gotoFloor(e *Elevator) stateFn {
	// Assumption: motor is stopped when entering this function, but
	// e.direction holds the previous direction of motion.
	if e.hasDest() {
		for f := range e.dest {
			// Are there more destinations in the direction of motion?
			if e.dest[f] && f > e.floor && e.direction == elev.Up {
				if !e.simulate {
					elev.SetMotorDirection(elev.Up)
				}
				return moving
			} else if e.dest[f] && f < e.floor && e.direction == elev.Down {
				if !e.simulate {
					elev.SetMotorDirection(elev.Down)
				}
				return moving
			} else if e.dest[f] && f == e.floor {
				return atFloor
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
		if !e.simulate {
			elev.SetMotorDirection(e.direction)
		}
		return moving
	}

	// If we get to this point, there are no more destinations.
	if !e.simulate {
		elev.SetMotorDirection(elev.Stop)
	}
	e.direction = elev.Stop
	return idle
}

func idle(e *Elevator) stateFn {
	if e.hasDest() {
		e.direction = elev.Up
		return gotoFloor
	}

	for floor := 0; floor < elev.NumFloors; floor++ {
		if e.requests[floor][indexOfDir(elev.Up)] || e.requests[floor][indexOfDir(elev.Down)] {
			if e.simulate && floor == e.floor && floor == e.virtualreq.floor {
				return nil
			} else if floor == e.floor && e.requests[floor][indexOfDir(elev.Up)] {
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

	if !e.simulate {
		time.Sleep(25 * time.Millisecond)
	}

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

// Checks if there are more requests in the current direction of motion.
func (e *Elevator) hasDest() bool {
	for floor := 0; floor < elev.NumFloors; floor++ {
		if e.dest[floor] {
			return true
		}
	}
	return false
}

// Clear requests and resets panel lamp.
func (e *Elevator) clearRequest(floor int, dir elev.Direction) {
	req := Request{floor, dir}
	if !req.isValid() {
		return
	}

	e.requests[floor][indexOfDir(dir)] = false
	e.requestsBuffer[floor][indexOfDir(dir)] = false
	if !e.simulate {
		e.panel.SetLamp(btnFromDir(dir), floor, false)
	}
}
