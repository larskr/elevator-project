package network

import (
	"fmt"
	"os"
	"testing"
	"net"
)

func TestMain(m *testing.M) {
	node := new(Node)
	err := node.Start()
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println(net.IP(NetworkAddr()))
	fmt.Println(net.IP(BroadcastAddr()))
	
	//fmt.Println(node.IsConnected())
	
	select{}

	os.Exit(0)
}
