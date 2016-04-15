package main

import (
	"elevator-project/pkg/elev"
)

const maxRequests = 2 * elev.NumFloors

type Request struct {
	floor     int
	direction elev.Direction
}

func (req Request) isValid() bool {
	if (req.floor == 0 && req.direction == elev.Down) ||
		(req.floor == elev.NumFloors-1 && req.direction == elev.Up) {
		return false
	}
	return true
}

// btnFromDir converts a elev.Direction to the corresponding elev.Button.
func btnFromDir(dir elev.Direction) elev.Button {
	if dir == elev.Up {
		return elev.CallUp
	}
	return elev.CallDown
}

// indexOfDir return the index of a elev.Direction.
func indexOfDir(dir elev.Direction) int {
	if dir == elev.Down {
		return 0
	}
	return 1
}
