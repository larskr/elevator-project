package network

import (
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
		IP:   Uint32ToIP(NetworkAddr()),
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
	var buf [MAX_PAYLOAD_SIZE]byte
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
		copy(msg.payload[:], buf[:])

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

		s.conn.WriteToUDP(msg.payload[:], &addr)
	}
}
