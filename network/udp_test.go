package network

import (
	"fmt"
	"net"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	fmt.Printf("NetworkAddr: %v\nBroadcastAddr: %v\n",
		NetworkAddr(), BroadcastAddr())

	udp, err := NewUDPService()
	if err != nil {
		fmt.Println(err)
	}

	msg1 := &UDPMessage{}
	msg1.to = IPToUint32(net.ParseIP("192.168.1.38"))
	msg1.from = IPToUint32(net.ParseIP("192.168.1.13"))
	copy(msg1.data[:], []byte("Hei på deg!\n"))
	udp.Send(msg1)

	for {
		msg := udp.Receive()
		fmt.Printf("%v:\n%v", Uint32ToIP(msg.from), string(msg.data[:]))
	}

	os.Exit(0)
}
