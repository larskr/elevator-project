package network

import (
	"encoding/binary"
)

type EncoderDecoder interface {
	Encode(p []byte) // encode struct as bytes into p
	Decode(p []byte) // decode bytes from p into struct
}

// type Message struct {
// 	ID        uint32
// 	Type      uint32
// 	ReadCount uint32
// 	_         uint32
// 	Data      [MAX_DATA_SIZE]byte
// }
func (m *Message) Encode(p []byte) {
	binary.BigEndian.PutUint32(p[:], m.ID)
	binary.BigEndian.PutUint32(p[4:], m.Type)
	binary.BigEndian.PutUint32(p[8:], m.ReadCount)
	copy(p[16:], m.Data[:])
}
func (m *Message) Decode(p []byte) {
	m.ID = binary.BigEndian.Uint32(p[:])
	m.Type = binary.BigEndian.Uint32(p[4:])
	m.ReadCount = binary.BigEndian.Uint32(p[8:])
	copy(m.Data[:], p[16:])
}

// type HelloData struct {
// 	newRight   uint32
// 	newLeft    uint32
//      newLeft2nd uint32
// }
func (hd *HelloData) Encode(p []byte) {
	binary.BigEndian.PutUint32(p[:], hd.newRight)
	binary.BigEndian.PutUint32(p[4:], hd.newLeft)
	binary.BigEndian.PutUint32(p[8:], hd.newLeft2nd)
}
func (hd *HelloData) Decode(p []byte) {
	hd.newRight = binary.BigEndian.Uint32(p[:])
	hd.newLeft = binary.BigEndian.Uint32(p[4:])
	hd.newLeft2nd = binary.BigEndian.Uint32(p[8:])
}

// type UpdateData struct {
// 	right   uint32
// 	left    uint32
//      left2nd uint32
// }
func (ud *UpdateData) Encode(p []byte) {
	binary.BigEndian.PutUint32(p[:], ud.right)
	binary.BigEndian.PutUint32(p[4:], ud.left)
	binary.BigEndian.PutUint32(p[8:], ud.left2nd)
}
func (ud *UpdateData) Decode(p []byte) {
	ud.right = binary.BigEndian.Uint32(p[:])
	ud.left = binary.BigEndian.Uint32(p[4:])
	ud.left2nd = binary.BigEndian.Uint32(p[8:])
}

// type KickData struct {
// 	deadNode   uint32
// 	senderNode uint32
// }
func (kd *KickData) Encode(p []byte) {
	binary.BigEndian.PutUint32(p[:], kd.deadNode)
	binary.BigEndian.PutUint32(p[4:], kd.senderNode)
}
func (kd *KickData) Decode(p []byte) {
	kd.deadNode = binary.BigEndian.Uint32(p[:])
	kd.senderNode = binary.BigEndian.Uint32(p[4:])
}
