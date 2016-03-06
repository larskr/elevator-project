package network

import (
	//"fmt"
	//"net"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	node := new(NetworkNode)
	node.Start()

	select{}

	os.Exit(0)
}
