package network

import (
	"encoding/binary"
	"errors"
)

var ErrShortBuffer = error.New("short buffer")

type EncoderDecoder interface {
	Encode(p []byte) (n int, err error) // encode struct as bytes into p
	Decode(p []byte) (n int, err error) // decode bytes from p into struct
}

// type Message struct {
// 	ID        uint32
// 	Type      uint32
// 	ReadCount uint32
// 	_         uint32
// 	Data      [MAX_DATA_SIZE]byte
// }
func (m *Message) Encode(p []byte) (n int, err error) {
	if len(p) < MAX_DATA_SIZE+12 {
		return 0, ErrShortBuffer
	}
	binary.BigEndian.PutUint32(p[:], m.ID)
	binary.BigEndian.PutUint32(p[4:], m.Type)
	binary.BigEndian.PutUint32(p[8:], m.ReadCount)
	n = copy(p[16:], m.Data[:]) + 12
	return
}
func (m *Message) Decode(p []byte) (n int, err error) {
	if len(p) < MAX_DATA_SIZE+12 {
		return 0, ErrShortBuffer
	}
	m.ID = binary.BigEndian.Uint32(p[:])
	m.Type = binary.BigEndian.Uint32(p[4:])
	m.ReadCount = binary.BigEndian.Uint32(p[8:])
	n = copy(m.Data[:], p[16:]) + 12
	return
}

// type HelloData struct {
// 	possibleNewRight uint32
// 	possibleNewLeft  uint32
// }
func (hd *HelloData) Encode(p []byte) (n int, err error) {
	if len(p) < 8 {
		return 0, ErrShortBuffer
	}
	binary.BigEndian.PutUint32(p[:], hd.possibleNewRight)
	binary.BigEndian.PutUint32(p[4:], hd.possibleNewLeft)
	return 8, nil
}
func (hd *HelloData) Decode(p []byte) (n int, err error) {
	if len(p) < 8 {
		return 0, ErrShortBuffer
	}
	hd.possibleNewRight = binary.BigEndian.Uint32(p[:])
	hd.possibleNewRight = binary.BigEndian.Uint32(p[4:])
	return 8, nil
}

// type AddData struct {
// 	asRight uint32
// 	asLeft  uint32
// }
func (ad *AddData) Encode(p []byte) (n int, err error) {
	if len(p) < 8 {
		return 0, ErrShortBuffer
	}
	binary.BigEndian.PutUint32(p[:], ad.asRight)
	binary.BigEndian.PutUint32(p[4:], ad.asLeft)
	return 8, nil
}
func (ad *AddData) Decode(p []byte) (n int, err error) {
	if len(p) < 8 {
		return 0, ErrShortBuffer
	}
	ad.asRight = binary.BigEndian.Uint32(p[:])
	ad.asLeft = binary.BigEndian.Uint32(p[4:])
	return 8, nil
}

// type KickData struct {
// 	deadNode   uint32
// 	senderNode uint32
// }
func (kd *KickData) Encode(p []byte) (n int, err error) {
	if len(p) < 8 {
		return 0, ErrShortBuffer
	}
	binary.BigEndian.PutUint32(p[:], kd.deadNode)
	binary.BigEndian.PutUint32(p[4:], kd.senderNode)
	return 8, nil
}
func (kd *KickData) Decode(p []byte) (n int, err error) {
	if len(p) < 8 {
		return 0, ErrShortBuffer
	}
	kd.deadNode = binary.BigEndian.Uint32(p[:])
	kd.senderNode = binary.BigEndian.Uint32(p[4:])
	return 8, nil
}
