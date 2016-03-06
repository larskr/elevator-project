package network

import (
	"encoding/binary"
	"net"
)

const (
	MAX_PAYLOAD_SIZE       = 256
	UDP_PORT               = 2048
	UDP_BUFFERED_CHAN_SIZE = 32
)

type UDPMessage struct {
	from    uint32
	to      uint32
	payload [MAX_PAYLOAD_SIZE]byte
}

type UDPService struct {
	conn     *net.UDPConn
	receivec chan *UDPMessage
	sendc    chan *UDPMessage
}

func NewUDPService() (*UDPService, error) {
	addr := net.UDPAddr{
		IP:   net.IPv4zero, // INADDR_ANY
		Port: UDP_PORT,
	}

	conn, err := net.ListenUDP("udp4", &addr)
	if err != nil {
		return nil, err
	}

	s := &UDPService{
		conn:     conn,
		receivec: make(chan *UDPMessage, UDP_BUFFERED_CHAN_SIZE),
		sendc:    make(chan *UDPMessage, UDP_BUFFERED_CHAN_SIZE),
	}

	go s.receiveLoop()
	go s.sendLoop()

	return s, nil
}

func (s *UDPService) Send(msg *UDPMessage) {
	s.sendc <- msg
}

func (s *UDPService) Receive() *UDPMessage {
	msg := <-s.receivec
	return msg
}

func (s *UDPService) receiveLoop() {
	var buf [MAX_PAYLOAD_SIZE + 8]byte
	for {
		n, _, err := s.conn.ReadFromUDP(buf[:])
		if n == 0 || err != nil {
			continue
		}

		umsg := new(UDPMessage)
		umsg.from = binary.BigEndian.Uint32(buf[:])
		umsg.to = binary.BigEndian.Uint32(buf[4:])
		copy(umsg.payload[:], buf[8:])

		s.receivec <- umsg
	}
}

func (s *UDPService) sendLoop() {
	var buf [MAX_PAYLOAD_SIZE + 8]byte
	for {
		umsg := <-s.sendc
		addr := net.UDPAddr{
			IP:   Uint32ToIP(umsg.to),
			Port: UDP_PORT,
		}

		binary.BigEndian.PutUint32(buf[:], umsg.from)
		binary.BigEndian.PutUint32(buf[4:], umsg.to)
		copy(buf[8:], umsg.payload[:])

		s.conn.WriteToUDP(buf[:], &addr)
	}
}
