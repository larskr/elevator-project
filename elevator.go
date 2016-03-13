package main

import (
	"time"

	"ring-network/elev"
)

// stateFn represents the state of the elevator as a function that
// returns the next state.
type stateFn func(*Elevator) stateFn

// Elevator holds the state of the elevator.
type Elevator struct {
	state     stateFn
	direction elev.Direction
}

func (e *Elevator) Start() {
	elev.Init()
	go e.run()
}

func (e *Elevator) run() {
	for e.state = start; e.state != nil; {
		e.state = e.state(e)
	}
}

func start(e *Elevator) stateFn {
	if f := elev.ReadFloorSensor(); f == -1 || f > 0 {
		e.setDirection(elev.Down)
		return moving
	}
	return idle
}

func moving(e *Elevator) stateFn {
	for elev.ReadFloorSensor() != 0 {
		time.Sleep(100 * time.Millisecond)
	}
	e.setDirection(elev.Stop)
	return idle
}

func idle(e *Elevator) stateFn {
	return nil
}

func (e * Elevator) setDirection(dir elev.Direction) {
	e.direction = dir
	elev.SetMotorDirection(dir)
}
