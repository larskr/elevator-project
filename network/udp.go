package network

import (
	"fmt"
	"net"
)

const (
	maxPayloadLength = 256
	UDPPort          = 2048
	UDPBufferSize    = 32
)

type UDPMessage struct {
	from Addr
	to   Addr
	buf  [maxPayloadLength]byte

	payload []byte
}

type UDPService struct {
	conn     *net.UDPConn
	receivec chan *UDPMessage
	sendc    chan *UDPMessage

	addr Addr
}

func NewUDPService() (*UDPService, error) {
	addr := net.UDPAddr{
		IP:   net.IPv4zero,
		Port: UDPPort,
	}
	laddr := NetworkAddr()

	conn, err := net.ListenUDP("udp4", &addr)
	if err != nil {
		return nil, err
	}
	s := &UDPService{
		conn:     conn,
		receivec: make(chan *UDPMessage, UDPBufferSize),
		sendc:    make(chan *UDPMessage, UDPBufferSize),
		addr:     laddr,
	}

	go s.receiveLoop()
	go s.sendLoop()

	return s, nil
}

func (s *UDPService) Send(umsg *UDPMessage) {
	s.sendc <- umsg
}

func (s *UDPService) Receive() *UDPMessage {
	umsg := <-s.receivec
	return umsg
}

func (s *UDPService) receiveLoop() {
	for {
		umsg := new(UDPMessage)
		umsg.to = s.addr
		n, raddr, err := s.conn.ReadFromUDP(umsg.buf[:])
		if n == 0 || err != nil {
			continue
		}
		umsg.from = Addr(raddr.IP.To16())
		umsg.payload = umsg.buf[:n]
		s.receivec <- umsg
	}
}

func (s *UDPService) sendLoop() {
	for {
		umsg := <-s.sendc
		addr := net.UDPAddr{
			IP:   net.IP(umsg.to),
			Port: UDPPort,
		}
		s.conn.WriteToUDP(umsg.payload, &addr)
		fmt.Println("Sent.")
	}
}
