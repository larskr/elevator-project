package network

import (
	"fmt"
	"log"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	laddr, _ := ResolveLocalAddr(6000)
	log.Printf("This computer has local address: %v\n", laddr)

	p, _ := NewPeer(6000)

	for {
		fr := <-p.Receive
		log.Printf("Received from %v:\n%v", fr.Raddr, fr.Data)
	}

	fmt.Printf("\nConnections:\n")
	for ip, _ := range p.m {
		fmt.Println(ip)
	}

	os.Exit(0)
}

func TestResolveLocalAddr(t *testing.T) {
	laddr, err := ResolveLocalAddr(6000)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Printf("This computer has local address: %v\n", laddr)
}
