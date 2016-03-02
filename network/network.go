package network

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
)

type peerConn struct {
	raddr  string
	conn   *net.TCPConn
	recvch chan string
	sendch chan string

	br *bufio.Reader

	parent *Peer
}

type Peer struct {
	port     int
	m        map[string]*peerConn
	addch    chan *peerConn
	removech chan *peerConn

	Receive chan Frame
	Send    chan Frame
}

type Frame struct {
	Raddr string
	Data  string
}

func NewPeer(port int) (*Peer, error) {
	p := &Peer{
		port:     port,
		m:        make(map[string]*peerConn),
		addch:    make(chan *peerConn),
		removech: make(chan *peerConn),
		Receive:  make(chan Frame),
		Send:     make(chan Frame),
	}

	go p.peerLoop()

	laddr, err := ResolveLocalAddr(port)
	if err != nil {
		return nil, err
	}

	listener, err := net.ListenTCP("tcp4", laddr)
	if err != nil {
		return nil, err
	}

	go p.listenerLoop(listener)

	log.Printf("Listening on port %v.\n", port)

	return p, nil
}

// Responsible for synchronising access to the connection list data
// structure. Runs in its own goroutine.
func (p *Peer) peerLoop() {
	for {
		select {
		case fr := <-p.Send:
			// lookup the peerConn and forward
			// TODO: Create connection if it doesn't exist
			pc, ok := p.m[fr.Raddr]
			if !ok {
				log.Printf("No connection to %v. Message dropped.\n", fr.Raddr)
				continue
			}
			pc.sendch <- fr.Data

		case pc := <-p.addch:
			p.m[pc.raddr] = pc
			log.Printf("Added connection to %v.\n", pc.raddr)
		case pc := <-p.removech:
			delete(p.m, pc.raddr)
		}
	}
}

// Listens for new connections, and starts goroutines to handle them.
func (p *Peer) listenerLoop(ln *net.TCPListener) {

	for {
		conn, err := ln.AcceptTCP()
		if err != nil {
			panic(err)
		}

		log.Printf("Accepted connection from %v.\n", conn.RemoteAddr())

		// TODO: check that this works correctly.
		pc := &peerConn{
			raddr:  conn.RemoteAddr().String(),
			conn:   conn,
			recvch: make(chan string),
			sendch: make(chan string),
			br:     bufio.NewReader(conn),
			parent: p,
		}
		p.addch <- pc

		go pc.readLoop()
		go pc.sendLoop()

		pc.sendch <- fmt.Sprintf("Hello from %v.\n", conn.LocalAddr())
	}
}

func (pc *peerConn) readLoop() {
	for {
		s, err := pc.br.ReadString('\n')
		if err != nil {
			pc.conn.Close()
			log.Printf("Connection from %v closed.\n", pc.raddr)
			pc.parent.removech <- pc
			return
		}

		pc.parent.Receive <- Frame{pc.raddr, s}
	}
}

func (pc *peerConn) sendLoop() {
	for {
		s := <-pc.sendch
		_, err := pc.conn.Write([]byte(s))
		if err != nil {
			log.Printf("Failed in sendLoop.\n")
			pc.parent.removech <- pc
		}

		log.Printf("Message sent to %v:\n%v", pc.raddr, s)
	}
}

// Finds the local address by searching through the network interfaces
// and returning the first IPv4 address that is not a loopback address.
func ResolveLocalAddr(port int) (*net.TCPAddr, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}

	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if ok && !ipnet.IP.IsLoopback() {
			if IPv4 := ipnet.IP.To4(); IPv4 != nil {
				return &net.TCPAddr{IP: IPv4, Port: port}, nil
			}
		}
	}

	return nil, errors.New("Unable to resolve local address.")
}
