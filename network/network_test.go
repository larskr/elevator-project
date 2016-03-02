package network

import (
	"fmt"
	"testing"
)

func TestMain(m *testing.M) {
	laddr, _ := ResolveLocalAddr(6000)
	fmt.Printf("This computer has local address: %v\n", laddr)

	Init(6000)
	select {}
}

func TestResolveLocalAddr(t *testing.T) {
	laddr, err := ResolveLocalAddr(6000)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Printf("This computer has local address: %v\n", laddr)
}
