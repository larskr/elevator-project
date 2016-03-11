package network

import (
	"fmt"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	node := new(Node)
	err := node.Start()
	if err != nil {
		fmt.Println(err)
	}

	NetworkAddrs()
	
	//fmt.Println(node.IsConnected())
	
	select{}

	os.Exit(0)
}
