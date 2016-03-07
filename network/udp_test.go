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
	
	select{}

	os.Exit(0)
}
