package main

import (
	"time"

	"elevator-project/elev"
)

const pollInterval = 25 * time.Millisecond

type Request struct {
	Floor     int
	Direction elev.Direction
}

type Panel struct {
	Requests chan Request
	Commands chan int

	lamps  [elev.NumFloors][3]bool
}

func (p *Panel) Start() {
	p.Requests = make(chan Request, 2*elev.NumFloors)
	p.Commands = make(chan int)
	go p.poll()
}

func (p *Panel) Reset(b elev.Button, floor int) {
	p.lamps[floor][b] = false
	elev.SetButtonLamp(b, floor, 0)
}

func (p *Panel) poll() {
	var prev [elev.NumFloors][3]int

	for {
		for floor := 0; floor < elev.NumFloors; floor++ {
			v := elev.ReadButton(elev.CallUp, floor)
			if v != 0 && prev[floor][elev.CallUp] == 0 {
				if !p.lamps[floor][elev.CallUp] {
					p.Requests <- Request{
						Floor:     floor,
						Direction: elev.Up,
					}
					elev.SetButtonLamp(elev.CallUp, floor, 1)
					p.lamps[floor][elev.CallUp].on = true
				}
			}
			prev[floor][elev.CallUp] = v

			v = elev.ReadButton(elev.CallDown, floor)
			if v != 0 && v != prev[floor][elev.CallDown] {
				if !p.lamps[floor][elev.CallDown] {
					p.Requests <- Request{
						Floor:     floor,
						Direction: elev.Down,
					}
					elev.SetButtonLamp(elev.CallDown, floor, 1)
					p.lamps[floor][elev.CallDown].on = true
				}
			}
			prev[floor][elev.CallDown] = v

			v = elev.ReadButton(elev.Command, floor)
			if v != 0 && v != prev[floor][elev.Command] {
				if !p.lamps[floor][elev.Command] {
					select {
					case p.Commands <- floor:
						elev.SetButtonLamp(elev.Command, floor, 1)
						p.lamps[floor][elev.Command].on = true
					default: // don't block
					}
				}
			}
			prev[floor][elev.Command] = v
		}
		time.Sleep(pollInterval)
	}
}
