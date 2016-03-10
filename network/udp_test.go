package network

import (
	"fmt"
	//"net"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	node := new(Node)
	node.Start()

	fmt.Println(node.IsConnected())
	
	select{}

	os.Exit(0)
}
