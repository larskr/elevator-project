package network

import (
	"net"
	"fmt"
)

type Config struct {
	Interface string
	Protocol  string
}

var config Config

func LoadConfig(conf map[string]string) {
	config.Interface = conf["network.interface"]
	config.Protocol = conf["network.protocol"]
}

// The Addr type is used to identify nodes and can be easliy converted
// to net.IP.
type Addr [16]byte

func (a *Addr) IsZero() bool {
	var zero Addr
	return *a == zero
}

func (a *Addr) SetZero() {
	var zero Addr
	*a = zero
}

func (a Addr) String() string {
	return fmt.Sprintf("%v", net.IP(a[:]))
}

func NetworkAddr() (ret Addr, err error) {
	ifi, err := net.InterfaceByName(config.Interface)
	if err != nil {
		return
	}

	addrs, err := ifi.Addrs()
	if err != nil {
		return
	}

	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ip := ipnet.IP.To16()
				copy(ret[:], ip)
				return
			}
		}
	}

	return
}

func BroadcastAddr() (ret Addr, err error) {
	ifi, err := net.InterfaceByName(config.Interface)
	if err != nil {
		return
	}

	addrs, err := ifi.Addrs()
	if err != nil {
		return
	}

	for _, addr := range addrs {
		ipnet, isIPNet := addr.(*net.IPNet)
		isBroadcast := (ifi.Flags&net.FlagBroadcast != 0)
		isIPv4 := (ipnet.IP.To4() != nil)
		if isIPNet && isIPv4 && isBroadcast {
			ip := ipnet.IP.To16()
			mask := net.IP(ipnet.Mask).To16()
			ip[12] |= ^mask[12]
			ip[13] |= ^mask[13]
			ip[14] |= ^mask[14]
			ip[15] |= ^mask[15]
			copy(ret[:], ip)
			return
		}
	}

	return
}
