package main

import (
	"encoding"
	"encoding/binary"
	"fmt"
	"math"
	"time"
	"errors"

	"elevator-project/pkg/elev"
	"elevator-project/pkg/network"
)

func packData(p []byte, data encoding.BinaryMarshaler) int {
	buf, _ := data.MarshalBinary()
	return copy(p, buf)
}

func unpackData(p []byte, data encoding.BinaryUnmarshaler) error {
	err := data.UnmarshalBinary(p)
	return err
}

// sendData sends a message with the data and returns the message ID.
// The data struct to be sent must be implmented in packData/unpackData.
func sendData(node *network.Node, mtype network.MsgType, data encoding.BinaryMarshaler) uint32 {
	buf, _ := data.MarshalBinary()
	msg := network.NewMessage(mtype, buf)
	node.SendMessage(msg)
	return msg.ID
}

const (
	COST   network.MsgType = 0x10
	ASSIGN network.MsgType = 0x11
	BACKUP network.MsgType = 0x12
)

type costData struct {
	elevator network.Addr
	req      Request
	cost     float64
}

type assignData struct {
	elevator network.Addr
	req      Request
	taken    bool
}

type backupData struct {
	elevator network.Addr
	created  time.Time

	floor     int
	direction elev.Direction
	requests  [elev.NumFloors][2]bool
	dest      [elev.NumFloors]bool
}

func (d costData) String() string {
	return fmt.Sprintf("(cost: %.1f, addr: %v, req.floor: %v, req.dir: %v)",
		d.cost, d.elevator, d.req.floor, d.req.direction)
}

func (d *costData) MarshalBinary() ([]byte, error) {
	p := make([]byte, 32)
	copy(p[:], d.elevator[:])
	binary.BigEndian.PutUint32(p[16:], uint32(d.req.floor))
	binary.BigEndian.PutUint32(p[20:], uint32(d.req.direction+1))
	binary.BigEndian.PutUint64(p[24:], math.Float64bits(d.cost))
	return p, nil
}

func (d *costData) UnmarshalBinary(p []byte) error {
	if len(p) != 32 {
		return errors.New("Cannot unmarshal costData")
	}
	copy(d.elevator[:], p[:])
	d.req.floor = int(binary.BigEndian.Uint32(p[16:]))
	d.req.direction = elev.Direction(int(binary.BigEndian.Uint32(p[20:])) - 1)
	d.cost = math.Float64frombits(binary.BigEndian.Uint64(p[24:]))
	return nil
}

func (d assignData) String() string {
	return fmt.Sprintf("(taken: %v, addr: %v, req.floor: %v, req.dir: %v)",
		d.taken, d.elevator, d.req.floor, d.req.direction)
}

func (d *assignData) MarshalBinary() ([]byte, error) {
	p := make([]byte, 28)
	copy(p[:], d.elevator[:])
	binary.BigEndian.PutUint32(p[16:], uint32(d.req.floor))
	binary.BigEndian.PutUint32(p[20:], uint32(d.req.direction+1))
	if d.taken {
		binary.BigEndian.PutUint32(p[24:], 1)
	} else {
		binary.BigEndian.PutUint32(p[24:], 0)
	}
	return p, nil
}

func (d *assignData) UnmarshalBinary(p []byte) error {
	if len(p) != 28 {
		return errors.New("Cannot unmarshal assignData")
	}
	copy(d.elevator[:], p[:])
	d.req.floor = int(binary.BigEndian.Uint32(p[16:]))
	d.req.direction = elev.Direction(int(binary.BigEndian.Uint32(p[20:])) - 1)
	if binary.BigEndian.Uint32(p[24:]) == 1 {
		d.taken = true
	}
	return nil
}

func (d backupData) String() string {
	return fmt.Sprintf("(addr: %v, reqs: %v, dest: %v)",
		d.elevator, d.requests, d.dest)
}

func (d *backupData) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 34+3*elev.NumFloors)
	p := buf

	copy(p, d.elevator[:])
	p = p[16:]

	timebuf, _ := d.created.MarshalBinary()
	copy(p[:15], timebuf)
	p = p[16:]

	p[0] = uint8(d.floor)
	switch d.direction {
	case elev.Down:
		p[1] = 255
	case elev.Up:
		p[1] = 1
	case elev.Stop:
		p[1] = 0
	}
	p = p[2:]

	for f := 0; f < elev.NumFloors; f++ {
		if d.requests[f][0] {
			p[0] = 1
		}
		if d.requests[f][1] {
			p[1] = 1
		}
		if d.dest[f] {
			p[2] = 1
		}
		p = p[3:]
	}

	return buf, nil
}

func (d *backupData) UnmarshalBinary(p []byte) error {
	if len(p) != 34+3*elev.NumFloors {
		return errors.New("Cannot unmarshal backupData")
	}
	
	copy(d.elevator[:], p)
	p = p[16:]

	d.created.UnmarshalBinary(p[:15])
	p = p[16:]

	d.floor = int(p[0])
	switch p[1] {
	case 255:
		d.direction = elev.Down
	case 0:
		d.direction = elev.Stop
	case 1:
		d.direction = elev.Up
	}
	p = p[2:]

	for f := 0; f < elev.NumFloors; f++ {
		d.requests[f][0] = (p[0] == 1)
		d.requests[f][1] = (p[1] == 1)
		d.dest[f] = (p[2] == 1)
		p = p[3:]
	}

	return nil
}
