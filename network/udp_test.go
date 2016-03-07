package network

import (
	"fmt"
	//"net"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	node := new(NetworkNode)
	node.Start()

	fmt.Println(node.IsConnected())
	
	for {
//		m := <- node.udp.receivec
//		fmt.Println(m)
	}

	os.Exit(0)
}
