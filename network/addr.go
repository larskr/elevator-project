package network

import (
	"net"
	"fmt"
)

// The Addr type is used to identify nodes and can be easliy converted
// to net.IP. It should always be 16 bytes long irregardless of IP version.
type Addr []byte

// Return the IPv6 address of this machine.
func NetworkAddrs() (Addr, Addr) {
	iface, err := net.InterfaceByName("en0")
	if err != nil {
		return nil, nil
	}
	
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, nil
	}
	
	for _, addr := range addrs {
		fmt.Println(addr)
		//if ipnet, ok := addr.(*net.IPNet); ok {
		//	if ipnet.IP.IsLoopback() {
		//		continue
		//	}
		//	fmt.Println(ipnet.IP)
		//}
	}
	
	return nil, nil
}
