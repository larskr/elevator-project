package main

import (
	"elevator-project/elev"
)

const maxRequests = 2 * elev.NumFloors

type Request struct {
	Floor     int
	Direction elev.Direction
}

// btnFromDir converts a elev.Direction to the corresponding elev.Button.
func btnFromDir(dir elev.Direction) elev.Button {
	if dir == elev.Up {
		return elev.CallUp
	}
	return elev.CallDown
}

// indexOf return the index of a elev.Direction.
func indexOf(dir elev.Direction) int {
	if dir == elev.Down {
		return 0
	}
	return 1
}
