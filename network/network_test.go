package network

import (
	"fmt"
	"testing"
)

func TestMain(m *testing.M) {
	laddr, _ := ResolveLocalAddr(6000)
	fmt.Printf("This computer has local address: %v\n", laddr)

	sendch, recvch := make(chan Frame), make(chan Frame)
	Init(6000, sendch, recvch)

	cmdch := make(chan Frame)
	for {
		select {
		case fr := <-recvch:
			fmt.Println(fr.Data)

			responsefr := Frame{
				RemoteAddr: fr.RemoteAddr,
				Data:       []byte(fmt.Sprintf("Received: %v.\n", string(fr.Data))),
			}
			sendch <- responsefr
		}

	}

}

func TestResolveLocalAddr(t *testing.T) {
	laddr, err := ResolveLocalAddr(6000)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Printf("This computer has local address: %v\n", laddr)
}
