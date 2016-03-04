package network

import (
	"net"
)

const (
	MAX_MESSAGE_SIZE = 256
	UDP_PORT         = 2048
)

type Message struct {
	From net.IP
	To   net.IP
	Data [MAX_MESSAGE_SIZE]byte
}

type UDPService struct {
	conn    *net.UDPConn
	addr   net.UDPAddr
	receive chan Message
	send    chan Message
}

func NewUDPService() (*UDPService, error) {
	addr := net.UDPAddr{
		IP:   net.ParseIP("192.168.1.13"),
		Port: UDP_PORT,
	}

	conn, err := net.ListenUDP("udp4", &addr)
	if err != nil {
		return nil, err
	}

	s := &UDPService{
		conn:    conn,
		addr:    addr,
		receive: make(chan Message),
		send:    make(chan Message),
	}

	go s.receiveLoop()
	go s.sendLoop()

	return s, nil
}

func (s *UDPService) receiveLoop() {
	var buf [MAX_MESSAGE_SIZE]byte
	for {
		n, raddr, err := s.conn.ReadFromUDP(buf[:])
		if n == 0 || err != nil {
			continue
		}

		msg := Message{
			From: raddr.IP,
			To:   s.addr.IP,
		}
		copy(msg.Data[:], buf[:])

		s.receive <- msg

		for i := range buf {
			buf[i] = 0
		}
	}
}

func (s *UDPService) sendLoop() {
	for {
		msg := <-s.send
		addr := net.UDPAddr{
			IP:   msg.To,
			Port: UDP_PORT,
		}

		s.conn.WriteToUDP(msg.Data[:], &addr)
	}
}
