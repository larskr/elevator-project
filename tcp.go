package network

import (
	"errors"
	"fmt"
	"net"
	"sync"
)

type Frame struct {
	RemoteAddr string
	Data       []byte
}

var connectionList struct {
	m map[string]*net.TCPConn
	sync.RWMutex
}

func Init(port int, sendch, recvch chan Frame) error {
	connectionList.m = make(map[string]*net.TCPConn)

	go listenLoop(recvch, port)
	go transmitLoop(sendch)

	return nil
}

func listenLoop(recvch chan Frame, port int) {
	laddr, err := ResolveLocalAddr(port)
	if err != nil {
		panic(err)
	}

	listener, err := net.ListenTCP("tcp4", laddr)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Listening on port %v.\n", port)

	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			panic(err)
		}

		fmt.Printf("Accepted connection from %v.\n", conn.RemoteAddr())

		raddr := conn.RemoteAddr()
		connectionList.Lock()
		connectionList.m[raddr.String()] = conn
		connectionList.Unlock()

		go handleConnection(conn, recvch)
	}
}

func handleConnection(conn *net.TCPConn, recvch chan Frame) {
	buf := make([]byte, 1024)
	raddr := conn.RemoteAddr().String()
	for {
		n, err := conn.Read(buf)
		if err != nil {
			connectionList.Lock()
			conn.Close()
			delete(connectionList.m, raddr)
			connectionList.Unlock()
			return
		}

		fr := Frame{
			RemoteAddr: raddr,
			Data:       make([]byte, n),
		}
		copy(fr.Data, buf)

		recvch <- fr
	}
}

func transmitLoop(sendch chan Frame) {
	for {
		fr := <-sendch
		_, ok := connectionList.m[fr.RemoteAddr]
		if !ok {
			panic("Trying to send to host without connection.")
		}

		connectionList.RLock()
		conn := connectionList.m[fr.RemoteAddr]
		conn.Write(fr.Data)
		connectionList.RUnlock()
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
