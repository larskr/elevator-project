package network

import (
	"errors"
	"fmt"
	"net"
)

// TODO: Do we stil need this?
// type Frame struct {
// 	RemoteAddr string
// 	Data       []byte
// }

type clientConn struct {
	conn   *net.TCPConn
	recvch chan string
	sendch chan string
}

var connectionList struct {
	m     map[string]*clientConn
	addch chan *clientConn
	rmch  chan *clientConn
}

func Init(port int) error {
	connectionList.m = make(map[string]*clientConn)
	connectionList.addch = make(chan *clientConn)
	connectionList.rmch = make(chan *clientConn)

	laddr, err := ResolveLocalAddr(port)
	if err != nil {
		return err
	}

	listener, err := net.ListenTCP("tcp4", laddr)
	if err != nil {
		return err
	}

	go listenerLoop(listener)
	
	fmt.Printf("Listening on port %v.\n", port)
	
	return nil
}

func listenerLoop(ln *net.TCPListener) {
	
	for {
		conn, err := ln.AcceptTCP()
		if err != nil {
			panic(err)
		}

		fmt.Printf("Accepted connection from %v.\n", conn.RemoteAddr())

		// TODO: check that this works correctly.
		cc := &clientConn{
			conn: conn,
			recvch: make(chan string),
			sendch: make(chan string),
		}
		connectionList.addch <- cc
		
		//go cc.readLoop()
		//go cc.sendLoop()
	}
}

func (cc *clientConn) readLoop() {
	// buf := make([]byte, 1024)
	// raddr := conn.RemoteAddr().String()
	// for {
	// 	n, err := conn.Read(buf)
	// 	if err != nil {
	// 		connectionList.Lock()
	// 		conn.Close()
	// 		delete(connectionList.m, raddr)
	// 		connectionList.Unlock()
	// 		return
	// 	}
	// 
	// 	fr := Frame{
	// 		RemoteAddr: raddr,
	// 		Data:       make([]byte, n),
	// 	}
	// 	copy(fr.Data, buf)
	// 
	// 	recvch <- fr
	// }
}

func (cc *clientConn) sendLoop() {
	// for {
	// 	fr := <-sendch
	// 	_, ok := connectionList.m[fr.RemoteAddr]
	// 	if !ok {
	// 		panic("Trying to send to host without connection.")
	// 	}
	// 
	// 	connectionList.RLock()
	// 	conn := connectionList.m[fr.RemoteAddr]
	// 	conn.Write(fr.Data)
	// 	connectionList.RUnlock()
	// }
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
