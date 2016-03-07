package network

import (
	"regexp"
	"fmt"
	"encoding/binary"
	"time"
)

// hexdump
func (m *UDPMessage) Hexdump() string {
	re := regexp.MustCompile("[^[:graph:] ]")
	buf := make([]byte, 256)
	buf = append(buf,
		[]byte(fmt.Sprintf("From: %v\nTo:   %v\nHexdump of data:\n",
			Uint32ToIP(m.from), Uint32ToIP(m.to)))...)
	for r := 0; r < 16; r++ {
		for i := 0; i < 16; i++ {
			buf = append(buf, []byte(fmt.Sprintf(" %02x ", m.payload[r*16+i]))...)
		}
		tmp := string(re.ReplaceAll(m.payload[r*16:(r+1)*16], []byte(".")))
		buf = append(buf, []byte(fmt.Sprintf(" | %v\n", tmp))...)
	}
	return string(buf)	
}

func (m *UDPMessage) String() string {
	buf := make([]byte, 256)
	buf = append(buf, []byte(fmt.Sprintf("%v | ", time.Now().Format("15:04:05")))...)
	buf = append(buf,
		[]byte(fmt.Sprintf("%v -> %v: ",
			Uint32ToIP(m.from), Uint32ToIP(m.to)))...)
	//for i := 0; i < 16; i++ {
	//	buf = append(buf, []byte(fmt.Sprintf("%02x ", m.payload[i]))...)
	//}
	//buf = append(buf, []byte(fmt.Sprintf("..."))...)

	msg := new(Message)
	msg.ID = binary.BigEndian.Uint32(m.payload[:])
	msg.Type = binary.BigEndian.Uint32(m.payload[4:])
	msg.ReadCount = binary.BigEndian.Uint32(m.payload[8:])
	copy(msg.Data[:], m.payload[16:])

	switch msg.Type {
	case BROADCAST:
		buf = append(buf, []byte(fmt.Sprintf("BROADCAST"))...)
	case HELLO:
		buf = append(buf, []byte(fmt.Sprintf("HELLO"))...)
	case ADD:
		buf = append(buf, []byte(fmt.Sprintf("ADD"))...)
	case ALIVE:
		buf = append(buf, []byte(fmt.Sprintf("ALIVE"))...)
	case KICK:
		buf = append(buf, []byte(fmt.Sprintf("KICK"))...)
	default:
		buf = append(buf, []byte(fmt.Sprintf("undefined"))...)
	}
	
	return string(buf)	
}
