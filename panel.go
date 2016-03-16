package main

import (
	"time"

	"elevator-project/elev"
)

const pollInterval = 25 * time.Millisecond


// Panel holds the state of the elevator panel.
type Panel struct {
	Requests chan Request
	Commands chan int     

	lamps  [elev.NumFloors][3]bool
}

func (p *Panel) Start() {
	p.Requests = make(chan Request, maxRequests)
	p.Commands = make(chan int)
	go p.poll()
}

func (p *Panel) Set(b elev.Button, floor int) {
	elev.SetButtonLamp(b, floor, 1)
	p.lamps[floor][b] = true
}

func (p *Panel) Reset(b elev.Button, floor int) {
	elev.SetButtonLamp(b, floor, 0)
	p.lamps[floor][b] = false
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
					p.lamps[floor][elev.CallUp] = true
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
					p.lamps[floor][elev.CallDown] = true
				}
			}
			prev[floor][elev.CallDown] = v

			v = elev.ReadButton(elev.Command, floor)
			if v != 0 && v != prev[floor][elev.Command] {
				if !p.lamps[floor][elev.Command] {
					select {
					case p.Commands <- floor:
						elev.SetButtonLamp(elev.Command, floor, 1)
						p.lamps[floor][elev.Command] = true
					default: // don't block
					}
				}
			}
			prev[floor][elev.Command] = v
		}
		time.Sleep(pollInterval)
	}
}
