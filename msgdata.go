package main

import (
	"encoding"
	"encoding/binary"
	"math"
	"time"

	"elevator-project/elev"
	"elevator-project/network"
)

func packData(p []byte, data encoding.BinaryMarshaler) int {
	buf, _ := data.MarshalBinary()
	return copy(p, buf)
}

func unpackData(p []byte, data encoding.BinaryUnmarshaler) {
	data.UnmarshalBinary(p)
}

// sendData sends a message with the data and returns the message ID.
// The data struct to be sent must be implmented in packData/unpackData.
func sendData(node *network.Node, mtype network.MsgType, data encoding.BinaryMarshaler) uint32 {
	var buf [network.MaxDataLength]byte

	databuf, _ := data.MarshalBinary()
	n := copy(buf[:], databuf)
	msg := network.NewMessage(mtype, buf[:n])

	node.SendMessage(msg)
	return msg.ID
}

const (
	PANEL  network.MsgType = 0x10
	COST   network.MsgType = 0x11
	ASSIGN network.MsgType = 0x12
	BACKUP network.MsgType = 0x13
)

type panelData struct {
	floor  int
	button elev.Button
	on     bool
}

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
	requests []Request
	dest     [elev.NumFloors]bool
}

func (d *panelData) MarshalBinary() ([]byte, error) {
	p := make([]byte, 12)
	binary.BigEndian.PutUint32(p[:], uint32(d.floor))
	binary.BigEndian.PutUint32(p[4:], uint32(d.button))
	if d.on {
		binary.BigEndian.PutUint32(p[8:], 1)
	} else {
		binary.BigEndian.PutUint32(p[8:], 0)
	}
	return p, nil
}

func (d *panelData) UnmarshalBinary(p []byte) error {
	d.floor = int(binary.BigEndian.Uint32(p[:]))
	d.button = elev.Button(binary.BigEndian.Uint32(p[4:]))
	if binary.BigEndian.Uint32(p[8:]) == 1 {
		d.on = true
	}
	return nil
}

func (d *costData) MarshalBinary() ([]byte, error) {
	p := make([]byte, 32)
	copy(p[:], d.elevator[:])
	binary.BigEndian.PutUint32(p[16:], uint32(d.req.Floor))
	binary.BigEndian.PutUint32(p[20:], uint32(d.req.Direction))
	binary.BigEndian.PutUint64(p[24:], math.Float64bits(d.cost))
	return p, nil
}

func (d *costData) UnmarshalBinary(p []byte) error {
	copy(d.elevator[:], p[:])
	d.req.Floor = int(binary.BigEndian.Uint32(p[16:]))
	d.req.Direction = elev.Direction(binary.BigEndian.Uint32(p[20:]))
	d.cost = math.Float64frombits(binary.BigEndian.Uint64(p[24:]))
	return nil
}

func (d *assignData) MarshalBinary() ([]byte, error) {
	p := make([]byte, 28)
	copy(p[:], d.elevator[:])
	binary.BigEndian.PutUint32(p[16:], uint32(d.req.Floor))
	binary.BigEndian.PutUint32(p[20:], uint32(d.req.Direction))
	if d.taken {
		binary.BigEndian.PutUint32(p[24:], 1)
	} else {
		binary.BigEndian.PutUint32(p[24:], 0)
	}
	return p, nil
}

func (d *assignData) UnmarshalBinary(p []byte) error {
	copy(d.elevator[:], p[:])
	d.req.Floor = int(binary.BigEndian.Uint32(p[16:]))
	d.req.Direction = elev.Direction(binary.BigEndian.Uint32(p[20:]))
	if binary.BigEndian.Uint32(p[24:]) == 1 {
		d.taken = true
	}
	return nil
}

func (d *backupData) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 40+2*maxRequests)
	p := buf
	copy(p, d.elevator[:])
	p = p[16:]
	timebuf, _ := d.created.MarshalBinary()
	nc := len(timebuf)
	p[0] = uint8(nc)
	copy(p[1:], timebuf)
	p = p[nc+1:]
	nreq := len(d.requests)
	p[0] = uint8(nreq)
	p = p[1:]
	for i := 0; i < nreq; i++ {
		p[0] = uint8(d.requests[i].Floor)
		if d.requests[i].Direction == elev.Down {
			p[1] = uint8(255)
		} else {
			p[1] = uint8(d.requests[i].Direction)
		}
		p = p[2:]
	}
	n := 16 + 1 + nc + 1 + nreq*2
	return buf[:n], nil
}

func (d *backupData) UnmarshalBinary(p []byte) error {
	copy(d.elevator[:], p)
	p = p[16:]
	ntime := int(p[0])
	p = p[1:]
	d.created.UnmarshalBinary(p[:ntime])
	p = p[ntime:]
	nreq := int(p[0])
	p = p[1:]
	d.requests = make([]Request, nreq)
	for i := 0; i < nreq; i++ {
		d.requests[i].Floor = int(p[0])
		dir := p[1]
		if dir == 255 {
			d.requests[i].Direction = elev.Down
		} else {
			d.requests[i].Direction = elev.Direction(dir)
		}
	}
	return nil
}
