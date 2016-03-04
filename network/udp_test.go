package network

import (
	"../utils"
	"fmt"
	"net"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	fmt.Printf("NetworkAddr: %v\nBroadcastAddr: %v\n",
		utils.NetworkAddr(), utils.BroadcastAddr())

	udp, err := NewUDPService()
	if err != nil {
		fmt.Println(err)
	}

	msg1 := Message{}
	msg1.To = net.ParseIP("192.168.1.38")
	msg1.From = net.ParseIP("192.168.1.13")
	copy(msg1.Data[:], []byte("Hei på deg!\n"))
	udp.send <- msg1

	for {
		msg := <-udp.receive
		fmt.Printf("%v:\n%v", msg.From, string(msg.Data[:]))
	}

	os.Exit(0)
}
