package network

import (
	"fmt"
	"net"
	"errors"
	"io"
)

const (
	MAX_MESSAGE_SIZE = 256
	UDP_PORT         = 2048
)

type UDPMessage struct {
	from uint32
	to   uint32
	data [MAX_MESSAGE_SIZE]byte
}

func (umsg *UDPMessage) Write(p []byte) (n int, err error) {
	if len(p) > MAX_MESSAGE_SIZE {
		n = 0
		err = errors.New("Message data size is greater than MAX_MESSAGE_SIZE.")
		return
	}
	n = copy(umsg.data[:], p)
}

func (umsg *UDPMessage) Read(p []byte) (n int, err error) {
	if len(p) > MAX_MESSAGE_SIZE {
		err = io.EOF
	}
	n = copy(p, umsg.data[:])
}

func (umsg *UDPMessage)

type UDPService struct {
	conn    *net.UDPConn
	receivec chan *UDPMessage
	sendc    chan *UDPMessage
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
		ReceiveC: make(chan *UDPMessage),
		SendC:    make(chan *UDPMessage),
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
	var buf [MAX_MESSAGE_SIZE]byte
	for {
		n, raddr, err := s.conn.ReadFromUDP(buf[:])
		if n == 0 || err != nil {
			continue
		}

		laddr := s.conn.LocalAddr().(*net.UDPAddr)
		
		msg := &UDPMessage{
			from: IPToUint32(raddr.IP),
			to:   IPToUint32(laddr.IP),
		}
		copy(msg.data[:], buf[:])

		s.receivec <- msg

		for i := range buf {
			buf[i] = 0
		}
	}
}

func (s *UDPService) sendLoop() {
	for {
		msg := <-s.sendc
		addr := net.UDPAddr{
			IP:   Uint32ToIP(msg.to),
			Port: UDP_PORT,
		}

		fmt.Println(addr)

		s.conn.WriteToUDP(msg.data[:], &addr)
	}
}
